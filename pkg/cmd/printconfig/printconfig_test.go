package printconfig

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
)

func TestPrintConfigOutputFormatting(t *testing.T) {
	preview := refresh.SSHConfigPreview{
		IncludeDirective: `Include "/home/test/.brev/ssh/config"`,
		BrevConfigPath:   "/home/test/.brev/ssh/config",
		BrevConfig:       "Host test-box\n  Hostname example.com\n",
	}

	got := "# Add this to your SSH config (for example via Home Manager)\n" +
		preview.IncludeDirective + "\n" +
		"# Brev-managed SSH config at " + preview.BrevConfigPath + "\n" +
		preview.BrevConfig

	want := "# Add this to your SSH config (for example via Home Manager)\n" +
		"Include \"/home/test/.brev/ssh/config\"\n" +
		"# Brev-managed SSH config at /home/test/.brev/ssh/config\n" +
		"Host test-box\n  Hostname example.com\n"

	if got != want {
		t.Fatalf("unexpected output\nwant:\n%s\ngot:\n%s", want, got)
	}
}
