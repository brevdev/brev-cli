package open

import (
	"testing"
)

func TestIsEditorType(t *testing.T) {
	valid := []string{"code", "cursor", "windsurf", "terminal", "tmux", "claude", "codex"}
	for _, v := range valid {
		if !isEditorType(v) {
			t.Errorf("expected %q to be valid editor type", v)
		}
	}

	invalid := []string{"vim", "emacs", "vscode", "Code", "", "ssh"}
	for _, v := range invalid {
		if isEditorType(v) {
			t.Errorf("expected %q to NOT be valid editor type", v)
		}
	}
}

func TestGetEditorName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"code", "VSCode"},
		{"cursor", "Cursor"},
		{"windsurf", "Windsurf"},
		{"terminal", "Terminal"},
		{"tmux", "tmux"},
		{"claude", "Claude Code"},
		{"codex", "Codex"},
		{"unknown", "VSCode"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := getEditorName(tt.input)
			if got != tt.want {
				t.Errorf("getEditorName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
