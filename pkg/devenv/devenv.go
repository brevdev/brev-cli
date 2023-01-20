package devenv

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"

	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func makeClient() (*client.Client, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return client, nil
}

func makeAuthStr(token string) (string, error) {
	authConfig := types.AuthConfig{
		Username: "AWS",
		Password: token,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", errors.WrapAndTrace(err)
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	return authStr, nil
}

// get a token via
// aws ecr get-login-password --region us-west-2
func pullImage(ctx context.Context, refStr string, token string) error {
	c, err := makeClient()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	authStr, err := makeAuthStr(token)
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	reader, err := c.ImagePull(ctx, refStr, types.ImagePullOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	return nil
}

func pushImage(ctx context.Context, refStr string, token string) error {
	c, err := makeClient()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	authStr, err := makeAuthStr(token)
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	reader, err := c.ImagePush(ctx, refStr, types.ImagePushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}
