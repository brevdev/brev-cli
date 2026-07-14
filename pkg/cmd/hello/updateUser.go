package hello

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/util"
)

func CompletedOnboardingIntro(user *entity.User, store HelloStore) error {
	newOnboardingStatus := make(map[string]interface{})
	newOnboardingStatus["cliOnboardingIntro"] = true

	_, err := store.UpdateUser(user.ID, &entity.UpdateUser{
		// username, name, and email are required fields, but we only care about onboarding status
		Username:       user.Username,
		Name:           user.Name,
		Email:          user.Email,
		OnboardingData: util.MapAppend(user.OnboardingData, newOnboardingStatus),
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
