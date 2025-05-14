package drew

import lipgloss "github.com/charmbracelet/lipgloss"

var (
	textColorNormalTitle       = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"}
	textColorNormalDescription = lipgloss.AdaptiveColor{Light: "#a0a59f", Dark: "#777777"}

	textColorSelectedTitle       = lipgloss.AdaptiveColor{Light: "#7af86f", Dark: "#7af86f"}
	textColorSelectedDescription = lipgloss.AdaptiveColor{Light: "#7df86f", Dark: "#58b460"}
	borderColorSelected          = lipgloss.AdaptiveColor{Light: "#9aff93", Dark: "#58b45e"}

	textColorDimmedTitle       = lipgloss.AdaptiveColor{Light: "#9fa59f", Dark: "#777777"}
	textColorDimmedDescription = lipgloss.AdaptiveColor{Light: "#b8c2b8", Dark: "#4D4D4D"}

	backgroundColorHeader = lipgloss.AdaptiveColor{Light: "#76b900", Dark: "#76b900"}
	textColorHeader       = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"}
)
