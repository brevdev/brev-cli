package writeconnectionevent

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/afero"
)

func TestRunWriteConnectionEvent(t *testing.T) {
	fs := afero.NewMemMapFs()
	type args struct {
		in0   *terminal.Terminal
		in1   []string
		store writeConnectionEventStore
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "write connection event",
			args: args{
				nil,
				[]string{},
				store.NewBasicStore().WithFileSystem(fs),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RunWriteConnectionEvent(tt.args.in0, tt.args.in1, tt.args.store); (err != nil) != tt.wantErr {
				t.Errorf("RunWriteConnectionEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
