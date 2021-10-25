package version

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

func TestBuildVersionString(t *testing.T) {
	terminalStub := &terminal.Terminal{}

	want := "unknown"
	got, err := buildVersionString(terminalStub)

	if want != got || err != nil {
		t.Errorf(`buildVersionString() = %q, %v, want match for %#q, nil`, got, err, want)
	}
}
