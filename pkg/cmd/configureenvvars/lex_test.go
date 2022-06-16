package configureenvvars

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_lex(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want []item
	}{
		{
			name: "base case",
			args: args{
				input: "",
			},
			want: []item{{
				typ: itemEOF,
				val: "",
			}},
		},
		{
			name: "key=val works",
			args: args{
				input: "key=val",
			},
			want: []item{
				{
					typ: itemKey,
					val: "key",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "val",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "parses envs other format",
			args: args{
				input: "export foo='bar';export alice='bob'",
			},
			want: []item{
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "'bar'",
				},
				{
					typ: itemSemiColon,
					val: ";",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "alice",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "'bob'",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "export prefixed file works ",
			args: args{
				input: `export foo=bar`,
			},
			want: []item{
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "multi line file works",
			args: args{
				input: `export foo=bar
export alice=bob`,
			},
			want: []item{
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemNewline,
					val: "\n",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "alice",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bob",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "multi newline file works",
			args: args{
				input: `export foo=bar

export alice=bob`,
			},
			want: []item{
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemNewline,
					val: "\n",
				},

				{
					typ: itemNewline,
					val: "\n",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "alice",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bob",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "semi colon -> newline file works",
			args: args{
				input: `export foo=bar;

export alice=bob`,
			},
			want: []item{
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemSemiColon,
					val: ";",
				},
				{
					typ: itemNewline,
					val: "\n",
				},

				{
					typ: itemNewline,
					val: "\n",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "alice",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bob",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "leading newline file works",
			args: args{
				input: `
export foo=bar;

export alice=bob`,
			},
			want: []item{
				{
					typ: itemNewline,
					val: "\n",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemSemiColon,
					val: ";",
				},
				{
					typ: itemNewline,
					val: "\n",
				},

				{
					typ: itemNewline,
					val: "\n",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "alice",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bob",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "trailing space with semi colon at end",
			args: args{
				input: `foo=bar  ;`,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemSemiColon,
					val: ";",
				},
				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "trailing space",
			args: args{
				input: `foo=bar `,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},
				{
					typ: itemSpace,
					val: " ",
				},

				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "leading space",
			args: args{
				input: ` foo=bar`,
			},
			want: []item{
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},

				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "spaces in vals with quotes",
			args: args{
				input: `foo='b ar'`,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "'b ar'",
				},

				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "spaces in vals without quotes",
			args: args{
				input: `foo=b ar`,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "b",
				},
				{
					typ: itemSpace,
					val: " ",
				},
				{
					typ: itemError,
					val: "unexpected eof",
				},
			},
		},
		{
			name: "spaces in vals without quotes, multiline",
			args: args{
				input: `foo=b ar
alice=bob`,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "b",
				},
				{
					typ: itemSpace,
					val: " ",
				},

				{
					typ: itemError,
					val: "unexpected newline",
				},
			},
		},
		{
			name: "spaces in keys",
			args: args{
				input: `fo o=bar`,
			},
			want: []item{
				{
					typ: itemError,
					val: "unexpected space",
				},
			},
		},
		{
			name: "lower case export in env var name with space after doesn't screw things up",
			args: args{
				input: `foexport o=bar`,
			},
			want: []item{
				{
					typ: itemError,
					val: "unexpected space",
				},
			},
		},
		{
			name: "tabs instead of spaces",
			args: args{
				input: `	foo=bar`,
			},
			want: []item{
				{
					typ: itemTab,
					val: "\t",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "bar",
				},

				{
					typ: itemEOF,
					val: "",
				},
			},
		},
		{
			name: "tabs value",
			args: args{
				input: `	foo=ba	r`,
			},
			want: []item{
				{
					typ: itemTab,
					val: "\t",
				},
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemValue,
					val: "ba",
				},
				{
					typ: itemTab,
					val: "\t",
				},
				{
					typ: itemError,
					val: "unexpected eof",
				},
			},
		},
		{
			name: "quoted value newline",
			args: args{
				input: `foo="bar
"`,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemError,
					val: "unexpected newline",
				},
			},
		},
		{
			name: "quoted value eof",
			args: args{
				input: `foo="bar`,
			},
			want: []item{
				{
					typ: itemKey,
					val: "foo",
				},
				{
					typ: itemEquals,
					val: "=",
				},
				{
					typ: itemError,
					val: "unexpected eof",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lex(tt.name, tt.args.input)
			out := []item{}

			for {
				token := got.nextItem()
				out = append(out, token)
				if token.typ == itemEOF || token.typ == itemError {
					break
				}

			}
			diff := cmp.Diff(out, tt.want, cmp.AllowUnexported(item{}))
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
