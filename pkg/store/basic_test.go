package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBasicStore(t *testing.T) {
	s := MakeMockBasicStore()
	if !assert.NotNil(t, s) {
		return
	}
}

func MakeMockBasicStore() *BasicStore {
	return NewBasicStore()
}

func TestBasicStore_GetWindowsDir(t *testing.T) {
	type fields struct {
		envGetter func(string) string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test",
			fields: fields{
				envGetter: func(s string) string {
					return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/usr/lib/wsl/lib:/mnt/c/WINDOWS/system32:/mnt/c/WINDOWS:/mnt/c/WINDOWS/System32/Wbem:/mnt/c/WINDOWS/System32/WindowsPowerShell/v1.0/:/mnt/c/WINDOWS/System32/OpenSSH/:/mnt/c/Users/15854/AppData/Local/Microsoft/WindowsApps:/mnt/c/Users/15854/AppData/Local/Programs/Microsoft VS Code/bin:/snap/bin"
				},
			},
			want:    "/mnt/c/Users/15854",
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				envGetter: func(s string) string {
					return ""
				},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BasicStore{
				envGetter: tt.fields.envGetter,
			}
			got, err := b.GetWSLHostHomeDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("BasicStore.GetWindowsDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BasicStore.GetWindowsDir() = %v, want %v", got, tt.want)
			}
		})
	}
}
