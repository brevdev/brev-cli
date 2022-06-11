package configureenvvars

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_getKeysFromEnvFile(t *testing.T) {
	type args struct {
		content string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty file gives empty string",
			args: args{
				content: ``,
			},
			want: []string{},
		},
		{
			name: "export prefixed file works ",
			args: args{
				content: `export foo=bar`,
			},
			want: []string{"foo"},
		},
		{
			name: "multi line file works",
			args: args{
				content: `export foo=bar
export alice=bob`,
			},
			want: []string{"foo", "alice"},
		},
		{
			name: "unexported strings works",
			args: args{
				content: `foo=bar`,
			},
			want: []string{"foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getKeysFromEnvFile(tt.args.content); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getKeysFromEnvFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateExportString(t *testing.T) {
	type args struct {
		brevEnvsString  string
		envFileContents string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "base case",
			args: args{
				brevEnvsString:  "",
				envFileContents: "",
			},
			want: "",
		},
		// TODO: Add test cases.
		{
			name: "deletes env vars not in envfile",
			args: args{
				brevEnvsString:  "foo,bar,baz",
				envFileContents: "",
			},
			want: `unset foo
unset bar
unset baz`,
		},
		{
			name: "sets env var",
			args: args{
				brevEnvsString:  "",
				envFileContents: "foo=bar",
			},
			want: `export foo=bar
export ` + BREV_MANGED_ENV_VARS_KEY + `=foo`,
		},
		{
			name: "sets env var with export prefix",
			args: args{
				brevEnvsString:  "",
				envFileContents: "export foo=bar",
			},
			want: `export foo=bar
export ` + BREV_MANGED_ENV_VARS_KEY + `=foo`,
		},
		{
			name: "is idempotent",
			args: args{
				brevEnvsString:  "foo",
				envFileContents: "foo=bar",
			},
			want: `export foo=bar
export ` + BREV_MANGED_ENV_VARS_KEY + "=foo",
		},
		{
			name: "multiple operations(journal) case",
			args: args{
				brevEnvsString:  "key1,key2,key3",
				envFileContents: "export key4=val",
			},
			want: `unset key1
unset key2
unset key3
export key4=val
export ` + BREV_MANGED_ENV_VARS_KEY + "=key4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateExportString(tt.args.brevEnvsString, tt.args.envFileContents)
			diff := cmp.Diff(tt.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func Test_addUnsetEntriesToOutput(t *testing.T) {
	type args struct {
		currentEnvs []string
		newEnvs     []string
		output      []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "base case",
			args: args{
				currentEnvs: []string{},
				newEnvs:     []string{},
				output:      []string{},
			},
			want: []string{},
		},
		{
			name: "base case with empty strings",
			args: args{
				currentEnvs: []string{""},
				newEnvs:     []string{""},
				output:      []string{},
			},
			want: []string{},
		},
		{
			name: "preserves output",
			args: args{
				currentEnvs: []string{""},
				newEnvs:     []string{""},
				output:      []string{""},
			},
			want: []string{""},
		},
		{
			name: "when a current env is not in the list of new envs, unset it",
			args: args{
				currentEnvs: []string{"foo"},
				newEnvs:     []string{},
				output:      []string{},
			},
			want: []string{"unset foo"},
		},
		{
			name: "when a current env is new envs, don't unset it",
			args: args{
				currentEnvs: []string{},
				newEnvs:     []string{"foo"},
				output:      []string{},
			},
			want: []string{},
		},
		{
			name: "when a current env is enpty entry, don't unset it",
			args: args{
				currentEnvs: []string{""},
				newEnvs:     []string{"foo"},
				output:      []string{},
			},
			want: []string{},
		},
		{
			name: "when a current env is new envs and current envs, don't unset it",
			args: args{
				currentEnvs: []string{"foo"},
				newEnvs:     []string{"foo"},
				output:      []string{},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addUnsetEntriesToOutput(tt.args.currentEnvs, tt.args.newEnvs, tt.args.output)
			diff := cmp.Diff(tt.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
