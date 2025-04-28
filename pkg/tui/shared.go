package tui

import (
	"github.com/brevdev/brev-cli/pkg/store"
	tea "github.com/charmbracelet/bubbletea"
)

type instanceTypesLoadedMsg struct {
    types *store.InstanceTypeResponse
    err   error
}

func loadInstanceTypes(s *store.AuthHTTPStore) tea.Cmd {
    return func() tea.Msg {
        types, err := s.GetInstanceTypes()
        return instanceTypesLoadedMsg{types: types, err: err}
    }
} 