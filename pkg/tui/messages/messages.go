package messages

import (
	"github.com/brevdev/brev-cli/pkg/store"
	tea "github.com/charmbracelet/bubbletea"
)

type InstanceTypesLoadedMsg struct {
	Types *store.InstanceTypeResponse
	Err   error
}

func LoadInstanceTypes(s *store.AuthHTTPStore) tea.Cmd {
	return func() tea.Msg {
		types, err := s.GetInstanceTypes()
		return InstanceTypesLoadedMsg{Types: types, Err: err}
	}
} 