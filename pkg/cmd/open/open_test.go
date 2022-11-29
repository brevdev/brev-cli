package open

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/mo"
)

type mockVscodePathStore struct{}

func (m *mockVscodePathStore) GetWindowsDir() (string, error) {
	return "/mnt/c/Users/1234", nil
}

type mockVscodePathStoreAlwaysError struct{}

func (m *mockVscodePathStoreAlwaysError) GetWindowsDir() (string, error) {
	return "", errors.New("error")
}

func Test_getCommonVsCodePaths(t *testing.T) {
	type args struct {
		store vscodePathStore
	}
	tests := []struct {
		name string
		args args
		want []mo.Result[string]
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				store: &mockVscodePathStore{},
			},
			want: append(
				commonVSCodePaths,
				[]mo.Result[string]{
					mo.Ok("/mnt/c/Users/1234/AppData/Local/Programs/Microsoft VS Code/Code.exe"),
					mo.Ok("/mnt/c/Users/1234/AppData/Local/Programs/Microsoft VS Code/bin/code"),
				}...),
		},
		{
			name: "test",
			args: args{
				store: &mockVscodePathStoreAlwaysError{},
			},
			want: append(
				commonVSCodePaths,
				[]mo.Result[string]{
					mo.Err[string](errors.New("error")),
					mo.Err[string](errors.New("error")),
				}...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCommonVsCodePaths(tt.args.store)
			diff := cmp.Diff(tt.want, got,
				cmp.AllowUnexported(mo.Result[string]{}),
				cmp.Comparer(func(x, y mo.Result[string]) bool {
					if x.IsOk() && y.IsOk() {
						return x.MustGet() == y.MustGet()
					}
					if x.IsError() && y.IsError() {
						return true
					}
					return false
				}),
			)
			if diff != "" {
				t.Errorf("getCommonVsCodePaths() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
