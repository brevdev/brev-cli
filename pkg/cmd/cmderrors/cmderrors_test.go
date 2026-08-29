package cmderrors

import (
	"bytes"
	"io"
	"os"
	"testing"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStderr runs fn while collecting everything written to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	// Close the writer before reading buf so the io.Copy goroutine sees
	// EOF and returns. Without this, the deferred close ran after the
	// return statement and buf was always empty.
	_ = w.Close()
	<-done
	_ = r.Close()
	os.Stderr = orig

	return buf.String()
}

// A NetworkError wrapped through WrapAndTrace should render as a short,
// friendly message — no stack trace, no "github.com/brevdev/..." lines.
func TestDisplayAndHandleError_NetworkErrorIsFriendly(t *testing.T) {
	netErr := &breverrors.NetworkError{Host: "api.ngc.nvidia.com"}
	wrapped := breverrors.WrapAndTrace(breverrors.WrapAndTrace(netErr))

	out := captureStderr(t, func() {
		DisplayAndHandleError(wrapped)
	})

	assert.Contains(t, out, "api.ngc.nvidia.com")
	assert.Contains(t, out, "internet connection")
	// The hallmark of the old convoluted output: stack trace lines pointing
	// at github.com/brevdev/brev-cli source paths. The friendly message
	// must not include them.
	assert.NotContains(t, out, "github.com/brevdev/brev-cli/pkg/auth")
	assert.NotContains(t, out, "[error]")
}

// A non-network error should still render through the default red path.
func TestDisplayAndHandleError_PlainError(t *testing.T) {
	err := breverrors.New("something else broke")
	out := captureStderr(t, func() {
		DisplayAndHandleError(err)
	})
	assert.Contains(t, out, "something else broke")
}
