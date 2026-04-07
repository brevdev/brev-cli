package hello

import "testing"

func TestShellOnboardingPollDone(t *testing.T) {
	t.Parallel()
	if !shellOnboardingPollDone(&OnboardingObject{HasRunBrevShell: true}) {
		t.Fatal("expected true when HasRunBrevShell is set")
	}
	if shellOnboardingPollDone(&OnboardingObject{HasRunBrevShell: false}) {
		t.Fatal("expected false when HasRunBrevShell is false")
	}
	if shellOnboardingPollDone(&OnboardingObject{HasRunBrevOpen: true, HasRunBrevShell: false}) {
		t.Fatal("HasRunBrevOpen alone must not end shell onboarding wait")
	}
	if shellOnboardingPollDone(nil) {
		t.Fatal("expected false for nil res")
	}
}
