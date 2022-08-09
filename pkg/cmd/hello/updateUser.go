package hello

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/util"
)

// currentOnboardingStatus, err := user.GetOnboardingData()
// if err != nil {
// 	return breverrors.WrapAndTrace(err)
// }

func SkippedOnboarding(user *entity.User, store HelloStore) error {
	newOnboardingStatus := make(map[string]interface{})
	newOnboardingStatus["cliOnboardingSkipped"] = true

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

func CompletedOnboardingLs(user *entity.User, store HelloStore) error {
	newOnboardingStatus := make(map[string]interface{})
	newOnboardingStatus["cliOnboardingLs"] = true

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

func CompletedOnboardingShell(user *entity.User, store HelloStore) error {
	newOnboardingStatus := make(map[string]interface{})
	newOnboardingStatus["cliOnboardingBrevShell"] = true

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

func CompletedOnboardingOpen(user *entity.User, store HelloStore) error {
	newOnboardingStatus := make(map[string]interface{})
	newOnboardingStatus["cliOnboardingBrevOpen"] = true

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

func CompletedOnboarding(user *entity.User, store HelloStore) error {
	newOnboardingStatus := make(map[string]interface{})
	newOnboardingStatus["cliOnboardingCompleted"] = true

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
