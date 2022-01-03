package store

import (
	"testing"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

const (
	WORKING_RSA_PRIVATE_KEY = `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`
	CORRUPT_RSA_PRIVATE_KEY = `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`
)

func setupSSHConfigFile(fs afero.Fs) error {
	sshConfigPath, err := files.GetUserSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = fs.Create(*sshConfigPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func TestGetSSHConfig(t *testing.T) {
	fs := MakeMockFileStore()
	err := setupSSHConfigFile(fs.fs)
	if !assert.Nil(t, err) {
		return
	}

	_, err = fs.GetSSHConfig()
	if !assert.Nil(t, err) {
		return
	}
}

func TestWriteSSHConfig(t *testing.T) {
	fs := MakeMockFileStore()
	err := fs.WriteSSHConfig(``)
	if !assert.Nil(t, err) {
		return
	}
}

func TestCreateNewSSHConfigBackup(t *testing.T) {
	fs := MakeMockFileStore()
	err := setupSSHConfigFile(fs.fs)
	if !assert.Nil(t, err) {
		return
	}

	err = fs.CreateNewSSHConfigBackup()
	if !assert.Nil(t, err) {
		return
	}
}

func TestWritePrivateKey(t *testing.T) {
	fs := MakeMockFileStore()
	err := fs.WritePrivateKey(WORKING_RSA_PRIVATE_KEY)
	if !assert.Nil(t, err) {
		return
	}
	privateKeyFilePath := fs.GetPrivateKeyFilePath()
	pk, err := fs.GetOrCreateFile(privateKeyFilePath)
	if !assert.Nil(t, err) {
		return
	}
	data, err := afero.ReadAll(pk)
	assert.Nil(t, VerifyPrivateKey(data))
}

func TestVerifyPrivateKey(t *testing.T) {
	type args struct {
		key []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Valid Private Key Parses",
			args: args{

				key: []byte(WORKING_RSA_PRIVATE_KEY),
			},
			wantErr: false,
		},
		{
			name: "looks valid but actually not",
			args: args{

				key: []byte(CORRUPT_RSA_PRIVATE_KEY),
			},
			wantErr: true,
		},
		{
			name: "empty key fails",
			args: args{

				key: []byte(``),
			},
			wantErr: true,
		},
		{
			name: "incorrectly formatted key fails",
			args: args{

				key: []byte(`slfkjafalkjfas`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifyPrivateKey(tt.args.key); (err != nil) != tt.wantErr {
				t.Errorf("VerifyPrivateKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
