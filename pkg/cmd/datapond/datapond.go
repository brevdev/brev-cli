package datapond

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

const (
	configFile = "/etc/default/datapond"
)

type DatapondConfig struct {
	AWSAccessKeyID       string        `json:"aws_access_key_id"`
	AWSSecretAccessKey   string        `json:"aws_secret_access_key"`
	AWSSessionToken      string        `json:"aws_session_token"`
	DatapondUser         string        `json:"datapond_user"`
	DatapondDir          string        `json:"datapond_dir"`
	DatapondBucketType   string        `json:"datapond_bucket_type"`
	DatapondBucketPath   string        `json:"datapond_bucket_path"`
	DatapondBucketRegion string        `json:"datapond_bucket_region"`
	DatapondSyncInterval time.Duration `json:"datapond_sync_interval"`
}

func NewCmdDatapond(t *terminal.Terminal) *cobra.Command {
	var targetDir string

	cmd := &cobra.Command{
		Use:   "datapond",
		Short: "Continuous sync for datapond",
		Long:  "Datapond is a tool for continuously syncing data from local storage to the cloud bucket.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setupContinuousSync()
		},
	}

	cmd.Flags().StringVarP(&targetDir, "target_dir", "t", "", "Local directory to sync from (required)")
	err := cmd.MarkFlagRequired("target_dir")
	if err != nil {
		return nil
	}

	return cmd
}

func setupContinuousSync() error {
	var datapondConfig *DatapondConfig
	var err error
	if datapondConfig, err = loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// check if service is already running
	if !isServiceRunning() {
		fmt.Println("Datapond service is not running, starting...")
		if err := setupService(); err != nil {
			return fmt.Errorf("failed to setup service: %w", err)
		}
	}

	for {
		if err := syncOnce(datapondConfig.DatapondDir); err != nil {
			fmt.Printf("Error syncing: %v\n", err)
		}
		syncInterval := datapondConfig.DatapondSyncInterval
		fmt.Printf("Sleeping for %d seconds\n", syncInterval)
		time.Sleep(datapondConfig.DatapondSyncInterval * time.Second)
	}
}

func loadConfig() (*DatapondConfig, error) {
	file, err := os.ReadFile(configFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var config DatapondConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &config, nil
}

func isServiceRunning() bool {
	if isSystemdAvailable() {
		cmd := exec.Command("systemctl", "is-active", "--quiet", "datapond")
		err := cmd.Run()
		return err == nil
	} else if isSupervisordAvailable() {
		cmd := exec.Command("supervisorctl", "status", "datapond")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), "RUNNING")
	}
	return false
}

func syncOnce(targetDir string) error {
	bucketPath := targetDir
	if bucketPath == "" {
		return fmt.Errorf("DATAPOND_BUCKET_PATH is not set")
	}

	cmd := exec.Command("s5cmd", "sync", "--delete", filepath.Join(targetDir, "*"), fmt.Sprintf("s3://%s/", bucketPath)) //nolint:gosec // ok to use s5cmd
	if err := cmd.Run(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func setupService() error {
	if isSystemdAvailable() {
		return setupSystemd()
	} else if isSupervisordAvailable() {
		return setupSupervisord()
	}
	return fmt.Errorf("neither systemd nor supervisord is available")
}

func isSystemdAvailable() bool {
	cmd := exec.Command("systemctl", "status")
	err := cmd.Run()
	return err == nil
}

func setupSystemd() error {
	unitContent := `[Unit]
Description=datapond continuous-sync
After=network.target

[Service]
ExecStart=brev datapond --target_dir %s
User=%s

[Install]
WantedBy=multi-user.target
`
	unitContent = fmt.Sprintf(unitContent, os.Getenv("DATAPOND_DIR"), os.Getenv("USER"))

	if err := os.WriteFile("/etc/systemd/system/datapond.service", []byte(unitContent), 0o600); err != nil {
		return fmt.Errorf("failed to write systemd unit file: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "datapond.service"},
		{"systemctl", "start", "datapond.service"},
	}

	for _, cmdArgs := range cmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec // ok to use systemctl
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %v: %w", cmdArgs, err)
		}
	}

	return nil
}

func isSupervisordAvailable() bool {
	cmd := exec.Command("pgrep", "-f", "supervisord")
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) != ""
}

func setupSupervisord() error {
	confFile := "/etc/supervisor/conf.d/datapond.conf"
	confContent := fmt.Sprintf(`[program:datapond]
command=brev datapond --target_dir %s
autorestart=true
user=%s
redirect_stderr=true
environment=HOME=%s
`, os.Getenv("DATAPOND_DIR"), os.Getenv("USER"), os.Getenv("HOME"))

	if err := os.WriteFile(confFile, []byte(confContent), 0o600); err != nil {
		return fmt.Errorf("failed to write supervisord config file: %w", err)
	}

	commands := []string{
		"supervisorctl reread",
		"supervisorctl update",
		"supervisorctl restart datapond",
	}

	for _, cmd := range commands {
		parts := strings.Fields(cmd)
		command := exec.Command("sudo", parts...) //nolint:gosec // ok to use sudo
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("failed to run '%s': %w", cmd, err)
		}
	}

	fmt.Println("Supervisord service installed and started.")
	return nil
}
