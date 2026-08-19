package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessCommandsExcludesHiddenCommands(t *testing.T) {
	root := &cobra.Command{Use: "brev"}
	visible := &cobra.Command{
		Use:         "visible",
		Annotations: map[string]string{"access": ""},
		Run:         func(_ *cobra.Command, _ []string) {},
	}
	hidden := &cobra.Command{
		Use:         "hidden",
		Annotations: map[string]string{"access": ""},
		Hidden:      true,
		Run:         func(_ *cobra.Command, _ []string) {},
	}
	root.AddCommand(visible, hidden)

	commands := accessCommands(root)
	require.Len(t, commands, 1)
	assert.Same(t, visible, commands[0])
}
