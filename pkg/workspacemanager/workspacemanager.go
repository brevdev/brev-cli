package workspacemanager

import "fmt"

type WorkspaceManager struct{}

func NewWorkspaceManager() *WorkspaceManager {
	return &WorkspaceManager{}
}

func (w WorkspaceManager) Start(workspaceID string) error {
	fmt.Println(workspaceID)
	return nil
}

func (w WorkspaceManager) Stop(_ string) error {
	return nil
}

func (w WorkspaceManager) Reset(_ string) error {
	return nil
}
