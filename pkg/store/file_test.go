package store

import (
	"log"
	"os"
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
			_, err := f.GetOrCreateFile(tt.args.fileToCreate)
			if (err != nil) != tt.wantErr {
				t.Errorf("error creating file %s,  %s", err, tt.args.fileToCreate)
				return
			}
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

func TestFileStore_GetOrCreateFile(t *testing.T) {
	bs := MakeMockBasicStore()
	fs := afero.NewMemMapFs()
	dirname, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	type fields struct {
		BasicStore BasicStore
		fs         afero.Fs
	}
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "top level input",
			fields: fields{
				*bs, fs,
			},
			args: args{
				"foo",
			},
			wantErr: false,
		},
		{
			name: "input with singly nested dir",
			fields: fields{
				*bs, fs,
			},
			args: args{
				dirname + "/foo/baz.txt",
			},
			wantErr: false,
		},
		{
			name: "input with doubly nested dir",
			fields: fields{
				*bs, fs,
			},
			args: args{
				dirname + "/foo/bar/baz.txt",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileStore{
				BasicStore: tt.fields.BasicStore,
				fs:         tt.fields.fs,
			}

			_, err := f.GetOrCreateFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileStore.GetOrCreateFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
