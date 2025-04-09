package create

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	nvidiaGreen = "#76B900"
)

var (
	configBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("white")).
			Width(70).
			Align(lipgloss.Left).
			PaddingLeft(1).
			MarginBottom(0)

	configSelectedBoxStyle = configBoxStyle.Copy().
				BorderForeground(lipgloss.Color(nvidiaGreen)).
				BorderStyle(lipgloss.DoubleBorder()).
				Foreground(lipgloss.Color(nvidiaGreen))
) 