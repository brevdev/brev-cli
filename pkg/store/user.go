package store

import (
	"context"
	"fmt"

	"buf.build/gen/go/brevdev/devplane/grpc/go/devplaneapi/v1/devplaneapiv1grpc"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var mePath = "api/me"

func (s AuthHTTPStore) GetCurrentUser() (*entity.User, error) {
	// Get the access token
	accessToken, err := s.authHTTPClient.auth.GetAccessToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Create gRPC connection
	grpcURL := config.GlobalConfig.GetBrevGRPCURL()

	// Configure TLS for HTTPS ingress (port 443)
	creds := credentials.NewTLS(nil)

	// Create connection with TLS and proper authority header
	conn, err := grpc.NewClient(
		grpcURL,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	defer conn.Close()

	// Create user service client
	client := devplaneapiv1grpc.NewUserServiceClient(conn)

	// Call GetUserByToken
	ctx := context.Background()
	resp, err := client.GetUserByToken(ctx, &devplaneapiv1.GetUserByTokenRequest{
		Token: accessToken,
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Convert protobuf User to entity.User
	if resp.User == nil {
		return nil, breverrors.WrapAndTrace(fmt.Errorf("user not found in response"))
	}

	result := convertProtoUserToEntityUser(resp.User)

	// Check if user has multiple identities and is using Auth0
	if len(result.ExternalIdentities) > 1 {
		// Check if the current token is from Auth0
		isAuth0Token := auth.IssuerCheck(accessToken, "https://brevdev.us.auth0.com/")
		if isAuth0Token {
			// User has multiple identities and is using Auth0, suggest NVIDIA login
			return nil, breverrors.NewNvidiaMigrationError("This account has an NVIDIA login available")
		}
	}

	breverrors.GetDefaultErrorReporter().SetUser(breverrors.ErrorUser{
		ID:       result.ID,
		Username: result.Username,
		Email:    result.Email,
	})

	return result, nil
}

// convertProtoUserToEntityUser converts a protobuf User to entity.User
func convertProtoUserToEntityUser(protoUser *devplaneapiv1.User) *entity.User {
	user := &entity.User{
		ID:       protoUser.UserId,
		Username: protoUser.Username,
		Name:     protoUser.DisplayName,
		Email:    protoUser.DefaultEmail,
	}

	// Convert external identities
	if len(protoUser.ExternalIdentities) > 0 {
		user.ExternalIdentities = make([]*entity.ExternalIdentity, len(protoUser.ExternalIdentities))
		for i, extID := range protoUser.ExternalIdentities {
			user.ExternalIdentities[i] = &entity.ExternalIdentity{
				IdentityID: extID.IdentityId,
				Provider:   extID.Provider,
				ExternalID: extID.ExternalId,
			}
		}
	}

	// Convert metadata to onboarding data
	if protoUser.Metadata != nil {
		user.OnboardingData = protoUser.Metadata.AsMap()
	}

	return user
}

func (s AuthHTTPStore) GetCurrentUserID() (string, error) {
	meta, err := s.GetCurrentWorkspaceMeta()
	if err != nil {
		return "", nil
	}
	if meta.UserID != "" {
		return meta.UserID, nil
	}
	user, err := s.GetCurrentUser()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return user.ID, nil
}

var userKeysPath = fmt.Sprintf("%s/keys", mePath)

func (s AuthHTTPStore) GetCurrentUserKeys() (*entity.UserKeys, error) {
	var result entity.UserKeys
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(userKeysPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

var usersPath = "api/users"

type UserCreateResponse struct {
	User         entity.User `json:"user"`
	ErrorMessage string      `json:"errorMessage"`
}

func (n NoAuthHTTPStore) CreateUser(identityToken string) (*entity.User, error) {
	var result UserCreateResponse
	res, err := n.noAuthHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Identity", identityToken).
		SetResult(&result).
		Post(usersPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result.User, nil
}

func (s AuthHTTPStore) UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error) {
	var result entity.User
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetBody(updatedUser).
		// SetPathParam(userIDParamName, userID).
		Put(usersPath + "/" + userID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

// 	userIDParamName = "userID"
// 	userIDParamStr  = fmt.Sprintf("{%s}", userIDParamName)

var usersIDPathPattern = fmt.Sprintf("%s/%s", usersPath, "%s")

// usersIDPath        = fmt.Sprintf(usersIDPathPattern, fmt.Sprintf("{%s}", userIDParamStr))

func (s AuthHTTPStore) GetUsers(queryParams map[string]string) ([]entity.User, error) {
	var result []entity.User
	res, err := s.authHTTPClient.restyClient.R().
		SetQueryParams(queryParams).
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(usersPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return result, nil
}
