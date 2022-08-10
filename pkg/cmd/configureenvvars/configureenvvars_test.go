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
		{
			name: "hyphen in env var shouldn't be included since that's not allowed in most shells",
			args: args{
				brevEnvsString:  "",
				envFileContents: `export NADER-TEST='nader-testing' `,
			},
			want: ``,
		},
		{
			name: "if we have an invalid env var, we should not include it",
			args: args{
				brevEnvsString:  "",
				envFileContents: `export f$*;_=nader-testing`,
			},
			want: ``,
		},
		{
			name: "invalid keys should be ignored",
			args: args{
				brevEnvsString:  "",
				envFileContents: `export f$*;=nader-testing`,
			},
			want: ``,
		},
		{
			name: "values are escaped",
			args: args{
				brevEnvsString:  "",
				envFileContents: `export foo='90ie&$>'`,
			},
			want: `export foo='90ie&$>'
export ` + BREV_MANGED_ENV_VARS_KEY + "=foo",
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
		{
			name: "env var with space in val",
			args: args{
				content: `export fo o=bar`,
			},
			want: nil,
		},
		{
			name: "env var with space in key",
			args: args{
				content: `export foo=ba r`,
			},
			want: nil,
		},
		{
			name: "leading spaces works",
			args: args{
				content: `  export foo=bar
 export alice=bob`,
			},
			want: envVars{"foo": "bar", "alice": "bob"},
		},
		{
			name: "trailing spaces works",
			args: args{
				content: `export foo=bar
export alice=bob  `,
			},
			want: envVars{"foo": "bar", "alice": "bob"},
		},
		{
			name: "export as key works",
			args: args{
				content: `export foo=bar
export export=bob`,
			},
			want: envVars{"foo": "bar", "export": "bob"},
		},
		{
			name: "export as key works",
			args: args{
				content: `export=bar`,
			},
			want: envVars{"export": "bar"},
		},
		{
			name: "invalid chars not parsed",
			args: args{
				content: `foo&bar>baz=foo\&bar\>baz`,
			},
			want: nil,
		},
		{
			name: "escaped values are parsed",
			args: args{
				content: `foo='foo&bar>baz'`,
			},
			want: envVars{"foo": "'foo&bar>baz'"},
		},
		{
			name: "unescaped values are parsed",
			args: args{
				content: `foo=foo&bar>baz`,
			},
			want: envVars{"foo": "foo&bar>baz"},
		},
		// todo add these cases in the correct formaat
		// 		{"Empty", "", "", ""},
		// {"Emptyish", " ", "", ""},
		// {"OnlyComment", "# ...", "", ""},
		// {"OnlyCommentish", " # ...", "", ""},
		// {"EmptyValue", "FoO=", "FoO", ""},
		// {"EmptyValueComment", "F=# ...", "F", ""},
		// {"EmptyValueSpace", "F_O= ", "F_O", ""},
		// {"EmptyValueSpaceComment", "F= # ...", "F", ""},
		// {"Simple", "FOO=bar", "FOO", "bar"},
		// {"Export", "export FOO=bar", "FOO", "bar"},
		// {"Spaces", " FOO = bar baz ", "FOO", "bar baz"},
		// {"Tabs", "	FOO	= 	bar 	", "FOO", "bar"},
		// {"ExportSpaces", "export FOO = bar", "FOO", "bar"},
		// {"ExportAsKey", "export = bar", "export", "bar"},
		// {"Nums", "A1B2C3=a1b2c3", "A1B2C3", "a1b2c3"},
		// {"Comments", "FOO=bar # ok", "FOO", "bar"},
		// {"EmptyComments1", "FOO=#bar#", "FOO", ""},
		// {"EmptyComments2", "FOO= # bar ", "FOO", ""},
		// {"DoubleQuotes", `FOO="bar#"`, "FOO", "bar#"},
		// {"DoubleQuoteNewline", `FOO="bar\n"`, "FOO", "bar\n"},
		// {"DoubleQuoteNewlineComment", `FOO="bar\n" # comment`, "FOO", "bar\n"},
		// {"DoubleQuoteSpaces", `FOO = " bar\t" `, "FOO", " bar\t"},
		// {"SingleQuotes", "FOO='bar#'", "FOO", "bar#"},
		// {"SingleQuotesNewline", `FOO='\n' # empty`, "FOO", "\\n"},
		// {"SingleQuotesEmpty", "FOO='' # empty", "FOO", ""},
		// {"NormalSingleMix", "FOO=normal'single ' ", "FOO", "normalsingle "},
		// {"NormalDoubleMix", `FOO= "double\\" normal # "EOL"`, "FOO", "double\\ normal"},
		// {"AllModes", `export FOO =  'single\n' \\normal\t "double\"\n " # comment`, "FOO", "single\\n \\\\normal\\t double\"\n "},
		// {"UnicodeLiteral", "U1=\U0001F525", "U1", "\U0001F525"},
		// {"UnicodeLiteralQuoted", "U2= ' \U0001F525 ' ", "U2", " \U0001F525 "},
		// {"EscapedUnicode1byte", `U3="\u2318"`, "U3", "\U00002318"},
		// {"EscapedUnicode2byte", `U3="\uD83D\uDE01"`, "U3", "\U0001F601"},
		// {"EscapedUnicodeCombined", `U4="\u2318\uD83D\uDE01"`, "U4", "\U00002318\U0001F601"},
		// {"README.mdEscapedUnicode", `FOO="The template value\nmay have included\nsome newlines!\n\ud83d\udd25"`, "FOO", "The template value\nmay have included\nsome newlines!\n🔥"},
		// {"UnderscoreKey", "_=x' ' ", "_", "x "},
		// {"DottedKey", "FOO.BAR=x", "FOO.BAR", "x"},
		// {"FwdSlashedKey", "FOO/BAR=x", "FOO/BAR", "x"},
		// {"README.md", `SOME_KEY = normal unquoted \text 'plus single quoted\' "\"double quoted " # EOL`, "SOME_KEY", `normal unquoted \text plus single quoted\ "double quoted `},
		// {"WindowsNewline", `w="\r\n"`, "w", "\r\n"},
		// 		{"MissingEqual", "foo bar", ErrMissingSeparator, ""},
		// {"EmptyKey", "=bar", ErrEmptyKey, ""},
		// {"EqualOnly", "=", ErrEmptyKey, ""},
		// {"InvalidKey", "1abc=x", nil, "key"},
		// {"InvalidKey2", "@abc=x", nil, "key"},
		// {"InvalidKey3", "a b c=x", nil, "key"},
		// {"InvalidKey4", "a\nb=x", nil, "key"},
		// {"InvalidValue", "FOO=\x00", nil, "value"},
		// {"OpenDoubleQuote", `FOO=" bar`, ErrUnmatchedDouble, ""},
		// {"OpenSingleQuote", `FOO=' bar`, ErrUnmatchedSingle, ""},
		// {"UnmatchedMix", `FOO=ok '"ok"' \"not ok ''`, ErrUnmatchedDouble, ""},
		// {"UnmatchedMix2", `FOO=ok '"ok"' \"not ok '"'`, ErrUnmatchedSingle, ""},
		// {"InvalidEscape", `FOO="\a"`, nil, `"a"`},
		// {"IncompleteEscape", `FOO="\`, ErrIncompleteEscape, ""},
		// {"IncompleteHex", `FOO="\u12"`, ErrIncompleteHex, ""},
		// {"InvalidHex", `FOO="\uabcZ"`, nil, `"Z"`},
		// {"IncompleteSurrogatePair1", `FOO="abc \uD83D"`, ErrIncompleteSur, ""},
		// {"IncompleteSurrogatePair2", `FOO="abc \uD83D \uDE01"`, ErrIncompleteSur, ""},
		// {"IncompleteSurrogatePair3", `FOO="abc \uD83DDE01"`, ErrIncompleteSur, ""},
		// {"IncompleteSurrogatePair4", `FOO="abc \uD83D\uDE0"`, nil, `"\""`},

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parse(tt.args.content); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
