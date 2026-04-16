package launchable

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

const (
	defaultLaunchableViewAccess = "public"
	defaultLaunchableMode       = "vm"
	defaultFlexibleStorageGiB   = 256
	maxLifecycleScriptBytes     = 16 * 1024
	maxCTAServiceCount          = 2
)

type launchableCreateOptions struct {
	Name              string
	Description       string
	ViewAccess        string
	Mode              string
	InstanceType      string
	Storage           string
	FileURL           string
	Jupyter           bool
	LifecycleScript   string
	LifecycleFilePath string
	ComposeURL        string
	ComposeFilePath   string
	ComposeYAML       string
	SecureLinks       []string
	FirewallRules     []string
	DryRun            bool
}

type secureLinkSpec struct {
	Name     string
	Port     int
	CTA      bool
	CTALabel string
}

type firewallRuleSpec struct {
	Ports      string
	AllowedIPs string
	StartPort  int
	EndPort    int
}

var (
	githubDirectoryURLPattern = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+/tree/[A-Za-z0-9_.-]+(?:/[\S]*)?$`)
	gitlabDirectoryURLPattern = regexp.MustCompile(`^https://gitlab\.com/(?:[A-Za-z0-9_-]+/)*[A-Za-z0-9_.-]+/-/tree/[A-Za-z0-9_.-]+(?:/[\S]*)?$`)
	sizePattern               = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)([a-z]+)$`)
)

func NewCmdLaunchableCreate(t *terminal.Terminal, launchableStore LaunchableCmdStore) *cobra.Command { //nolint:funlen // easier to read with flags close to command
	var opts launchableCreateOptions

	cmd := &cobra.Command{
		Use:                   "create [name]",
		DisableFlagsInUseLine: true,
		Short:                 "Create a launchable",
		Long: `Create a Brev launchable using VM or Docker Compose configuration.

Every supported setting in the current launchable create flow is exposed as flags,
including view access, storage, file source, networking, lifecycle scripts, and
Docker Compose source selection.`,
		Example: `
  brev launchable create demo-vm --mode vm --instance-type n2-standard-4 \
    --description "My VM launchable" --jupyter \
    --lifecycle-script-file ./setup.sh \
    --secure-link name=notebook,port=8888,cta=true,cta-label=OpenNotebook

  brev launchable create demo-compose --mode compose --instance-type g5.xlarge \
    --compose-file ./docker-compose.yml --file-url https://github.com/acme/demo \
    --secure-link name=web,port=3000,cta=true \
    --firewall-rule ports=8000-8100,allowed-ips=user-ip
`,
		Args: cmderrors.TransformToValidationError(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && opts.Name == "" {
				opts.Name = args[0]
			}

			err := runLaunchableCreate(t, launchableStore, opts)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Launchable name (or pass as first argument)")
	cmd.Flags().StringVar(&opts.Description, "description", "", "Launchable description")
	cmd.Flags().StringVar(&opts.ViewAccess, "view-access", defaultLaunchableViewAccess, "Launchable visibility: public, organization, or published")
	cmd.Flags().StringVarP(&opts.Mode, "mode", "m", defaultLaunchableMode, "Launchable build mode: vm or compose")
	cmd.Flags().StringVarP(&opts.InstanceType, "instance-type", "t", "", "Instance type to use for this launchable")
	cmd.Flags().StringVar(&opts.Storage, "storage", "", "Root disk size in GiB for instance types with flexible storage")
	cmd.Flags().StringVar(&opts.FileURL, "file-url", "", "Public notebook, markdown, or git repo URL to preload into the launchable")
	cmd.Flags().BoolVar(&opts.Jupyter, "jupyter", true, "Install Jupyter on the host (applies to vm and compose modes)")
	cmd.Flags().StringVar(&opts.LifecycleScript, "lifecycle-script", "", "Inline lifecycle script for VM mode")
	cmd.Flags().StringVar(&opts.LifecycleFilePath, "lifecycle-script-file", "", "Path to a lifecycle script file for VM mode")
	cmd.Flags().StringVar(&opts.ComposeURL, "compose-url", "", "Public Docker Compose URL")
	cmd.Flags().StringVar(&opts.ComposeFilePath, "compose-file", "", "Path to a local Docker Compose file")
	cmd.Flags().StringVar(&opts.ComposeYAML, "compose-yaml", "", "Inline Docker Compose YAML")
	cmd.Flags().StringArrayVar(&opts.SecureLinks, "secure-link", nil, "Expose a secure link: name=<name>,port=<port>[,cta=true][,cta-label=<label>]")
	cmd.Flags().StringArrayVar(&opts.FirewallRules, "firewall-rule", nil, "Add a firewall rule: ports=<port|start-end>[,allowed-ips=all|user-ip]")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the launchable request JSON without creating it")

	return cmd
}

func runLaunchableCreate(t *terminal.Terminal, launchableStore LaunchableCmdStore, opts launchableCreateOptions) error {
	err := validateCreateOptions(opts)
	if err != nil {
		return err
	}

	org, err := launchableStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return breverrors.NewValidationError("no active organization found; run `brev org set <NAME>` or create an organization first")
	}

	instanceTypes, err := launchableStore.GetAllInstanceTypesWithWorkspaceGroups(org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	selectedInstance, err := getSelectedInstanceType(instanceTypes, opts.InstanceType)
	if err != nil {
		return err
	}

	storageValue, err := resolveStorageValue(*selectedInstance, opts.Storage)
	if err != nil {
		return err
	}

	fileURL, err := normalizeOptionalFileURL(opts.FileURL)
	if err != nil {
		return err
	}

	lifecycleScript, err := resolveLifecycleScript(opts)
	if err != nil {
		return err
	}

	secureLinks, err := parseSecureLinks(opts.SecureLinks)
	if err != nil {
		return err
	}

	firewallRules, err := parseFirewallRules(opts.FirewallRules)
	if err != nil {
		return err
	}

	err = validatePortConflicts(secureLinks, firewallRules)
	if err != nil {
		return err
	}

	dockerCompose, err := resolveDockerCompose(launchableStore, opts)
	if err != nil {
		return err
	}

	req := buildCreateEnvironmentLaunchableRequest(opts, *selectedInstance, storageValue, fileURL, lifecycleScript, dockerCompose, secureLinks, firewallRules)

	if opts.DryRun {
		body, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		t.Vprint(string(body))
		return nil
	}

	res, err := launchableStore.CreateEnvironmentLaunchable(org.ID, req)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green(fmt.Sprintf("Created launchable %q (%s)", req.Name, res.ID)))
	t.Vprint(fmt.Sprintf("Deploy URL: %s", buildLaunchableDeployURL(res.ID)))

	return nil
}

func validateCreateOptions(opts launchableCreateOptions) error {
	if strings.TrimSpace(opts.Name) == "" {
		return breverrors.NewValidationError("launchable name is required")
	}
	if strings.TrimSpace(opts.InstanceType) == "" {
		return breverrors.NewValidationError("--instance-type is required")
	}

	switch opts.ViewAccess {
	case "public", "organization", "published":
	default:
		return breverrors.NewValidationError("--view-access must be one of: public, organization, published")
	}

	switch opts.Mode {
	case "vm", "compose":
	default:
		return breverrors.NewValidationError("--mode must be one of: vm, compose")
	}

	if opts.LifecycleScript != "" && opts.LifecycleFilePath != "" {
		return breverrors.NewValidationError("--lifecycle-script and --lifecycle-script-file are mutually exclusive")
	}

	composeSources := 0
	for _, source := range []string{opts.ComposeURL, opts.ComposeFilePath, opts.ComposeYAML} {
		if strings.TrimSpace(source) != "" {
			composeSources++
		}
	}

	if opts.Mode == "vm" {
		if composeSources > 0 {
			return breverrors.NewValidationError("--compose-url, --compose-file, and --compose-yaml are only valid with --mode compose")
		}
		return nil
	}

	if opts.Mode == "compose" {
		if opts.LifecycleScript != "" || opts.LifecycleFilePath != "" {
			return breverrors.NewValidationError("lifecycle script flags are only valid with --mode vm")
		}
		if composeSources != 1 {
			return breverrors.NewValidationError("exactly one of --compose-url, --compose-file, or --compose-yaml is required with --mode compose")
		}
	}

	return nil
}

func getSelectedInstanceType(allInstanceTypes *gpusearch.AllInstanceTypesResponse, instanceType string) (*gpusearch.InstanceType, error) {
	if allInstanceTypes == nil {
		return nil, breverrors.NewValidationError("no instance types were returned for your organization")
	}

	for i := range allInstanceTypes.AllInstanceTypes {
		if allInstanceTypes.AllInstanceTypes[i].Type == instanceType {
			if len(allInstanceTypes.AllInstanceTypes[i].WorkspaceGroups) == 0 {
				return nil, breverrors.NewValidationError(fmt.Sprintf("instance type %q does not have a workspace group for this organization", instanceType))
			}
			return &allInstanceTypes.AllInstanceTypes[i], nil
		}
	}

	return nil, breverrors.NewValidationError(fmt.Sprintf("instance type %q is not available in the active organization", instanceType))
}

func resolveStorageValue(instanceType gpusearch.InstanceType, requested string) (string, error) {
	if len(instanceType.SupportedStorage) == 0 {
		if requested != "" {
			return "", breverrors.NewValidationError(fmt.Sprintf("instance type %q does not expose configurable storage; omit --storage", instanceType.Type))
		}
		return "", nil
	}

	storage := instanceType.SupportedStorage[0]
	if storage.MinSize != "" && storage.MaxSize != "" {
		minGiB, err := parseSizeToGiB(storage.MinSize)
		if err != nil {
			return "", err
		}
		maxGiB, err := parseSizeToGiB(storage.MaxSize)
		if err != nil {
			return "", err
		}

		value := strings.TrimSpace(requested)
		if value == "" {
			defaultGiB := defaultFlexibleStorageGiB
			if minGiB > 0 && defaultGiB < minGiB {
				defaultGiB = minGiB
			}
			if maxGiB > 0 && defaultGiB > maxGiB {
				defaultGiB = maxGiB
			}
			value = strconv.Itoa(defaultGiB)
		}

		sizeGiB, err := strconv.Atoi(value)
		if err != nil || sizeGiB <= 0 {
			return "", breverrors.NewValidationError("--storage must be a positive integer number of GiB")
		}
		if minGiB > 0 && sizeGiB < minGiB {
			return "", breverrors.NewValidationError(fmt.Sprintf("--storage must be at least %d GiB for instance type %q", minGiB, instanceType.Type))
		}
		if maxGiB > 0 && sizeGiB > maxGiB {
			return "", breverrors.NewValidationError(fmt.Sprintf("--storage must be at most %d GiB for instance type %q", maxGiB, instanceType.Type))
		}

		return strconv.Itoa(sizeGiB), nil
	}

	if requested != "" {
		return "", breverrors.NewValidationError(fmt.Sprintf("instance type %q uses fixed storage; omit --storage", instanceType.Type))
	}

	return "", nil
}

func resolveLifecycleScript(opts launchableCreateOptions) (string, error) {
	if opts.Mode != "vm" {
		return "", nil
	}

	var script string
	switch {
	case opts.LifecycleFilePath != "":
		content, err := os.ReadFile(opts.LifecycleFilePath)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		script = string(content)
	case opts.LifecycleScript != "":
		script = opts.LifecycleScript
	default:
		return "", nil
	}

	err := validateLifecycleScript(script)
	if err != nil {
		return "", err
	}

	return script, nil
}

func validateLifecycleScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return nil
	}
	if len([]byte(script)) > maxLifecycleScriptBytes {
		return breverrors.NewValidationError("--lifecycle-script exceeds the 16 KiB limit")
	}
	if !strings.HasPrefix(strings.TrimSpace(script), "#!/bin/bash") {
		return breverrors.NewValidationError("lifecycle scripts must start with #!/bin/bash")
	}
	return nil
}

func normalizeOptionalFileURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	err := validateFileURL(raw)
	if err != nil {
		return "", err
	}
	return replaceBlobWithRaw(strings.TrimSpace(raw)), nil
}

func validateFileURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if strings.HasSuffix(trimmed, "/") {
		return breverrors.NewValidationError("invalid URL: remove the trailing slash")
	}
	if githubDirectoryURLPattern.MatchString(trimmed) || gitlabDirectoryURLPattern.MatchString(trimmed) {
		return breverrors.NewValidationError("invalid URL: provide a repository or file URL, not a directory URL")
	}
	return nil
}

func replaceBlobWithRaw(raw string) string {
	if strings.Contains(raw, "github.com") && strings.Contains(raw, "/blob/") {
		return strings.Replace(raw, "/blob/", "/raw/", 1)
	}
	if strings.Contains(raw, "gitlab.com") && strings.Contains(raw, "/-/blob/") {
		return strings.Replace(raw, "/-/blob/", "/-/raw/", 1)
	}
	return raw
}

func parseSecureLinks(rawLinks []string) ([]secureLinkSpec, error) {
	if len(rawLinks) == 0 {
		return nil, nil
	}

	links := make([]secureLinkSpec, 0, len(rawLinks))
	nameSet := map[string]struct{}{}
	ctaCount := 0

	for _, raw := range rawLinks {
		values, err := parseFlagAssignments(raw)
		if err != nil {
			return nil, err
		}

		name := strings.TrimSpace(values["name"])
		if name == "" {
			return nil, breverrors.NewValidationError("each --secure-link must include name=<value>")
		}
		if msg := validateTunnelName(name); msg != "" {
			return nil, breverrors.NewValidationError(fmt.Sprintf("invalid secure link name %q: %s", name, msg))
		}
		if _, exists := nameSet[name]; exists {
			return nil, breverrors.NewValidationError(fmt.Sprintf("duplicate secure link name %q", name))
		}
		nameSet[name] = struct{}{}

		portText := strings.TrimSpace(values["port"])
		if portText == "" {
			return nil, breverrors.NewValidationError(fmt.Sprintf("secure link %q is missing port=<value>", name))
		}
		if strings.Contains(portText, "-") {
			return nil, breverrors.NewValidationError(fmt.Sprintf("secure link %q must use a single port, not a port range", name))
		}

		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, breverrors.NewValidationError(fmt.Sprintf("secure link %q must use a port between 1 and 65535", name))
		}

		cta := false
		if rawCTA, ok := values["cta"]; ok && strings.TrimSpace(rawCTA) != "" {
			cta, err = strconv.ParseBool(strings.TrimSpace(rawCTA))
			if err != nil {
				return nil, breverrors.NewValidationError(fmt.Sprintf("secure link %q has invalid cta value %q; use true or false", name, rawCTA))
			}
		}

		ctaLabel := strings.TrimSpace(values["cta-label"])
		if ctaLabel != "" && !cta {
			return nil, breverrors.NewValidationError(fmt.Sprintf("secure link %q cannot set cta-label without cta=true", name))
		}
		if cta {
			ctaCount++
		}

		links = append(links, secureLinkSpec{
			Name:     name,
			Port:     port,
			CTA:      cta,
			CTALabel: ctaLabel,
		})
	}

	if ctaCount > maxCTAServiceCount {
		return nil, breverrors.NewValidationError(fmt.Sprintf("at most %d secure links can have cta=true", maxCTAServiceCount))
	}

	return links, nil
}

func parseFirewallRules(rawRules []string) ([]firewallRuleSpec, error) {
	if len(rawRules) == 0 {
		return nil, nil
	}

	rules := make([]firewallRuleSpec, 0, len(rawRules))
	for _, raw := range rawRules {
		values, err := parseFlagAssignments(raw)
		if err != nil {
			return nil, err
		}

		ports := strings.TrimSpace(values["ports"])
		if ports == "" {
			return nil, breverrors.NewValidationError("each --firewall-rule must include ports=<port|start-end>")
		}

		start, end, err := parsePortRange(ports)
		if err != nil {
			return nil, err
		}

		allowedIPs := strings.TrimSpace(values["allowed-ips"])
		if allowedIPs == "" {
			allowedIPs = "all"
		}
		if allowedIPs != "all" && allowedIPs != "user-ip" {
			return nil, breverrors.NewValidationError(fmt.Sprintf("firewall rule %q has invalid allowed-ips %q; use all or user-ip", ports, allowedIPs))
		}

		rules = append(rules, firewallRuleSpec{
			Ports:      ports,
			AllowedIPs: allowedIPs,
			StartPort:  start,
			EndPort:    end,
		})
	}

	return rules, nil
}

func validatePortConflicts(secureLinks []secureLinkSpec, firewallRules []firewallRuleSpec) error {
	portOwners := map[int]string{}
	for _, link := range secureLinks {
		if owner, exists := portOwners[link.Port]; exists {
			return breverrors.NewValidationError(fmt.Sprintf("secure link %q conflicts with %s on port %d", link.Name, owner, link.Port))
		}
		portOwners[link.Port] = fmt.Sprintf("secure link %q", link.Name)
	}

	for i := 0; i < len(firewallRules); i++ {
		for port := firewallRules[i].StartPort; port <= firewallRules[i].EndPort; port++ {
			if owner, exists := portOwners[port]; exists {
				return breverrors.NewValidationError(fmt.Sprintf("firewall rule %q conflicts with %s on port %d", firewallRules[i].Ports, owner, port))
			}
		}

		for j := i + 1; j < len(firewallRules); j++ {
			if firewallRulesOverlap(firewallRules[i], firewallRules[j]) {
				return breverrors.NewValidationError(fmt.Sprintf("firewall rule %q conflicts with firewall rule %q", firewallRules[i].Ports, firewallRules[j].Ports))
			}
		}
	}

	return nil
}

func firewallRulesOverlap(a, b firewallRuleSpec) bool {
	return !(a.EndPort < b.StartPort || b.EndPort < a.StartPort)
}

func parsePortRange(raw string) (int, int, error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.Contains(trimmed, "-") {
		port, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port %q must be a valid number", raw))
		}
		if port < 1 || port > 65535 {
			return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port %q must be between 1 and 65535", raw))
		}
		return port, port, nil
	}

	parts := strings.Split(trimmed, "-")
	if len(parts) != 2 {
		return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port range %q must use the format start-end", raw))
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port range %q has an invalid start port", raw))
	}
	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port range %q has an invalid end port", raw))
	}
	if start < 1 || start > 65535 || end < 1 || end > 65535 {
		return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port range %q must stay within 1-65535", raw))
	}
	if start >= end {
		return 0, 0, breverrors.NewValidationError(fmt.Sprintf("port range %q must have start < end", raw))
	}

	return start, end, nil
}

func resolveDockerCompose(launchableStore LaunchableCmdStore, opts launchableCreateOptions) (*store.DockerCompose, error) {
	if opts.Mode != "compose" {
		return nil, nil
	}

	req := store.ValidateDockerComposeRequest{}
	build := &store.DockerCompose{
		JupyterInstall: opts.Jupyter,
	}

	switch {
	case opts.ComposeURL != "":
		err := validateFileURL(opts.ComposeURL)
		if err != nil {
			return nil, err
		}
		req.FileURL = replaceBlobWithRaw(strings.TrimSpace(opts.ComposeURL))
		build.FileURL = req.FileURL
	case opts.ComposeFilePath != "":
		content, err := os.ReadFile(opts.ComposeFilePath)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return nil, breverrors.NewValidationError("--compose-file cannot be empty")
		}
		req.YamlString = string(content)
		build.YamlString = req.YamlString
	case opts.ComposeYAML != "":
		if strings.TrimSpace(opts.ComposeYAML) == "" {
			return nil, breverrors.NewValidationError("--compose-yaml cannot be empty")
		}
		req.YamlString = opts.ComposeYAML
		build.YamlString = opts.ComposeYAML
	}

	_, err := launchableStore.ValidateDockerCompose(req)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return build, nil
}

func buildCreateEnvironmentLaunchableRequest(
	opts launchableCreateOptions,
	instanceType gpusearch.InstanceType,
	storageValue string,
	fileURL string,
	lifecycleScript string,
	dockerCompose *store.DockerCompose,
	secureLinks []secureLinkSpec,
	firewallRules []firewallRuleSpec,
) store.CreateEnvironmentLaunchableRequest {
	createWorkspaceRequest := store.CreateEnvironmentLaunchableWorkspaceRequest{
		WorkspaceGroupID: instanceType.WorkspaceGroups[0].ID,
		InstanceType:     instanceType.Type,
		Storage:          storageValue,
	}

	if len(firewallRules) > 0 {
		createWorkspaceRequest.FirewallRules = make([]store.CreateFirewallRule, 0, len(firewallRules))
		for _, rule := range firewallRules {
			createWorkspaceRequest.FirewallRules = append(createWorkspaceRequest.FirewallRules, store.CreateFirewallRule{
				Port:       rule.Ports,
				AllowedIPs: rule.AllowedIPs,
			})
		}
	}

	buildRequest := store.CreateEnvironmentLaunchableBuildRequest{}
	if len(secureLinks) > 0 {
		buildRequest.Ports = make([]store.LaunchablePort, 0, len(secureLinks))
		for _, link := range secureLinks {
			var labels map[string]string
			if link.CTA {
				labels = map[string]string{
					"cta-enabled": "true",
					"cta-value":   defaultString(link.CTALabel, link.Name),
				}
			}
			buildRequest.Ports = append(buildRequest.Ports, store.LaunchablePort{
				Name:   link.Name,
				Port:   strconv.Itoa(link.Port),
				Labels: labels,
			})
		}
	}

	if opts.Mode == "vm" {
		buildRequest.VMBuild = &store.VMBuild{
			ForceJupyterInstall: opts.Jupyter,
		}
		if lifecycleScript != "" {
			buildRequest.VMBuild.LifeCycleScriptAttr = &store.LifeCycleScriptAttr{
				Script: lifecycleScript,
			}
		}
	} else if dockerCompose != nil {
		buildRequest.DockerCompose = dockerCompose
	}

	var file *store.LaunchableFile
	if fileURL != "" {
		file = &store.LaunchableFile{
			URL:  fileURL,
			Path: "./",
		}
	}

	return store.CreateEnvironmentLaunchableRequest{
		Name:                   strings.TrimSpace(opts.Name),
		Description:            strings.TrimSpace(opts.Description),
		ViewAccess:             opts.ViewAccess,
		CreateWorkspaceRequest: createWorkspaceRequest,
		BuildRequest:           buildRequest,
		File:                   file,
	}
}

func parseFlagAssignments(raw string) (map[string]string, error) {
	assignments := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, breverrors.NewValidationError(fmt.Sprintf("invalid flag value %q; use comma-separated key=value pairs", raw))
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if key == "" {
			return nil, breverrors.NewValidationError(fmt.Sprintf("invalid flag value %q; keys cannot be empty", raw))
		}
		assignments[key] = value
	}
	return assignments, nil
}

func validateTunnelName(name string) string {
	if name == "" {
		return "Name is required"
	}
	if len(name) > 63 {
		return "Name must be 63 characters or fewer"
	}
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(name) {
		return "Name must start with a lowercase letter or digit"
	}
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(name) {
		return "Name may only contain lowercase letters, digits, and hyphens"
	}
	if !regexp.MustCompile(`[a-z0-9]$`).MatchString(name) {
		return "Name must end with a lowercase letter or digit"
	}
	return ""
}

func buildLaunchableDeployURL(launchableID string) string {
	consoleURL := config.GlobalConfig.GetConsoleURL()
	u, err := url.Parse(consoleURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(consoleURL, "/") + "/launchable/deploy?launchableID=" + launchableID
	}

	u.Path = "/launchable/deploy"
	query := u.Query()
	query.Set("launchableID", launchableID)
	u.RawQuery = query.Encode()

	return u.String()
}

func parseSizeToGiB(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	match := sizePattern.FindStringSubmatch(trimmed)
	if match == nil {
		return 0, breverrors.NewValidationError(fmt.Sprintf("unsupported size value %q", raw))
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}

	unit := strings.ToLower(match[2])
	switch unit {
	case "gib", "gb":
		return int(value), nil
	case "mib", "mb":
		return int(value / 1024), nil
	case "tib", "tb":
		return int(value * 1024), nil
	case "b":
		return int(value / (1024 * 1024 * 1024)), nil
	default:
		return 0, breverrors.NewValidationError(fmt.Sprintf("unsupported size unit in %q", raw))
	}
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
