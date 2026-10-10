package exec

import (
	stderrors "errors"
	"os/exec"
	"strconv"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exitErrWithCode returns a real *exec.ExitError carrying the given code.
func exitErrWithCode(t *testing.T, code int) *exec.ExitError {
	t.Helper()
	var exitErr *exec.ExitError
	err := exec.Command("bash", "-c", "exit "+strconv.Itoa(code)).Run()
	require.ErrorAs(t, err, &exitErr)
	return exitErr
}

func TestClassifySSHError(t *testing.T) {
	assert.NoError(t, classifySSHError(nil))

	// A remote command's own exit code means the connection worked.
	for _, code := range []int{1, 2, 7, 127} {
		var remoteErr breverrors.RemoteExitError
		err := classifySSHError(exitErrWithCode(t, code))
		require.True(t, stderrors.As(err, &remoteErr), "code %d should be a RemoteExitError", code)
		assert.Equal(t, code, remoteErr.Code)
	}

	// 255 is ssh's own failure code, so it must stay a connection error.
	var remoteErr breverrors.RemoteExitError
	err := classifySSHError(exitErrWithCode(t, sshConnectionFailedExitCode))
	require.Error(t, err)
	assert.False(t, stderrors.As(err, &remoteErr), "255 must not be treated as a remote exit")
}

func TestExitCodeOf(t *testing.T) {
	assert.Equal(t, 3, exitCodeOf(exitErrWithCode(t, 3)))
	assert.Equal(t, -1, exitCodeOf(stderrors.New("not an exit error")))
	assert.Equal(t, -1, exitCodeOf(nil))
}

// A multi-instance run must not exit with one instance's remote code.
func TestFlattenMultiInstanceErr(t *testing.T) {
	assert.NoError(t, flattenMultiInstanceErr(nil))

	agg := multierror.Append(nil, breverrors.RemoteExitError{Code: 3}, breverrors.RemoteExitError{Code: 7})
	flat := flattenMultiInstanceErr(agg)

	var remoteErr breverrors.RemoteExitError
	require.Error(t, flat)
	assert.False(t, stderrors.As(flat, &remoteErr), "no exit code may leak from a multi-instance run")
	// The per-instance detail is still readable in the message.
	assert.Contains(t, flat.Error(), "status 3")
	assert.Contains(t, flat.Error(), "status 7")
}

type fakeTokenStore struct {
	tokens *entity.AuthTokens
	err    error
}

func (f fakeTokenStore) GetAuthTokens() (*entity.AuthTokens, error) { return f.tokens, f.err }

func TestHasNoSavedCredentials(t *testing.T) {
	cases := map[string]struct {
		store fakeTokenStore
		want  bool
	}{
		"no credentials file": {fakeTokenStore{err: &breverrors.CredentialsFileNotFound{}}, true},
		"wrapped not found":   {fakeTokenStore{err: breverrors.WrapAndTrace(&breverrors.CredentialsFileNotFound{})}, true},
		"nil tokens":          {fakeTokenStore{}, true},
		"empty tokens":        {fakeTokenStore{tokens: &entity.AuthTokens{}}, true},
		"has access token":    {fakeTokenStore{tokens: &entity.AuthTokens{AccessToken: "a"}}, false},
		"has refresh token":   {fakeTokenStore{tokens: &entity.AuthTokens{RefreshToken: "r"}}, false},
		"has api key":         {fakeTokenStore{tokens: &entity.AuthTokens{APIKey: "k"}}, false},
		// An unrelated read error must not be reported as missing credentials.
		"other error": {fakeTokenStore{err: stderrors.New("permission denied")}, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasNoSavedCredentials(tc.store))
		})
	}
}

// RemoteExitError must survive wrapping, since main.go recovers the code with errors.As.
func TestRemoteExitErrorSurvivesWrapping(t *testing.T) {
	cases := map[string]error{
		"unwrapped":    breverrors.RemoteExitError{Code: 4},
		"wrapped":      breverrors.WrapAndTrace(breverrors.RemoteExitError{Code: 4}),
		"doubleWrap":   breverrors.WrapAndTrace(breverrors.WrapAndTrace(breverrors.RemoteExitError{Code: 4})),
		"multierror":   multierror.Append(nil, breverrors.RemoteExitError{Code: 4}),
		"multiWrapped": multierror.Append(nil, breverrors.WrapAndTrace(breverrors.RemoteExitError{Code: 4})),
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			var remoteErr breverrors.RemoteExitError
			require.True(t, stderrors.As(err, &remoteErr), "errors.As must match through %s", name)
			assert.Equal(t, 4, remoteErr.Code)
		})
	}
}
