package drew

import (
	"github.com/charmbracelet/bubbles/spinner"
	lipgloss "github.com/charmbracelet/lipgloss"
)

type Environment struct {
	ID           string
	Name         string
	InstanceType InstanceType
	Storage      string
	Status       EnvironmentStatus
	Containers   []Container
	PortMappings []PortMapping
	Tunnels      []Tunnel
}

type EnvironmentStatus int

const (
	EnvironmentStatusRunning EnvironmentStatus = iota
	EnvironmentStatusError
	EnvironmentStatusBuilding
	EnvironmentStatusStarting
	EnvironmentStatusStopping
	EnvironmentStatusStopped
	EnvironmentStatusDeleting
)

var environmentStatusName = map[EnvironmentStatus]string{
	EnvironmentStatusRunning:  "Running",
	EnvironmentStatusError:    "Error",
	EnvironmentStatusBuilding: "Building",
	EnvironmentStatusStarting: "Starting",
	EnvironmentStatusStopping: "Stopping",
	EnvironmentStatusStopped:  "Stopped",
	EnvironmentStatusDeleting: "Deleting",
}

func (e EnvironmentStatus) Name() string {
	return environmentStatusName[e]
}

func (e EnvironmentStatus) IsTemporary() bool {
	return e == EnvironmentStatusBuilding || e == EnvironmentStatusStarting || e == EnvironmentStatusStopping || e == EnvironmentStatusDeleting
}

func (e EnvironmentStatus) StatusView(spinner spinner.Model) string {
	var styledName string

	switch e {
	case EnvironmentStatusRunning:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("118")).Render(e.Name())
	case EnvironmentStatusError:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(e.Name())
	case EnvironmentStatusBuilding:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(e.Name())
	case EnvironmentStatusStarting:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(e.Name())
	case EnvironmentStatusStopping:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(e.Name())
	case EnvironmentStatusStopped:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Render(e.Name())
	case EnvironmentStatusDeleting:
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(e.Name())
	default:
		styledName = e.Name()
	}

	if e.IsTemporary() {
		return styledName + " " + spinner.View()
	}
	return styledName
}

type PortMapping struct {
	HostPort   string
	PublicPort string
}

type Tunnel struct {
	HostPort  string
	PublicURL string
}
