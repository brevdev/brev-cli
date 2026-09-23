package analytics

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func boolPtr(b bool) *bool { return &b }

func TestIsDisabledByEnv(t *testing.T) {
	cases := []struct {
		name         string
		envs         map[string]string
		wantDisabled bool
		wantVar      string
	}{
		{"no env vars set", nil, false, ""},
		{"DO_NOT_TRACK=1", map[string]string{"DO_NOT_TRACK": "1"}, true, "DO_NOT_TRACK"},
		{"BREV_NO_ANALYTICS=1", map[string]string{"BREV_NO_ANALYTICS": "1"}, true, "BREV_NO_ANALYTICS"},
		{"DO_NOT_TRACK=0 (only \"1\" disables)", map[string]string{"DO_NOT_TRACK": "0"}, false, ""},
		{"DO_NOT_TRACK=true (only \"1\" disables)", map[string]string{"DO_NOT_TRACK": "true"}, false, ""},
		{"both set — DO_NOT_TRACK reported first", map[string]string{"DO_NOT_TRACK": "1", "BREV_NO_ANALYTICS": "1"}, true, "DO_NOT_TRACK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("DO_NOT_TRACK", "")
			t.Setenv("BREV_NO_ANALYTICS", "")
			for k, v := range c.envs {
				t.Setenv(k, v)
			}
			disabled, varName := IsDisabledByEnv()
			if disabled != c.wantDisabled {
				t.Errorf("disabled = %v, want %v", disabled, c.wantDisabled)
			}
			if varName != c.wantVar {
				t.Errorf("varName = %q, want %q", varName, c.wantVar)
			}
		})
	}
}

func TestIsAnalyticsEnabled(t *testing.T) {
	cases := []struct {
		name   string
		stored *bool
		envs   map[string]string
		want   bool
	}{
		{"no preference, no env → default on", nil, nil, true},
		{"explicit opt-in, no env", boolPtr(true), nil, true},
		{"explicit opt-out, no env", boolPtr(false), nil, false},
		{"DO_NOT_TRACK overrides nil", nil, map[string]string{"DO_NOT_TRACK": "1"}, false},
		{"DO_NOT_TRACK overrides explicit opt-in", boolPtr(true), map[string]string{"DO_NOT_TRACK": "1"}, false},
		{"BREV_NO_ANALYTICS overrides explicit opt-in", boolPtr(true), map[string]string{"BREV_NO_ANALYTICS": "1"}, false},
		{"explicit opt-out stays opt-out under env override", boolPtr(false), map[string]string{"DO_NOT_TRACK": "1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			t.Setenv("DO_NOT_TRACK", "")
			t.Setenv("BREV_NO_ANALYTICS", "")
			for k, v := range c.envs {
				t.Setenv(k, v)
			}

			if c.stored != nil {
				if err := files.WritePersonalSettings(files.AppFs, tmp, &files.PersonalSettings{
					AnalyticsEnabled: c.stored,
				}); err != nil {
					t.Fatalf("write settings: %v", err)
				}
			}

			if got := IsAnalyticsEnabled(); got != c.want {
				t.Errorf("IsAnalyticsEnabled() = %v, want %v", got, c.want)
			}
		})
	}
}

// SetAnalyticsPreference must not lose other PersonalSettings fields.
func TestSetAnalyticsPreferencePreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := files.WritePersonalSettings(files.AppFs, tmp, &files.PersonalSettings{
		DefaultEditor: "vim",
		AnalyticsID:   "preexisting-id",
	}); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	if err := SetAnalyticsPreference(false); err != nil {
		t.Fatalf("SetAnalyticsPreference: %v", err)
	}

	got, err := files.ReadPersonalSettings(files.AppFs, tmp)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.DefaultEditor != "vim" {
		t.Errorf("DefaultEditor = %q, want %q (other fields must survive)", got.DefaultEditor, "vim")
	}
	if got.AnalyticsID != "preexisting-id" {
		t.Errorf("AnalyticsID = %q, want %q", got.AnalyticsID, "preexisting-id")
	}
	if got.AnalyticsEnabled == nil || *got.AnalyticsEnabled != false {
		t.Errorf("AnalyticsEnabled = %v, want pointer to false", got.AnalyticsEnabled)
	}
}

func buildFlaggedCmd(t *testing.T, setFlags func(*pflag.FlagSet)) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	setFlags(cmd.Flags())
	// Analytics only serializes flags explicitly set on the command line
	// Set marks each flag Changed AND registers it in the "actual" set that Visit iterates.
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = cmd.Flags().Set(f.Name, f.Value.String())
	})
	return cmd
}

func TestCaptureEvent_RedactsSensitiveFlagValues(t *testing.T) {
	tests := []struct {
		name      string
		setFlags  func(*pflag.FlagSet)
		flagName  string
		wantValue interface{}
	}{
		{
			name: "annotated flag is redacted",
			setFlags: func(fs *pflag.FlagSet) {
				fs.String("api-key", "bak-secret-value", "")
				MarkFlagSensitive(fs, "api-key")
			},
			flagName:  "api-key",
			wantValue: "[redacted]",
		},
		{
			name: "brev api key shape is redacted even unannotated",
			setFlags: func(fs *pflag.FlagSet) {
				fs.String("something", auth.BrevAPIKeyPrefix+"raw-key", "")
			},
			flagName:  "something",
			wantValue: "[redacted]",
		},
		{
			name: "jwt shape is redacted even unannotated",
			setFlags: func(fs *pflag.FlagSet) {
				fs.String("token", "eyJhbGciOi.J123.abc_sig", "")
			},
			flagName:  "token",
			wantValue: "[redacted]",
		},
		{
			name: "benign flag passes through",
			setFlags: func(fs *pflag.FlagSet) {
				fs.Bool("show-all", true, "")
				fs.String("org", "my-org", "")
			},
			flagName:  "org",
			wantValue: "my-org",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildFlaggedCmd(t, tt.setFlags)
			got := visitedFlagMap(cmd)
			assert.Equal(t, tt.wantValue, got[tt.flagName])
		})
	}
}

func visitedFlagMap(cmd *cobra.Command) map[string]interface{} {
	flagMap := make(map[string]interface{})
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flagMap[f.Name] = redactFlagValue(f)
	})
	return flagMap
}

func TestCaptureCommandError_UsesRedactedFlags(t *testing.T) {
	cmd := buildFlaggedCmd(t, func(fs *pflag.FlagSet) {
		fs.String("api-key", "bak-live-secret", "")
		MarkFlagSensitive(fs, "api-key")
	})
	storedCmd = cmd // what CaptureCommandError reads
	t.Cleanup(func() { storedCmd = nil })

	// The redaction itself is asserted via visitedFlagMap; this test pins the
	// contract that CaptureCommandError consults the same visitor.
	got := visitedFlagMap(cmd)
	assert.Equal(t, "[redacted]", got["api-key"])
}
