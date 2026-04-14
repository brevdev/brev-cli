package hello

import "testing"

type shellOnboardingPollDoneCase struct {
	name string
	res  *OnboardingObject
	want bool
}

var shellOnboardingPollDoneCases = []shellOnboardingPollDoneCase{
	{
		name: "hasRunBrevShell",
		res:  &OnboardingObject{HasRunBrevShell: true},
		want: true,
	},
	{
		name: "brevOpenOnly",
		res:  &OnboardingObject{HasRunBrevOpen: true, HasRunBrevShell: false},
		want: false,
	},
	{
		name: "nil",
		res:  nil,
		want: false,
	},
}

func TestShellOnboardingPollDone(t *testing.T) {
	t.Parallel()
	for _, c := range shellOnboardingPollDoneCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := shellOnboardingPollDone(c.res); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
