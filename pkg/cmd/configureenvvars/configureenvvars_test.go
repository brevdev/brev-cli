package configureenvvars

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
		{
			name: "using env format found on workspace",
			args: args{
				brevEnvsString:  "",
				envFileContents: `export foo='bar';export alice='bob'`,
			},
			want: `export alice='bob'
export foo='bar'
export ` + BREV_MANGED_ENV_VARS_KEY + "=alice,foo",
		},
		{
			name: "multi line file",
			args: args{
				brevEnvsString: "",
				envFileContents: `export foo='bar';
export alice='bob'`,
			},
			want: `export alice='bob'
export foo='bar'
export ` + BREV_MANGED_ENV_VARS_KEY + "=alice,foo",
		},
		{
			name: "semicolon -> newline  file ",
			args: args{
				brevEnvsString: "",
				envFileContents: `export foo='bar';

export alice='bob'`,
			},
			want: `export alice='bob'
export foo='bar'
export ` + BREV_MANGED_ENV_VARS_KEY + "=alice,foo",
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

func Test_parse(t *testing.T) {
	type args struct {
		content string
	}
	tests := []struct {
		name string
		args args
		want envVars
	}{
		// TODO: Add test cases.
		{
			name: "base case",
			args: args{
				content: "",
			},
			want: envVars{},
		},
		{
			name: "parses envs",
			args: args{
				content: "foo=bar",
			},
			want: envVars{"foo": "bar"},
		},
		{
			name: "parses envs other format",
			args: args{
				content: "export foo='bar';export alice='bob'",
			},
			want: envVars{"foo": "'bar'", "alice": "'bob'"},
		},
		{
			name: "export prefixed file works ",
			args: args{
				content: `export foo=bar`,
			},
			want: envVars{"foo": "bar"},
		},
		{
			name: "multi line file works",
			args: args{
				content: `export foo=bar
export alice=bob`,
			},
			want: envVars{"foo": "bar", "alice": "bob"},
		},
		{
			name: "multi newline file works",
			args: args{
				content: `export foo=bar

export alice=bob`,
			},
			want: envVars{"foo": "bar", "alice": "bob"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parse(tt.args.content); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
