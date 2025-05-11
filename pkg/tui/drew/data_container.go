package drew

import (
	"github.com/charmbracelet/bubbles/spinner"
	lipgloss "github.com/charmbracelet/lipgloss"
)

type Container struct {
	Name   string
	Image  string
	Status ContainerStatus
}

type ContainerStatus int

const (
	ContainerStatusRunning ContainerStatus = iota
	ContainerStatusError
	ContainerStatusBuilding
	ContainerStatusStarting
	ContainerStatusStopping
	ContainerStatusStopped
	ContainerStatusDeleting
)

var containerStatuses = map[ContainerStatus]string{
	ContainerStatusRunning:  "Running",
	ContainerStatusError:    "Error",
	ContainerStatusBuilding: "Building",
	ContainerStatusStarting: "Starting",
	ContainerStatusStopping: "Stopping",
	ContainerStatusStopped:  "Stopped",
	ContainerStatusDeleting: "Deleting",
}

func (s ContainerStatus) Name() string {
	return containerStatuses[s]
}

func (s ContainerStatus) IsTemporary() bool {
	return s == ContainerStatusBuilding || s == ContainerStatusStarting || s == ContainerStatusStopping || s == ContainerStatusDeleting
}

func (s ContainerStatus) StatusView(spinner spinner.Model) string {
	var styledName string

	switch s {
	case ContainerStatusRunning:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("118")).Render(s.Name())
	case ContainerStatusError:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(s.Name())
	case ContainerStatusBuilding:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(s.Name())
	case ContainerStatusStarting:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(s.Name())
	case ContainerStatusStopping:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(s.Name())
	case ContainerStatusStopped:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Render(s.Name())
	case ContainerStatusDeleting:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(s.Name())
	default:
		styledName = s.Name()
	}

	if s.IsTemporary() {
		return styledName + " " + spinner.View()
	}
	return styledName
}
