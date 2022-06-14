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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lex(tt.name, tt.args.input)
			out := []item{}

			for {
				token := got.nextItem()
				out = append(out, token)
				if token.typ == itemEOF {
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
