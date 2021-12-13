package store

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestWithFileSystem(t *testing.T) {
	fs := MakeMockFileStore()
	if !assert.NotNil(t, fs) {
		return
	}
}

func MakeMockFileStore() *FileStore {
	bs := MakeMockBasicStore()
	fs := bs.WithFileSystem(afero.NewMemMapFs())
	return fs
}

func TestFileStore_FileExists(t *testing.T) {
	bs := MakeMockBasicStore()
	fs := afero.NewMemMapFs()
	type fields struct {
		BasicStore BasicStore
		fs         afero.Fs
	}
	type args struct {
		fileToCreate string
		filepath     string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "non existing file doesn't exist",
			fields: fields{
				*bs,
				fs,
			},
			args: args{
				"foo",
				"bar",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "existing file exists",
			fields: fields{
				*bs,
				fs,
			},
			args: args{
				"foo",
				"foo",
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileStore{
				BasicStore: tt.fields.BasicStore,
				fs:         tt.fields.fs,
			}
			f.GetOrCreateFile(tt.args.fileToCreate)
			got, err := f.FileExists(tt.args.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileStore.FileExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FileStore.FileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
