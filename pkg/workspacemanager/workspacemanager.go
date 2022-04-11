package workspacemanager

import "fmt"

type WorkspaceManager struct{}

func NewWorkspaceManager() *WorkspaceManager {
	return &WorkspaceManager{}
}

func (w WorkspaceManager) Start(workspaceID string) error {
	fmt.Println("")
	return nil
}

func (w WorkspaceManager) Stop(workspaceID string) error {
	return nil
}

func (w WorkspaceManager) Reset(workspaceID string) error {
	return nil
}
