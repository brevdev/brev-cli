package devenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthPull(t *testing.T) {
	ctx := context.Background()
	// get a token via
	// aws ecr get-login-password --region us-west-2
	token := ""
	err := pullImage(ctx, "273910284571.dkr.ecr.us-west-2.amazonaws.com/test", token)
	t.Skip("TODO: fix this test")
	assert.NoError(t, err)
}

func TestAuthPush(t *testing.T) {
	ctx := context.Background()
	// get a token via
	// aws ecr get-login-password --region us-west-2
	token := ""
	err := pushImage(ctx, "273910284571.dkr.ecr.us-west-2.amazonaws.com/test", token)
	assert.NoError(t, err)
}
