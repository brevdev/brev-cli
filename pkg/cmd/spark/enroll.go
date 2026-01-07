package spark

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	brevcloud "github.com/brevdev/brev-cli/pkg/brevcloud"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	sparklib "github.com/brevdev/brev-cli/pkg/spark"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

//go:embed install-binary.sh
var installBinaryScript string

//go:embed install-service.sh
var installServiceScript string

const (
	defaultEnrollTimeout = 10 * time.Minute
	envFilePath          = "/etc/default/brevd"
	stateDirDefault      = "/var/lib/devplane/brevd"
	serviceName          = "brevd"
	binaryPath           = "/usr/local/bin/brevd"
)

type enrollOptions struct {
	cloudCredID      string
	user             string
	noSudoers        bool
	systemd          bool
	agentVersion     string
	wait             bool
	timeout          time.Duration
	printCmd         bool
	dryRun           bool
	json             bool
	assumeInstalled  bool
	mockRegistration bool
}

func NewCmdSparkEnroll(t *terminal.Terminal, loginCmdStore *store.AuthHTTPStore) *cobra.Command {
	var opts enrollOptions
	cmd := &cobra.Command{
		Use:           "enroll [spark-alias]",
		Short:         "Enroll a Spark node into Brev",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := ""
			if len(args) > 0 {
				alias = args[0]
			}

			if env := os.Getenv("BREVD_ASSUME_INSTALLED"); strings.EqualFold(env, "true") {
				opts.assumeInstalled = true
			}

			if loginCmdStore == nil {
				return breverrors.WrapAndTrace(errors.New("authenticated store unavailable"))
			}

			if opts.cloudCredID == "" {
				if cid, err := resolveDefaultCloudCred(cmd.Context(), loginCmdStore); err == nil && cid != "" {
					opts.cloudCredID = cid
					logStep(t, fmt.Sprintf("resolved default brev cloud cred: %s", opts.cloudCredID))
				} else {
					logStep(t, "no default brev cloud cred found; proceeding without cloud cred")
				}
			} else {
				logStep(t, fmt.Sprintf("using provided brevcloud cloud cred: %s", opts.cloudCredID))
			}

			return runSparkEnroll(cmd.Context(), t, loginCmdStore, alias, opts)
		},
	}

	cmd.Flags().StringVar(&opts.cloudCredID, "cloud-cred-id", "", "Brev cloud credential id")
	cmd.Flags().StringVar(&opts.user, "user", "brevcloud", "Remote user to ensure exists")
	cmd.Flags().BoolVar(&opts.noSudoers, "no-sudoers", false, "Skip sudoers entry for the user")
	cmd.Flags().BoolVar(&opts.systemd, "systemd", true, "Manage systemd unit (set false to skip)")
	cmd.Flags().StringVar(&opts.agentVersion, "agent-version", "", "Agent version to expect")
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Wait for Brev node to report active")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", defaultEnrollTimeout, "Overall timeout for enroll")
	cmd.Flags().BoolVar(&opts.printCmd, "print-cmd", false, "Print remote ssh commands")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print actions without executing")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output JSON result")
	cmd.Flags().BoolVar(&opts.assumeInstalled, "assume-installed", false, "Assume brevd binary and systemd unit already exist")

	return cmd
}

type enrollResult struct {
	BrevCloudNodeID string `json:"brev_cloud_node_id"`
	CloudCredID     string `json:"cloud_cred_id"`
	CloudName       string `json:"cloud_name,omitempty"`
	Phase           string `json:"phase,omitempty"`
	LastSeenAt      string `json:"last_seen_at,omitempty"`
	AgentVersion    string `json:"agent_version,omitempty"`
}

func runSparkEnroll(ctx context.Context, t *terminal.Terminal, loginStore *store.AuthHTTPStore, alias string, opts enrollOptions) error {
	ctx, cancel := sparklib.WithTimeout(ctx, opts.timeout)
	defer cancel()

	uiEnabled := !opts.json
	var sp *spinner.Spinner
	stopSpinner := func() {
		if sp != nil {
			sp.Stop()
			sp = nil
		}
	}
	defer stopSpinner()

	fail := func(err error) error {
		msg := formatEnrollError(err, opts)
		stopSpinner()
		t.Eprint(t.Red("\n  Failed: " + msg))
		return errors.New(msg)
	}

	if uiEnabled {
		if user, err := loginStore.GetCurrentUser(); err == nil {
			identity := user.Email
			if identity == "" {
				identity = user.Username
			}
			if identity != "" {
				t.Print(fmt.Sprintf("Logged in as %s", identity))
			}
		}
	}

	if uiEnabled {
		searchLabel := t.Yellow("DGX Spark")
		if alias != "" {
			searchLabel = fmt.Sprintf("%s %s", t.Yellow("DGX Spark"), t.Yellow(alias))
		}
		sp = t.NewSpinner()
		sp.Suffix = fmt.Sprintf(" Searching for %s...", searchLabel)
		sp.Start()
	}

	logStep(t, "resolving spark host via Sync ssh_config")
	host, err := resolveSparkHost(t, alias)
	if err != nil {
		return fail(err)
	}

	logStep(t, fmt.Sprintf("target host: %s@%s:%d", host.User, host.Hostname, host.Port))
	aliasLabel := host.Alias
	if aliasLabel == "" {
		aliasLabel = sparklib.HostLabel(host)
	}
	if uiEnabled {
		stopSpinner()
		t.Print(fmt.Sprintf("\n  %s %s %s", t.Green("✓"), t.Green("Found"), t.Yellow(aliasLabel)))
		sp = t.NewSpinner()
		sp.Suffix = fmt.Sprintf(" Registering %s 🤙 ...", t.Yellow(aliasLabel))
		sp.Start()
	}

	if opts.dryRun {
		logStep(t, fmt.Sprintf("dry-run: would connect to %s with cloud_cred_id=%s", host.Alias, opts.cloudCredID))
		if uiEnabled {
			t.Print(fmt.Sprintf("Dry-run: would connect to %s with cloud_cred_id=%s", sparklib.HostLabel(host), opts.cloudCredID))
		}
		return nil
	}

	brevCloudClient := brevcloud.NewClient(loginStore)
	remote := sparklib.NewRemoteRunner(files.AppFs)
	orgID := ""
	if org, err := loginStore.GetActiveOrganizationOrDefault(); err == nil && org != nil {
		orgID = org.ID
	}

	if uiEnabled {
		sp = t.NewSpinner()
		if opts.mockRegistration {
			sp.Suffix = " Configuring (mock)..."
		} else {
			sp.Suffix = " Configuring..."
		}
		sp.Start()
	}

	logStep(t, "probing SSH connectivity (uname/whoami/hostname)")
	if err := probeConnectivity(ctx, remote, host, opts.printCmd); err != nil {
		return fail(err)
	}
	logStep(t, "SSH connectivity probe succeeded")

	// Minimal path: ensure agent and unit pre-exist.
	logStep(t, "checking for existing brevd binary and systemd unit on remote")
	if err := ensureAgentPresent(ctx, remote, host, opts.printCmd); err != nil {
		return fail(err)
	}
	logStep(t, "verified brevd binary and unit are present")

	var intent brevcloud.CreateRegistrationIntentResponse
	if opts.mockRegistration {
		logStep(t, "mocking registration intent (no API call)")
		intent = brevcloud.MockRegistrationIntent(opts.cloudCredID)
	} else {
		logStep(t, fmt.Sprintf("requesting registration intent for cloud cred %s", opts.cloudCredID))
		req := brevcloud.CreateRegistrationIntentRequest{
			CloudCredID: opts.cloudCredID,
			OrgID:       orgID,
		}
		ref := ""
		switch {
		case req.CloudCredID != "" && req.OrgID != "":
			ref = fmt.Sprintf("cloud cred %s org %s", req.CloudCredID, req.OrgID)
		case req.CloudCredID != "":
			ref = fmt.Sprintf("cloud cred %s", req.CloudCredID)
		case req.OrgID != "":
			ref = fmt.Sprintf("org %s", req.OrgID)
		default:
			ref = "no reference"
		}
		resp, err := brevCloudClient.CreateRegistrationIntent(ctx, req)
		if err != nil {
			return fail(fmt.Errorf("failed to create registration intent (%s): %w", ref, err))
		}
		intent = *resp
		logStep(t, fmt.Sprintf("created registration intent: brev_cloud_node_id=%s expires_at=%s", intent.BrevCloudNodeID, intent.ExpiresAt))
		if strings.TrimSpace(intent.RegistrationToken) == "" {
			return fail(fmt.Errorf("registration intent returned empty registration token for %s", ref))
		}
	}

	logStep(t, "generating remote enroll script")
	brevCloudBase := buildAgentAPIBase(config.GlobalConfig.GetDevplaneAPIURL())
	script := buildEnrollScript(intent.BrevCloudNodeID, intent.RegistrationToken, opts.cloudCredID, brevCloudBase, opts.mockRegistration)
	writeCmd := fmt.Sprintf("mkdir -p /tmp/brevd && cat <<'ENROLL_EOF' >/tmp/brevd/enroll.sh\n%s\nENROLL_EOF\nchmod +x /tmp/brevd/enroll.sh", script)
	if opts.printCmd {
		fmt.Printf("[remote] write enroll script to /tmp/brevd/enroll.sh\n")
	}
	out, err := remote.Run(ctx, host, writeCmd)
	if err != nil {
		return fail(fmt.Errorf("failed to write enroll script on %s: err=%v output=%s", sparklib.HostLabel(host), err, strings.TrimSpace(out)))
	}

	logStep(t, "executing remote enroll script")
	if opts.printCmd {
		fmt.Printf("[remote] sudo /tmp/brevd/enroll.sh || /tmp/brevd/enroll.sh\n")
	}
	runEnrollCmd := "sudo /tmp/brevd/enroll.sh || /tmp/brevd/enroll.sh"
	out, err = remote.Run(ctx, host, runEnrollCmd)
	if err != nil {
		return fail(fmt.Errorf("remote enroll script failed on %s: err=%v output=%s", sparklib.HostLabel(host), err, strings.TrimSpace(out)))
	}

	var result enrollResult
	result.BrevCloudNodeID = intent.BrevCloudNodeID
	result.CloudCredID = intent.CloudCredID

	if opts.wait && !opts.mockRegistration {
		logStep(t, fmt.Sprintf("waiting for brev cloud node %s to report active...", intent.BrevCloudNodeID))
		node, err := waitForBrevCloudNode(ctx, brevCloudClient, intent.BrevCloudNodeID, t)
		if err != nil {
			return fail(err)
		}
		result.CloudName = node.CloudName
		result.Phase = node.Phase
		result.LastSeenAt = node.LastSeenAt
		result.AgentVersion = node.AgentVersion
		t.Vprintf("brev cloud node active: phase=%s last_seen_at=%s agent=%s", node.Phase, node.LastSeenAt, node.AgentVersion)
	}

	stopSpinner()
	if uiEnabled {
		if opts.mockRegistration {
			t.Print("\n" + t.Green("✓ Mock enroll finished (config written, no restart)"))
		} else {
			t.Print("\n" + t.Green("✓ Registration complete"))
		}
		if manageURL := enrollmentManageURL(loginStore, aliasLabel); manageURL != "" {
			t.Print(fmt.Sprintf("Manage your %s %s", t.Yellow("Spark"), t.Blue(manageURL)))
		}
	}

	if opts.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return breverrors.WrapAndTrace(enc.Encode(result))
	}

	t.Vprintf("Enrolled BrevCloud node: %s (cloud cred: %s)", result.BrevCloudNodeID, result.CloudCredID)
	if result.Phase != "" {
		t.Vprintf("Phase: %s LastSeen: %s Agent: %s", result.Phase, result.LastSeenAt, result.AgentVersion)
	}

	return nil
}

func resolveSparkHost(t *terminal.Terminal, alias string) (sparklib.Host, error) {
	locator := sparklib.NewSyncSSHConfigLocator()
	resolver := sparklib.NewDefaultSyncConfigResolver(files.AppFs, locator)
	hosts, err := resolver.ResolveHosts()
	if err != nil {
		return sparklib.Host{}, breverrors.WrapAndTrace(err)
	}
	selected, err := sparklib.SelectHost(hosts, alias, sparklib.TerminalPrompter{})
	if err != nil {
		return sparklib.Host{}, breverrors.WrapAndTrace(err)
	}
	return selected, nil
}

func resolveDefaultCloudCred(ctx context.Context, loginStore *store.AuthHTTPStore) (string, error) {
	client := brevcloud.NewClient(loginStore)
	creds, err := client.ListCloudCred(ctx)
	if err != nil {
		return "", err
	}

	matchAll := map[string]string{
		"name":      "brevcloud-default",
		"managedBy": "system",
		"provider":  "brevcloud",
	}

	for _, cc := range creds {
		if cc.ProviderID != "" && strings.ToLower(cc.ProviderID) != "brevcloud" {
			continue
		}
		if hasAllLabels(cc.Labels, matchAll) {
			return cc.ID, nil
		}
	}

	for _, cc := range creds {
		if hasAllLabels(cc.Labels, matchAll) {
			return cc.ID, nil
		}
	}

	return "", fmt.Errorf("no default brevcloud cloud credential found; provide --cloud-cred-id or BREV_CLOUD_CRED_ID")
}

func hasAllLabels(labels map[string]string, required map[string]string) bool {
	if len(required) == 0 {
		return true
	}
	for k, v := range required {
		if labels == nil {
			return false
		}
		if val, ok := labels[k]; !ok || val != v {
			return false
		}
	}
	return true
}

func ensureAgentPresent(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, printCmd bool) error {
	// First check if brevd is already installed
	checkCmd := fmt.Sprintf("test -x %s || sudo test -x %s", binaryPath, binaryPath)
	if printCmd {
		fmt.Printf("[remote] %s\n", checkCmd)
	}
	_, err := remote.Run(ctx, host, checkCmd)
	// If brevd doesn't exist, install it from GitHub releases
	if err != nil {
		if printCmd {
			fmt.Printf("[remote] Installing brevd from GitHub releases...\n")
		}

		// Write the install script to remote and execute it
		writeScriptCmd := fmt.Sprintf("cat > /tmp/install-brevd-binary.sh <<'SCRIPT_EOF'\n%s\nSCRIPT_EOF\nchmod +x /tmp/install-brevd-binary.sh", installBinaryScript)
		if printCmd {
			fmt.Printf("[remote] Writing install script to /tmp/install-brevd-binary.sh\n")
		}
		if _, err := remote.Run(ctx, host, writeScriptCmd); err != nil {
			return fmt.Errorf("failed to write install script on %s: %w", sparklib.HostLabel(host), err)
		}

		// Execute the install script
		executeCmd := "/tmp/install-brevd-binary.sh && rm -f /tmp/install-brevd-binary.sh"
		if printCmd {
			fmt.Printf("[remote] %s\n", executeCmd)
		}
		out, err := remote.Run(ctx, host, executeCmd)
		if err != nil {
			return fmt.Errorf("failed to install brevd on %s: err=%v output=%s", sparklib.HostLabel(host), err, strings.TrimSpace(out))
		}
		if printCmd {
			fmt.Printf("[remote] brevd install output: %s\n", strings.TrimSpace(out))
		}
	}

	// Now check for systemd service file
	serviceCheck := fmt.Sprintf("systemctl status %s >/dev/null 2>&1 || sudo systemctl status %s >/dev/null 2>&1 || test -f /etc/systemd/system/%s.service", serviceName, serviceName, serviceName)
	if printCmd {
		fmt.Printf("[remote] %s\n", serviceCheck)
	}
	out, err := remote.Run(ctx, host, serviceCheck)
	if err != nil {
		// If systemd service doesn't exist, install it
		if printCmd {
			fmt.Printf("[remote] Installing brevd systemd service...\n")
		}

		// Write the service install script to remote and execute it with STATE_DIR env var
		writeScriptCmd := fmt.Sprintf("cat > /tmp/install-brevd-service.sh <<'SCRIPT_EOF'\n%s\nSCRIPT_EOF\nchmod +x /tmp/install-brevd-service.sh", installServiceScript)
		if printCmd {
			fmt.Printf("[remote] Writing service install script to /tmp/install-brevd-service.sh\n")
		}
		if _, err := remote.Run(ctx, host, writeScriptCmd); err != nil {
			return fmt.Errorf("failed to write service install script on %s: %w", sparklib.HostLabel(host), err)
		}

		// Execute the service install script with STATE_DIR environment variable
		executeCmd := fmt.Sprintf("STATE_DIR=%s /tmp/install-brevd-service.sh && rm -f /tmp/install-brevd-service.sh", stateDirDefault)
		if printCmd {
			fmt.Printf("[remote] %s\n", executeCmd)
		}
		out, err := remote.Run(ctx, host, executeCmd)
		if err != nil {
			return fmt.Errorf("failed to install brevd systemd service on %s: err=%v output=%s", sparklib.HostLabel(host), err, strings.TrimSpace(out))
		}
		if printCmd {
			fmt.Printf("[remote] systemd service install output: %s\n", strings.TrimSpace(out))
		}
	} else if printCmd {
		fmt.Printf("[remote] systemd service check output: %s\n", strings.TrimSpace(out))
	}

	return nil
}

func ensureConfigDir(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, printCmd bool) error {
	cmds := []string{
		fmt.Sprintf("sudo -n mkdir -p %s", stateDirDefault),
		fmt.Sprintf("sudo mkdir -p %s", stateDirDefault),
		fmt.Sprintf("mkdir -p %s", stateDirDefault),
	}
	var errs []string
	for _, cmd := range cmds {
		if printCmd {
			fmt.Printf("[remote] %s\n", cmd)
		}
		out, err := remote.Run(ctx, host, cmd)
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Sprintf("cmd=%s err=%v output=%s", cmd, err, strings.TrimSpace(out)))
	}
	return fmt.Errorf("failed to create config dir on %s; attempts: %s", sparklib.HostLabel(host), strings.Join(errs, " | "))
}

func writeAgentConfig(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, brevCloudNodeID, registrationToken, cloudCredID string, printCmd bool) error {
	var b strings.Builder
	b.WriteString("brev_cloud_node_id: ")
	b.WriteString(brevCloudNodeID)
	b.WriteString("\nregistration_token: ")
	b.WriteString(registrationToken)
	if cloudCredID != "" {
		b.WriteString("\ncloud_cred_id: ")
		b.WriteString(cloudCredID)
	}
	b.WriteString("\n")
	payload := b.String()
	cmds := []string{
		fmt.Sprintf("cat <<'EOF' | sudo -n tee %s >/dev/null\n%sEOF\n", envFilePath, payload),
		fmt.Sprintf("cat <<'EOF' | sudo tee %s >/dev/null\n%sEOF\n", envFilePath, payload),
		fmt.Sprintf("cat <<'EOF' | tee %s >/dev/null\n%sEOF\n", envFilePath, payload),
	}
	var errs []string
	for _, cmd := range cmds {
		if printCmd {
			fmt.Printf("[remote] %s\n", cmd)
		}
		out, err := remote.Run(ctx, host, cmd)
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Sprintf("cmd=%s err=%v output=%s", cmd, err, strings.TrimSpace(out)))
	}
	return fmt.Errorf("failed to write config on %s; attempts: %s", sparklib.HostLabel(host), strings.Join(errs, " | "))
}

func restartService(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, printCmd bool) error {
	cmd := fmt.Sprintf("sudo systemctl restart %s", serviceName)
	if printCmd {
		fmt.Printf("[remote] %s\n", cmd)
	}
	_, err := remote.Run(ctx, host, cmd)
	return breverrors.WrapAndTrace(err)
}

func waitForBrevCloudNode(ctx context.Context, client *brevcloud.Client, brevCloudNodeID string, t *terminal.Terminal) (*brevcloud.BrevCloudNode, error) {
	interval := 3 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		node, err := client.GetBrevCloudNode(ctx, brevCloudNodeID)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(node.Phase, "ACTIVE") || node.LastSeenAt != "" {
			return node, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			logStep(t, fmt.Sprintf("still waiting for brev cloud node %s (phase=%s last_seen_at=%s)", brevCloudNodeID, node.Phase, node.LastSeenAt))
		}
	}
}

func logStep(t *terminal.Terminal, msg string) {
	// step logging suppressed for quieter output
	_ = t
	_ = msg
}

func formatEnrollError(err error, opts enrollOptions) string {
	if err == nil {
		return ""
	}

	raw := strings.TrimSpace(err.Error())
	if isSudoError(raw) {
		return "Sudo required on target; rerun with a TTY or configure passwordless sudo."
	}

	var httpErr *store.HTTPResponseError
	if errors.As(err, &httpErr) && httpErr.Response != nil && httpErr.Response.StatusCode() == http.StatusNotFound {
		hint := ""
		if !opts.mockRegistration {
			hint = " Use --mock-registration for demos."
		}
		return fmt.Sprintf("Failed to create registration intent (cloud cred %s): endpoint not available (404).%s", opts.cloudCredID, hint)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "Timed out waiting for node to register"
	}

	firstLine := raw
	if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
		firstLine = strings.TrimSpace(raw[:idx])
	}
	return firstLine
}

func isSudoError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "usage: sudo") ||
		strings.Contains(lower, "sudo: a password is required") ||
		strings.Contains(lower, "sudo: no tty present") ||
		strings.Contains(lower, "sudo: sorry, you must have a tty")
}

func enrollmentManageURL(loginStore *store.AuthHTTPStore, alias string) string {
	if loginStore == nil {
		return ""
	}
	meta, err := loginStore.GetCurrentWorkspaceMeta()
	if err != nil || meta == nil || meta.OrganizationID == "" || alias == "" {
		return ""
	}
	return fmt.Sprintf("https://brev.nvidia.com/org/%s/compute/%s", meta.OrganizationID, url.PathEscape(alias))
}

func buildAgentAPIBase(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + strings.TrimPrefix(base, "//")
	}
	base = strings.TrimRight(base, "/")
	return base + "/agent/v1"
}

func buildEnrollScript(brevCloudNodeID, registrationToken, cloudCredID, brevCloudBase string, mock bool) string {
	var envContent strings.Builder
	envContent.WriteString("BREV_AGENT_BREV_CLOUD_NODE_ID=")
	envContent.WriteString(brevCloudNodeID)
	envContent.WriteString("\nBREV_AGENT_REGISTRATION_TOKEN=")
	envContent.WriteString(registrationToken)
	envContent.WriteString("\nBREV_AGENT_STATE_DIR=")
	envContent.WriteString(stateDirDefault)
	if brevCloudBase != "" {
		envContent.WriteString("\nBREV_AGENT_BREV_CLOUD_URL=")
		envContent.WriteString(brevCloudBase)
	}
	if cloudCredID != "" {
		envContent.WriteString("\nBREV_AGENT_CLOUD_CRED_ID=")
		envContent.WriteString(cloudCredID)
	}
	envContent.WriteString("\n")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

LOG_DIR=/tmp/brevd
LOG_FILE="${LOG_DIR}/enroll.log"
ENV_FILE="%s"
SERVICE="%s"
MOCK=%t

log() {
	local msg="$1"
	mkdir -p "${LOG_DIR}"
	logger -t brev-enroll "${msg}" 2>/dev/null || true
	printf '%%s\n' "${msg}" >>"${LOG_FILE}"
}

run_cmd() {
	local cmd="$1"
	bash -c "${cmd}" >>"${LOG_FILE}" 2>&1
}

ensure_dir() {
	local dir="$1"
	local cmds=(
		"sudo -n mkdir -p \"${dir}\""
		"sudo mkdir -p \"${dir}\""
		"mkdir -p \"${dir}\""
	)
	for c in "${cmds[@]}"; do
		if run_cmd "${c}"; then
			return 0
		fi
	done
	log "failed to create dir ${dir}"
	exit 1
}

write_file() {
	local content="$1"
	local dest="$2"
	local tmp="${dest}.tmp"

	printf '%%s\n' "${content}" >"${tmp}"

	local cmds=(
		"sudo -n tee \"${dest}\" >/dev/null"
		"sudo tee \"${dest}\" >/dev/null"
		"tee \"${dest}\" >/dev/null"
	)
	for c in "${cmds[@]}"; do
		if bash -c "${c}" <"${tmp}" >>"${LOG_FILE}" 2>&1; then
			rm -f "${tmp}"
			return 0
		fi
	done

	log "failed to write file to ${dest}"
	rm -f "${tmp}"
	exit 1
}

set_file_mode() {
	local mode="$1"
	local path="$2"
	local cmds=(
		"sudo -n chmod ${mode} \"${path}\""
		"sudo chmod ${mode} \"${path}\""
		"chmod ${mode} \"${path}\""
	)
	for c in "${cmds[@]}"; do
		if run_cmd "${c}"; then
			return 0
		fi
	done
	log "failed to chmod ${mode} ${path}"
	exit 1
}

extract_env_value() {
	local key="$1"
	local content="$2"
	while IFS= read -r line; do
		if [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]]; then
			continue
		fi
		line="${line#"${line%%[![:space:]]*}"}"
		if [[ "${line}" == "${key}="* ]]; then
			local val="${line#${key}=}"
			if [[ "${val}" =~ ^\".*\"$ || "${val}" =~ ^\'.*\'$ ]]; then
				val="${val:1:${#val}-2}"
			fi
			printf '%%s' "${val}"
			break
		fi
	done <<<"${content}"
	return 0
}

restart_service() {
	local svc="$1"
	local cmds=(
		"sudo -n systemctl restart \"${svc}\""
		"sudo systemctl restart \"${svc}\""
		"systemctl restart \"${svc}\""
	)
	for c in "${cmds[@]}"; do
		if run_cmd "${c}"; then
			return 0
		fi
	done
	log "failed to restart service ${svc}"
	exit 1
}

ENV_CONTENT=$(cat <<'EOF'
%s
EOF
)

log "enroll start"
log "probe: $(uname -a)"
log "user: $(whoami)"
log "host: $(hostname)"
log "env file target: ${ENV_FILE}"

ensure_dir "$(dirname "${ENV_FILE}")"
write_file "${ENV_CONTENT}" "${ENV_FILE}"
set_file_mode 600 "${ENV_FILE}"

state_dir="$(extract_env_value "BREV_AGENT_STATE_DIR" "${ENV_CONTENT}")"
device_token_path="$(extract_env_value "BREV_AGENT_DEVICE_TOKEN_PATH" "${ENV_CONTENT}")"

if [[ -n "${state_dir}" ]]; then
	log "ensuring state dir ${state_dir}"
	ensure_dir "${state_dir}"
fi

if [[ -n "${device_token_path}" ]]; then
	log "ensuring device token parent dir for ${device_token_path}"
	ensure_dir "$(dirname "${device_token_path}")"
elif [[ -n "${state_dir}" ]]; then
	ensure_dir "${state_dir}"
fi

if [[ "${MOCK}" == "true" ]]; then
	log "mock mode: skipping agent restart"
	exit 0
fi

restart_service "${SERVICE}"
log "enroll success"
`, envFilePath, serviceName, mock, envContent.String())

	return script
}

func probeConnectivity(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, printCmd bool) error {
	cmd := "uname -a && whoami && hostname"
	if printCmd {
		fmt.Printf("[remote] %s\n", cmd)
	}
	out, err := remote.Run(ctx, host, cmd)
	if err != nil {
		return fmt.Errorf("ssh connectivity check failed on %s: err=%v output=%s", sparklib.HostLabel(host), err, strings.TrimSpace(out))
	}
	return nil
}
