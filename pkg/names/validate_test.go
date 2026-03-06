package names

import (
	"strings"
	"testing"
)

func TestValidateNodeName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
	}{
		{"Valid", "my-dgx-spark", false, ""},
		{"WithDots", "node.local.1", false, ""},
		{"WithUnderscore", "my_node", false, ""},
		{"SingleChar", "a", false, ""},
		{"MaxLength", strings.Repeat("a", 63), false, ""},
		{"Spaces", "My Spark", true, "letters, digits"},
		{"ShellInjection", "$(whoami)", true, "letters, digits"},
		{"PathTraversal", "../etc/passwd", true, "letters, digits"},
		{"Backticks", "`rm -rf`", true, "letters, digits"},
		{"Semicolon", "a;rm -rf /", true, "letters, digits"},
		{"Pipe", "a|cat", true, "letters, digits"},
		{"Ampersand", "a&bg", true, "letters, digits"},
		{"LeadingHyphen", "-node", true, "start with"},
		{"LeadingDot", ".hidden", true, "start with"},
		{"TooLong", strings.Repeat("a", 64), true, "63 characters"},
		{"Empty", "", true, "name is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNodeName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
