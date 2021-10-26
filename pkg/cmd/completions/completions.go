package completions

import "github.com/brevdev/brev-cli/pkg/brev_api"

type CompletionHelpers struct{}

func (c CompletionHelpers) GetWorkspaceNames() ([]string, error) {
	activeorg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil, err
	}

	client, _ := brev_api.NewClient()
	wss, err := client.GetWorkspaces(activeorg.ID)
	if err != nil {
		return nil, err
	}

	wsNames := []string{}
	for _, v := range wss {
		wsNames = append(wsNames, v.Name)
	}

	return wsNames, nil
}
