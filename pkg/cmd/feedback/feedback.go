package feedback

import (
	"strings"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type FeedbackStore interface {
	GetCurrentUser() (*entity.User, error)
}

func NewCmdFeedback(t *terminal.Terminal, store FeedbackStore) *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "feedback [message]",
		DisableFlagsInUseLine: true,
		Short:                 "Send feedback to the Brev team",
		Long:                  "Send feedback, bug reports, or feature requests to the Brev team",
		Example:               "brev feedback \"I love this tool!\"\nbrev feedback",
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" && len(args) > 0 {
				message = strings.Join(args, " ")
			}
			if message == "" {
				message = terminal.PromptGetInput(terminal.PromptContent{
					Label:    "What feedback do you have for us?",
					ErrorMsg: "Please enter some feedback",
				})
			}

			userID := analytics.GetOrCreateAnalyticsID()
			if user, err := store.GetCurrentUser(); err == nil && user != nil {
				userID = user.ID
			}

			analytics.CaptureFeedback(userID, message)

			t.Vprintf("%s", t.Green("Thanks for your feedback! We really appreciate it.\n"))
			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Feedback message (skips interactive prompt)")

	return cmd
}
