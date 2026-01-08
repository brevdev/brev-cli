package spark

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	brevcloud "github.com/brevdev/brev-cli/pkg/brevcloud"
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

//go:embed install-user.sh
var installUserScript string

const (
	defaultEnrollTimeout     = 10 * time.Minute
	envFilePath              = "/etc/default/brevd"
	stateDirDefault          = "/var/lib/devplane/brevd"
	serviceName              = "brevd"
	binaryPath               = "/usr/local/bin/brevd"
	binaryInstallScriptPath  = "/tmp/install-brevd-binary.sh"
	serviceInstallScriptPath = "/tmp/install-brevd-service.sh"
)

type enrollOptions struct {
	agentVersion     string
	wait             bool
	timeout          time.Duration
	printCmd         bool
	dryRun           bool
	json             bool
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

			if loginCmdStore == nil {
				return breverrors.WrapAndTrace(errors.New("authenticated store unavailable"))
			}

			return runSparkEnroll(cmd.Context(), t, loginCmdStore, alias, opts)
		},
	}

	cmd.Flags().StringVar(&opts.agentVersion, "agent-version", "", "Agent version to expect")
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Wait for Brev node to report active")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", defaultEnrollTimeout, "Overall timeout for enroll")
	cmd.Flags().BoolVar(&opts.printCmd, "print-cmd", false, "Print remote ssh commands")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print actions without executing")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output JSON result")

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

	host, err := resolveSparkHost(t, alias)
	if err != nil {
		return fail(err)
	}

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

	cloudCredID, err := resolveDefaultCloudCred(ctx, loginStore)

	if opts.dryRun {
		t.Print(fmt.Sprintf("Dry-run: would connect to %s with cloud_cred_id=%s", sparklib.HostLabel(host), cloudCredID))
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

	if err := probeConnectivity(ctx, remote, host, opts.printCmd); err != nil {
		return fail(err)
	}

	// Minimal path: ensure agent and unit pre-exist.
	if err := ensureAgentPresent(ctx, remote, host, opts.printCmd); err != nil {
		return fail(err)
	}

	var intent brevcloud.CreateRegistrationIntentResponse
	if opts.mockRegistration {
		intent = brevcloud.MockRegistrationIntent(cloudCredID)
	} else {
		req := brevcloud.CreateRegistrationIntentRequest{
			CloudCredID: cloudCredID,
			OrgID:       orgID,
		}
		resp, err := brevCloudClient.CreateRegistrationIntent(ctx, req)
		if err != nil {
			return fail(err)
		}
		intent = *resp
	}

	var result enrollResult
	result.BrevCloudNodeID = intent.BrevCloudNodeID
	result.CloudCredID = intent.CloudCredID

	if opts.wait && !opts.mockRegistration {
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
		t.Print(fmt.Sprintf("Manage your %s %s", t.Yellow("Spark"), t.Blue(fmt.Sprintf("https://brev.nvidia.com/org/%s/environments", orgID))))
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

func resolveSparkHost(_ *terminal.Terminal, alias string) (sparklib.Host, error) {
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
	org, err := loginStore.GetActiveOrganizationOrDefault()
	cred, err := client.ListCloudCredID(ctx, org.ID)
	if err != nil {
		return "", err
	}

	return cred, nil
}

func ensureAgentPresent(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, printCmd bool) error {
	binaryWriteCmd := buildWriteScriptCmd(binaryInstallScriptPath, installBinaryScript)
	binaryExecuteCmd := buildExecuteScriptCmd(binaryInstallScriptPath, "")

	serviceWriteCmd := buildWriteScriptCmd(serviceInstallScriptPath, installServiceScript)
	serviceExecuteCmd := buildExecuteScriptCmd(serviceInstallScriptPath, fmt.Sprintf("STATE_DIR=%s", stateDirDefault))

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

		if err := runInstallScript(ctx, remote, host, printCmd, "brevd", binaryInstallScriptPath, binaryWriteCmd, binaryExecuteCmd); err != nil {
			return err
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

		if err := runInstallScript(ctx, remote, host, printCmd, "brevd systemd service", serviceInstallScriptPath, serviceWriteCmd, serviceExecuteCmd); err != nil {
			return err
		}
	} else if printCmd {
		fmt.Printf("[remote] systemd service check output: %s\n", strings.TrimSpace(out))
	}

	return nil
}

func buildWriteScriptCmd(remotePath, script string) string {
	return fmt.Sprintf("cat > %[1]s <<'SCRIPT_EOF'\n%[2]s\nSCRIPT_EOF\nchmod +x %[1]s", remotePath, script)
}

func buildExecuteScriptCmd(remotePath, envPrefix string) string {
	if envPrefix != "" && !strings.HasSuffix(envPrefix, " ") {
		envPrefix += " "
	}
	return fmt.Sprintf("%s %s && rm -f %s", envPrefix, remotePath, remotePath)
}

func runInstallScript(ctx context.Context, remote sparklib.RemoteRunner, host sparklib.Host, printCmd bool, scriptName, remotePath, writeCmd, executeCmd string) error {
	if printCmd {
		fmt.Printf("[remote] Writing %s install script to %s\n", scriptName, remotePath)
	}
	if _, err := remote.Run(ctx, host, writeCmd); err != nil {
		return fmt.Errorf("failed to write %s install script on %s: %w", scriptName, sparklib.HostLabel(host), err)
	}

	if printCmd {
		fmt.Printf("[remote] %s\n", executeCmd)
	}
	out, err := remote.Run(ctx, host, executeCmd)
	if err != nil {
		return fmt.Errorf("failed to install %s on %s: err=%v output=%s", scriptName, sparklib.HostLabel(host), err, strings.TrimSpace(out))
	}
	if printCmd {
		fmt.Printf("[remote] %s install output: %s\n", scriptName, strings.TrimSpace(out))
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
		}
	}
}

func formatEnrollError(err error, opts enrollOptions) string {
	if err == nil {
		return ""
	}

	raw := strings.TrimSpace(err.Error())
	if isSudoError(raw) {
		return "Sudo required on target; rerun with a TTY or configure passwordless sudo."
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
