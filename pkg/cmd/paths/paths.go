package paths

import (
	"fmt"
	"io/ioutil"
	// breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func GetPaths() string {
	return ""
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
