package store

import (
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	user "github.com/tweekmonster/luser"
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
				b:  tt.fields.BasicStore,
				fs: tt.fields.fs,
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
				b:  tt.fields.BasicStore,
				fs: tt.fields.fs,
			}

			_, err := f.GetOrCreateFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileStore.GetOrCreateFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestFileStore_AppendString(t *testing.T) {
	bs := MakeMockBasicStore()
	fs := afero.NewMemMapFs()
	type fields struct {
		BasicStore BasicStore
		fs         afero.Fs
	}
	type args struct {
		path    string
		content string
	}
	tests := []struct {
		name                 string
		fields               fields
		args                 args
		wantErr              bool
		path                 string
		content              string
		expectedFileContents string
	}{
		// TODO: Add test cases.
		{
			name: "append to file",
			fields: fields{
				*bs, fs,
			},
			args: args{
				"foo",
				"bar",
			},
			wantErr:              false,
			path:                 "foo",
			content:              "bar",
			expectedFileContents: "barbar",
		},
		{
			name: "append to file",
			fields: fields{
				*bs, fs,
			},
			args: args{
				"foo",
				"\nbar\n",
			},
			wantErr:              false,
			path:                 "foo",
			content:              "bar",
			expectedFileContents: "bar\nbar\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileStore{
				b:    tt.fields.BasicStore,
				fs:   tt.fields.fs,
				User: nil,
			}
			err := f.WriteString(tt.args.path, tt.args.content)
			if err != nil {
				t.Errorf("error writing string to file %s", err)
			}
			if err := f.AppendString(tt.args.path, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("FileStore.AppendString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileStore_OverWriteString(t *testing.T) {
	bs := MakeMockBasicStore()
	fs := afero.NewMemMapFs()
	type fields struct {
		b                 BasicStore
		fs                afero.Fs
		User              *user.User
		userHomeDirGetter func() (string, error)
	}
	type args struct {
		path    string
		content string
	}
	tests := []struct {
		name                 string
		fields               fields
		args                 args
		wantErr              bool
		expectedFileContents string
	}{
		// TODO: Add test cases.
		{
			name: "overwrite file",
			fields: fields{
				*bs, fs, nil, nil,
			},
			args: args{
				"foo",
				"bar",
			},
			wantErr:              false,
			expectedFileContents: "bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileStore{
				b:                 tt.fields.b,
				fs:                tt.fields.fs,
				User:              tt.fields.User,
				userHomeDirGetter: tt.fields.userHomeDirGetter,
			}
			if err := f.OverWriteString(tt.args.path, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("FileStore.OverWriteString() error = %v, wantErr %v", err, tt.wantErr)
			}
			// call twice to make sure it overwrites
			if err := f.OverWriteString(tt.args.path, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("FileStore.OverWriteString() error = %v, wantErr %v", err, tt.wantErr)
			}

			fileContents, err := f.GetFileAsString(tt.args.path)
			if err != nil {
				t.Errorf("error reading file %s", err)
			}
			difference := cmp.Diff(fileContents, tt.args.content)
			if difference != "" {
				t.Errorf("file contents don't match expected %s", difference)
			}
		})
	}
}

func TestFileStore_WriteConnectionEvent(t *testing.T) {
	bs := MakeMockBasicStore()
	fs := afero.NewMemMapFs()
	type fields struct {
		b                 BasicStore
		fs                afero.Fs
		User              *user.User
		userHomeDirGetter func() (string, error)
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "write connection event",
			fields: fields{
				*bs, fs, nil, nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FileStore{
				b:                 tt.fields.b,
				fs:                tt.fields.fs,
				User:              tt.fields.User,
				userHomeDirGetter: tt.fields.userHomeDirGetter,
			}
			if err := f.WriteConnectionEvent(); (err != nil) != tt.wantErr {
				t.Errorf("FileStore.WriteConnectionEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			fileContents, err := f.GetFileAsString("/etc/meta/connection_event")
			if err != nil {
				t.Errorf("error reading file %s", err)
			}
			if fileContents == "" {
				t.Errorf("file contents are empty")
			}
		})
	}
}
