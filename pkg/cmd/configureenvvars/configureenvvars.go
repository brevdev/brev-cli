package configureenvvars

import (
	"fmt"
	"os"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

const (
	BREV_WORKSPACE_ENV_PATH  = "/home/brev/workspace/.env"
	BREV_MANGED_ENV_VARS_KEY = "BREV_MANAGED_ENV_VARS"
)

type ConfigureEnvVarsStore interface {
	GetFileAsString(path string) (string, error)
}

func NewCmdConfigureEnvVars(_ *terminal.Terminal, cevStore ConfigureEnvVarsStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "configure-env-vars",
		DisableFlagsInUseLine: true,
		Short:                 "configure env vars in supported shells",
		Long:                  "Import your IDE config",
		Example:               "",
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := RunConfigureEnvVars(cevStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			fmt.Print(output)
			return nil
		},
	}

	return cmd
}

func RunConfigureEnvVars(cevStore ConfigureEnvVarsStore) (string, error) {
	brevEnvsString := os.Getenv(BREV_MANGED_ENV_VARS_KEY)
	envFileContents, err := cevStore.GetFileAsString(BREV_WORKSPACE_ENV_PATH)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return generateExportString(brevEnvsString, envFileContents), nil
}

func generateExportString(brevEnvsString, envFileContents string) string {
	if brevEnvsString == "" && envFileContents == "" {
		return ""
	}
	brevEnvKeys := strings.Split(brevEnvsString, ",")

	envFileKeys := getKeysFromEnvFile(envFileContents)
	newBrevEnvKeys := strings.Join(envFileKeys, ",")
	newBrevEnvKeysEntry := ""
	if newBrevEnvKeys != "" {
		newBrevEnvKeysEntry = BREV_MANGED_ENV_VARS_KEY + "=" + newBrevEnvKeys
	}

	// todo parameterize by shell
	envCmdOutput := []string{}
	envCmdOutput = addUnsetEntriesToOutput(brevEnvKeys, envFileKeys, envCmdOutput)
	// todo parameterize by shell and check for export prefix
	envCmdOutput = append(envCmdOutput, addExportPrefix(strings.Split(envFileContents, "\n"))...)
	envCmdOutput = append(envCmdOutput, addExportPrefix([]string{newBrevEnvKeysEntry})...)
	return strings.Join(filterEmpty(envCmdOutput), "\n")
}

func addExportPrefix(envFileLines []string) []string {
	if len(envFileLines) == 0 {
		return envFileLines
	}
	out := []string{}
	for _, line := range envFileLines {
		if strings.HasPrefix(line, "export ") || line == "" {
			out = append(out, line)
		} else {
			out = append(out, "export "+line)
		}
	}
	return out
}

func filterEmpty[T comparable](l []T) []T {
	var zero T
	out := []T{}
	for _, i := range l {
		if i != zero {
			out = append(out, i)
		}
	}
	return out
}

func contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func getKeysFromEnvFile(content string) []string {
	output := []string{}
	for _, k := range strings.Split(content, "\n") {
		k = strings.TrimPrefix(k, "export ")
		if strings.Contains(k, "=") {
			output = append(output, strings.Split(k, "=")[0])
		}
	}
	return output
}

// this may be a good place to parameterize bby shell
func addUnsetEntriesToOutput(currentEnvs, newEnvs, output []string) []string {
	for _, envKey := range currentEnvs {
		if !contains(newEnvs, envKey) && envKey != "" {
			output = append(output, "unset "+envKey)
		}
	}
	return output
}
