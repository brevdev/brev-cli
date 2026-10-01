package auth

import (
	"testing"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Closing the browser leaves the poll to time out. Cause() is what
// cmderrors.DisplayAndHandleError prints, so it is the whole message.
func TestPollForTokens_TimesOutWithAClearMessage(t *testing.T) {
	a := KasAuthenticator{PollTimeout: 10 * time.Millisecond}

	_, err := a.pollForTokens("session-key", "device-id")

	require.Error(t, err)
	assert.Equal(t, "timed out waiting for login", pkgerrors.Cause(err).Error())
}
