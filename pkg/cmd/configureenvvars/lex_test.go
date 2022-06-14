package configureenvvars

import (
	"reflect"
	"testing"
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
		}, {
			name: "key=val works",
			args: args{
				input: "key=val",
			},
			want: []item{{
				typ: itemKey,
				val: "key",
			},{
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
			if !reflect.DeepEqual(out, tt.want) {
				t.Errorf("lex() = %v, want %v", got, tt.want)
			}
		})
	}
}
