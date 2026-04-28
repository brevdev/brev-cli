package analytics

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/google/uuid"
	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const posthogAPIKey = "phc_PWWXIQgQ31lXWMGI2dnTY3FyjBh7gPcMhlno1RLapLm"

// UserStore provides the current user ID for analytics tracking.
type UserStore interface {
	GetCurrentUserID() (string, error)
}

var (
	client     posthog.Client
	clientOnce sync.Once
	clientErr  error

	// Command timing
	commandStartTime time.Time

	// Stored command context for error-path capture
	storedCmd  *cobra.Command
	storedArgs []string
	storedUser string

	// userStore is set via SetUserStore so GetOrCreateAnalyticsID can resolve the real user ID.
	userStore UserStore
)

func getClient() (posthog.Client, error) {
	clientOnce.Do(func() {
		client, clientErr = posthog.NewWithConfig(posthogAPIKey, posthog.Config{})
	})
	return client, clientErr
}

// IsAnalyticsFeatureEnabled is the remote kill switch for CLI telemetry; gating capture lets us turn it off without a release.
func IsAnalyticsFeatureEnabled() bool {
	anonID := GetOrCreateAnalyticsID()
	if anonID == "" {
		return false
	}

	c, err := getClient()
	if err != nil {
		return false
	}

	result, err := c.IsFeatureEnabled(posthog.FeatureFlagPayload{
		Key:        "enable-cli-analytics",
		DistinctId: anonID,
	})
	if err != nil {
		return false
	}

	enabled, ok := result.(bool)
	return ok && enabled
}

// RecordCommandStart should be called from PersistentPreRunE to record the start time
// and store the command context for potential error-path capture.
func RecordCommandStart(cmd *cobra.Command, args []string) {
	commandStartTime = time.Now()
	storedCmd = cmd
	storedArgs = args
}

// IsAnalyticsEnabled defaults to true; DO_NOT_TRACK and BREV_NO_ANALYTICS override.
func IsAnalyticsEnabled() bool {
	if disabled, _ := IsDisabledByEnv(); disabled {
		return false
	}
	settings := readSettings()
	if settings.AnalyticsEnabled == nil {
		return true
	}
	return *settings.AnalyticsEnabled
}

func IsDisabledByEnv() (disabled bool, varName string) {
	if os.Getenv("DO_NOT_TRACK") == "1" {
		return true, "DO_NOT_TRACK"
	}
	if os.Getenv("BREV_NO_ANALYTICS") == "1" {
		return true, "BREV_NO_ANALYTICS"
	}
	return false, ""
}

// SetAnalyticsPreference persists the user's analytics preference.
func SetAnalyticsPreference(enabled bool) error {
	fs := files.AppFs
	home, err := getHomeDir()
	if err != nil {
		return err
	}
	settings, err := files.ReadPersonalSettings(fs, home)
	if err != nil {
		return fmt.Errorf("reading personal settings: %w", err)
	}
	settings.AnalyticsEnabled = &enabled
	if err := files.WritePersonalSettings(fs, home, settings); err != nil {
		return fmt.Errorf("writing personal settings: %w", err)
	}
	return nil
}

// SetUserStore configures the store used to resolve the current user ID.
func SetUserStore(s UserStore) {
	userStore = s
}

// GetOrCreateAnalyticsID returns the user's distinct ID for tracking.
// It prefers the real user ID from the store if available, falling back to a stable anonymous UUID.
func GetOrCreateAnalyticsID() string {
	if userStore != nil {
		if uid, err := userStore.GetCurrentUserID(); err == nil && uid != "" {
			return uid
		}
	}

	fs := files.AppFs
	home, err := getHomeDir()
	if err != nil {
		return ""
	}
	settings, err := files.ReadPersonalSettings(fs, home)
	if err != nil {
		return ""
	}
	if settings.AnalyticsID != "" {
		return settings.AnalyticsID
	}
	settings.AnalyticsID = uuid.New().String()
	_ = files.WritePersonalSettings(fs, home, settings)
	return settings.AnalyticsID
}

// IdentifyUser links the anonymous analytics ID to a real user ID using PostHog Alias.
func IdentifyUser(userID string) {
	if !IsAnalyticsEnabled() {
		return
	}

	anonID := GetOrCreateAnalyticsID()
	if anonID == "" || userID == "" {
		return
	}

	c, err := getClient()
	if err != nil {
		return
	}

	_ = c.Enqueue(posthog.Alias{
		DistinctId: userID,
		Alias:      anonID,
	})
}

// CaptureCommand captures a CLI command invocation event (success path).
func CaptureCommand(userID string, cmd *cobra.Command, args []string) {
	// Store for potential error-path capture
	storedCmd = cmd
	storedArgs = args
	storedUser = userID

	captureEvent(userID, cmd, args, true)
}

// CaptureCommandError captures a CLI command failure event from main.go.
// Uses stored command context from PersistentPreRunE/PersistentPostRunE.
func CaptureCommandError() {
	if storedCmd == nil {
		return
	}
	captureEvent(storedUser, storedCmd, storedArgs, false)
}

func captureEvent(userID string, cmd *cobra.Command, args []string, succeeded bool) {
	if !IsAnalyticsEnabled() {
		return
	}
	if !IsAnalyticsFeatureEnabled() {
		return
	}

	// Resolve the analytics ID lazily, only after gates pass — avoids writing a
	// persistent UUID to ~/.brev/personal_settings.json for opted-out users.
	if userID == "" {
		userID = GetOrCreateAnalyticsID()
	}
	if userID == "" {
		return
	}

	c, err := getClient()
	if err != nil {
		return
	}

	// Flags
	flagMap := make(map[string]interface{})
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flagMap[f.Name] = f.Value.String()
	})

	// Parent process
	parentName, parentCmdline := getParentProcessInfo()

	// CWD
	cwd, _ := os.Getwd()

	// Duration
	var durationMs int64
	if !commandStartTime.IsZero() {
		durationMs = time.Since(commandStartTime).Milliseconds()
	}

	// TTY / piped detection
	stdinStat, _ := os.Stdin.Stat()
	stdoutStat, _ := os.Stdout.Stat()
	isTTY := (stdoutStat.Mode() & os.ModeCharDevice) != 0
	isStdinPiped := (stdinStat.Mode() & os.ModeCharDevice) == 0
	isStdoutPiped := (stdoutStat.Mode() & os.ModeCharDevice) == 0

	// CI detection
	isCI := detectCI()

	// SSH session detection
	isSSH := os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != ""

	// GPU info
	gpuInfo := getGPUInfo()

	properties := posthog.NewProperties().
		// Command info
		Set("command", cmd.CommandPath()).
		Set("args", strings.Join(args, " ")).
		Set("flags", flagMap).
		Set("succeeded", succeeded).
		Set("duration_ms", durationMs).
		// System
		Set("os", runtime.GOOS).
		Set("arch", runtime.GOARCH).
		Set("num_cpus", runtime.NumCPU()).
		Set("go_version", runtime.Version()).
		Set("cli_version", version.Version).
		// Context
		Set("cwd", cwd).
		Set("parent_process", parentName).
		Set("parent_cmdline", parentCmdline).
		// Terminal
		Set("is_tty", isTTY).
		Set("is_stdin_piped", isStdinPiped).
		Set("is_stdout_piped", isStdoutPiped).
		Set("shell", os.Getenv("SHELL")).
		Set("terminal", os.Getenv("TERM")).
		// Environment
		Set("is_ci", isCI).
		Set("is_ssh", isSSH).
		Set("locale", getLocale()).
		Set("timezone", getTimezone()).
		// GPU
		Set("gpu_info", gpuInfo)

	_ = c.Enqueue(posthog.Capture{
		DistinctId: userID,
		Event:      "cli_command",
		Properties: properties,
	})
}

// CaptureFeedback sends a brev-cli-feedback event to PostHog.
// This is sent regardless of analytics opt-in since the user explicitly chose to send feedback.
func CaptureFeedback(userID, message string) {
	if userID == "" {
		userID = GetOrCreateAnalyticsID()
	}
	if userID == "" {
		return
	}

	c, err := getClient()
	if err != nil {
		return
	}

	_ = c.Enqueue(posthog.Capture{
		DistinctId: userID,
		Event:      "brev-cli-feedback",
		Properties: posthog.NewProperties().
			Set("message", message).
			Set("os", runtime.GOOS).
			Set("arch", runtime.GOARCH).
			Set("cli_version", version.Version),
	})
}

// Close flushes any pending events and closes the PostHog client.
func Close() {
	if client != nil {
		_ = client.Close()
	}
}

func readSettings() *files.PersonalSettings {
	fs := files.AppFs
	home, err := getHomeDir()
	if err != nil {
		return &files.PersonalSettings{}
	}
	settings, err := files.ReadPersonalSettings(fs, home)
	if err != nil {
		return &files.PersonalSettings{}
	}
	return settings
}

func getHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return home, nil
}

func detectCI() bool {
	ciVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"JENKINS_URL",
		"BUILDKITE",
		"CODEBUILD_BUILD_ID",
		"TF_BUILD",
		"BITBUCKET_PIPELINE",
	}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

func getLocale() string {
	if lang := os.Getenv("LANG"); lang != "" {
		return lang
	}
	if lcAll := os.Getenv("LC_ALL"); lcAll != "" {
		return lcAll
	}
	return ""
}

func getTimezone() string {
	zone, _ := time.Now().Zone()
	return zone
}

func getGPUInfo() string {
	type result struct {
		out string
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{out: getGPUInfoSync()}
	}()
	select {
	case r := <-ch:
		return r.out
	case <-time.After(100 * time.Millisecond):
		return ""
	}
}

func getGPUInfoSync() string {
	out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,driver_version,count", "--format=csv,noheader,nounits").Output() // #nosec G204
	if err != nil {
		if runtime.GOOS == "darwin" {
			return getAppleGPUInfo()
		}
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getAppleGPUInfo() string {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType", "-detailLevel", "mini").Output() // #nosec G204
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	var gpuLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Chipset Model:") || strings.HasPrefix(trimmed, "VRAM") || strings.HasPrefix(trimmed, "Total Number of Cores:") {
			gpuLines = append(gpuLines, trimmed)
		}
	}
	return strings.Join(gpuLines, "; ")
}
