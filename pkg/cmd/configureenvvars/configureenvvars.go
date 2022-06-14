package configureenvvars

import (
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/collections" //nolint:typecheck
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

const (
	BREV_WORKSPACE_ENV_PATH  = "/home/brev/workspace/.env"
	BREV_MANGED_ENV_VARS_KEY = "BREV_MANAGED_ENV_VARS"
)

type envVars map[string]string

type ConfigureEnvVarsStore interface {
	GetFileAsString(path string) (string, error)
}

func NewCmdConfigureEnvVars(_ *terminal.Terminal, cevStore ConfigureEnvVarsStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "configure-env-vars",
		DisableFlagsInUseLine: true,
		Short:                 "configure env vars in supported shells",
		Long:                  "configure env vars in supported shells",
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

	envfileEntries := parse(envFileContents)
	envFileKeys := maps.Keys(envfileEntries)
	newBrevEnvKeys := strings.Join(envFileKeys, ",")
	newBrevEnvKeysEntry := ""
	if newBrevEnvKeys != "" {
		newBrevEnvKeysEntry = BREV_MANGED_ENV_VARS_KEY + "=" + newBrevEnvKeys
	}

	// todo parameterize by shell
	envCmdOutput := []string{}
	envCmdOutput = addUnsetEntriesToOutput(brevEnvKeys, envFileKeys, envCmdOutput)
	envCmdOutput = append(envCmdOutput, addExportPrefix(envfileEntries)...)
	if newBrevEnvKeysEntry != "" {
		envCmdOutput = append(envCmdOutput, "export "+newBrevEnvKeysEntry)
	}
	return strings.Join(collections.FilterEmpty(envCmdOutput), "\n")
}

func addExportPrefix(envFile envVars) []string {
	if len(envFile) == 0 {
		return []string{}
	}
	out := []string{}
	for k, v := range envFile {
		out = append(out, fmt.Sprintf("%s %s=%s", "export", k, v))
	}
	return out
}

// this may be a good place to parameterize bby shell
func addUnsetEntriesToOutput(currentEnvs, newEnvs, output []string) []string {
	for _, envKey := range currentEnvs {
		if !collections.Contains(newEnvs, envKey) && envKey != "" {
			output = append(output, "unset "+envKey)
		}
	}
	return output
}

// https://stackoverflow.com/a/38579502
func zip(elements []string, elementMap map[string]string) map[string]string {
	for i := 0; i < len(elements); i += 2 {
		elementMap[elements[i]] = elements[i+1]
	}
	return elementMap
}

func parse(content string) envVars {
	keyValPairs := []string{}
	lexer := lex("keys from env", content)
	scanning := true
	for scanning {
		token := lexer.nextItem()
		switch token.typ {
		case itemKey, itemValue:
			keyValPairs = append(keyValPairs, token.val)
		case itemError:
			return nil
		case itemEOF:
			scanning = false

		}

	}
	if len(keyValPairs)%2 != 0 {
		return nil
	}
	envVars := make(envVars)
	return zip(keyValPairs, envVars)
}
