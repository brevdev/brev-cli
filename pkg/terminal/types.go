package terminal

// Confirmer prompts for yes/no confirmation.
type Confirmer interface {
	ConfirmYesNo(label string) bool
}

// Selector prompts the user to choose from a list of items.
type Selector interface {
	Select(label string, items []string) string
}

// Inputter prompts the user for free-form text input.
type Inputter interface {
	Input(pc PromptContent) string
}

// LineInputter reads one line without an interactive terminal UI.
type LineInputter interface {
	InputLine(t *Terminal, label string) (string, error)
}
