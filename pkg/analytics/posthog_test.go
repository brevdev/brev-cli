package analytics

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/files"
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
