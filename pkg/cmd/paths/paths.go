package paths

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	// breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func NewCmdApprove(_ *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{},
		Use:         "paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := GetPaths()
			fmt.Println(paths)
			return nil
		},
	}

	return cmd
}

func GetPaths() string {
	vscodePaths := GetVsCodePaths()
	return strings.Join(vscodePaths, ":")
}

func GetVsCodePaths() []string {
	fi, err := ioutil.ReadDir("/home/brev/.vscode-server/bin")
	if err != nil {
		return []string{}
	}
	paths := []string{}
	for _, f := range fi {
		paths = append(paths, fmt.Sprintf("/home/brev/.vscode-server/bin/%s/bin/remote-cli", f.Name()))
	}
	return paths
}
