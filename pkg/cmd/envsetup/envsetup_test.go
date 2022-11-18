package envsetup

import (
	_ "embed"
	"os"
	"os/user"
	"testing"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"
	"github.com/tweekmonster/luser"
)

func Test_appendLogToFile(t *testing.T) {
	t.Skip()
	err := appendLogToFile("test", "test")
	if err != nil {
		t.Errorf("error appending to file %s", err)
	}
}

func Test_MOTDExists(t *testing.T) {
	if motd == "" {
		t.Errorf("motd is empty")
	}
}

func Test_SpeedtestExists(t *testing.T) {
	if speedtest == "" {
		t.Errorf("speedtest is empty")
	}
}

func makeMockFS() setupKeyI {
	bs := store.NewBasicStore().WithEnvGetter(
		func(s string) string {
			return "test"
		},
	)
	fs := bs.WithFileSystem(afero.NewMemMapFs())

	fs = fs.WithUserHomeDirGetter(
		func() (string, error) {
			return "/home/test", nil
		},
	)
	fs.User = &luser.User{
		User: &user.User{
			Uid: "1000",
			Gid: "1000",
		},
	}
	return fs
}

func Test_setupKey(t *testing.T) {
	type args struct {
		path    string
		content string
		perm    os.FileMode
		store   setupKeyI
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				path:    "test",
				content: "test",
				perm:    0o644,
				store:   makeMockFS(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := setupKey(tt.args.path, tt.args.content, tt.args.perm, tt.args.store); (err != nil) != tt.wantErr {
				t.Errorf("setupKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
