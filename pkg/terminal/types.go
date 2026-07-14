package terminal

// Confirmer prompts for yes/no confirmation.
type Confirmer interface {
	ConfirmYesNo(label string) bool
}

// Selector prompts the user to choose from a list of items.
type Selector interface {
	Select(label string, items []string) string
}
