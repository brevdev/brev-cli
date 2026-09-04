package store

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type fakeManagedSecretService struct {
	nodev1connect.UnimplementedManagedSecretServiceHandler

	createSecretFn       func(*connect.Request[nodev1.ManagedSecretServiceCreateSecretRequest]) (*nodev1.ManagedSecretServiceCreateSecretResponse, error)
	listSecretsFn        func(*connect.Request[nodev1.ManagedSecretServiceListSecretsRequest]) (*nodev1.ManagedSecretServiceListSecretsResponse, error)
	getSecretFn          func(*connect.Request[nodev1.ManagedSecretServiceGetSecretRequest]) (*nodev1.ManagedSecretServiceGetSecretResponse, error)
	listSecretVersionsFn func(*connect.Request[nodev1.ManagedSecretServiceListSecretVersionsRequest]) (*nodev1.ManagedSecretServiceListSecretVersionsResponse, error)
	getSecretValueFn     func(*connect.Request[nodev1.ManagedSecretServiceGetSecretValueRequest]) (*nodev1.ManagedSecretServiceGetSecretValueResponse, error)
	setSecretValueFn     func(*connect.Request[nodev1.ManagedSecretServiceSetSecretValueRequest]) (*nodev1.ManagedSecretServiceSetSecretValueResponse, error)
	deleteSecretFn       func(*connect.Request[nodev1.ManagedSecretServiceDeleteSecretRequest]) (*nodev1.ManagedSecretServiceDeleteSecretResponse, error)
}

func (f *fakeManagedSecretService) CreateSecret(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceCreateSecretRequest]) (*connect.Response[nodev1.ManagedSecretServiceCreateSecretResponse], error) {
	res, err := f.createSecretFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (f *fakeManagedSecretService) ListSecrets(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceListSecretsRequest]) (*connect.Response[nodev1.ManagedSecretServiceListSecretsResponse], error) {
	res, err := f.listSecretsFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (f *fakeManagedSecretService) GetSecret(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceGetSecretRequest]) (*connect.Response[nodev1.ManagedSecretServiceGetSecretResponse], error) {
	res, err := f.getSecretFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (f *fakeManagedSecretService) ListSecretVersions(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceListSecretVersionsRequest]) (*connect.Response[nodev1.ManagedSecretServiceListSecretVersionsResponse], error) {
	res, err := f.listSecretVersionsFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (f *fakeManagedSecretService) GetSecretValue(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceGetSecretValueRequest]) (*connect.Response[nodev1.ManagedSecretServiceGetSecretValueResponse], error) {
	res, err := f.getSecretValueFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (f *fakeManagedSecretService) SetSecretValue(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceSetSecretValueRequest]) (*connect.Response[nodev1.ManagedSecretServiceSetSecretValueResponse], error) {
	res, err := f.setSecretValueFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (f *fakeManagedSecretService) DeleteSecret(_ context.Context, req *connect.Request[nodev1.ManagedSecretServiceDeleteSecretRequest]) (*connect.Response[nodev1.ManagedSecretServiceDeleteSecretResponse], error) {
	res, err := f.deleteSecretFn(req)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// newManagedSecretTestStore wires an AuthHTTPStore to an in-process connect
// server backed by svc, mirroring TestListOrganizationMembersUsesDevPlaneRPC.
func newManagedSecretTestStore(t *testing.T, svc *fakeManagedSecretService) *AuthHTTPStore {
	t.Helper()
	_, handler := nodev1connect.NewManagedSecretServiceHandler(svc)
	rpcServer := httptest.NewServer(handler)
	t.Cleanup(rpcServer.Close)
	t.Setenv("BREV_PUBLIC_API_URL", rpcServer.URL)

	legacyRESTServer := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(legacyRESTServer.Close)

	token := "tok"
	fileStore, _, _ := newAuthTokenTestStore(t)
	return fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &token}, legacyRESTServer.URL))
}

var testSecretTime = timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

func testManagedSecret(id, name string) *nodev1.ManagedSecret {
	return &nodev1.ManagedSecret{SecretId: id, Name: name, CreateTime: testSecretTime}
}

func testManagedSecretVersion(id string, number int64) *nodev1.ManagedSecretVersion {
	return &nodev1.ManagedSecretVersion{VersionId: id, VersionNumber: number, CreateTime: testSecretTime}
}

func TestConnectErrToBrevErr(t *testing.T) {
	t.Run("not found becomes validation error", func(t *testing.T) {
		err := connectErrToBrevErr(connect.NewError(connect.CodeNotFound, errors.New("managed secret not found")))

		var ve breverrors.ValidationError
		require.ErrorAs(t, err, &ve)
		assert.Equal(t, "managed secret not found", ve.Message)
	})

	t.Run("other connect codes pass through", func(t *testing.T) {
		connectErr := connect.NewError(connect.CodeInternal, errors.New("boom"))

		assert.Same(t, connectErr, connectErrToBrevErr(connectErr))
	})

	t.Run("non connect errors pass through", func(t *testing.T) {
		plainErr := errors.New("plain")

		assert.Same(t, plainErr, connectErrToBrevErr(plainErr))
	})
}

func TestCreateManagedSecret_SendsFieldsAndReturnsSecret(t *testing.T) {
	var gotAuth, gotOrgID, gotName, gotValue string
	svc := &fakeManagedSecretService{
		createSecretFn: func(req *connect.Request[nodev1.ManagedSecretServiceCreateSecretRequest]) (*nodev1.ManagedSecretServiceCreateSecretResponse, error) {
			gotAuth = req.Header().Get("Authorization")
			gotOrgID = req.Msg.GetOrganizationId()
			gotName = req.Msg.GetName()
			gotValue = req.Msg.GetValue()
			return &nodev1.ManagedSecretServiceCreateSecretResponse{Secret: testManagedSecret("msec-1", "my-secret")}, nil
		},
	}
	s := newManagedSecretTestStore(t, svc)

	secret, err := s.CreateManagedSecret("org-1", "my-secret", "s3cr3t")

	require.NoError(t, err)
	assert.Equal(t, "Bearer tok", gotAuth)
	assert.Equal(t, "org-1", gotOrgID)
	assert.Equal(t, "my-secret", gotName)
	assert.Equal(t, "s3cr3t", gotValue)
	require.NotNil(t, secret)
	assert.Equal(t, "msec-1", secret.GetSecretId())
}

func TestListManagedSecrets_PaginatesThroughPages(t *testing.T) {
	var gotOrgIDs []string
	var gotPageTokens []string
	call := 0
	svc := &fakeManagedSecretService{
		listSecretsFn: func(req *connect.Request[nodev1.ManagedSecretServiceListSecretsRequest]) (*nodev1.ManagedSecretServiceListSecretsResponse, error) {
			call++
			gotOrgIDs = append(gotOrgIDs, req.Msg.GetOrganizationId())
			gotPageTokens = append(gotPageTokens, req.Msg.GetPageParams().GetPageToken())
			if call == 1 {
				return &nodev1.ManagedSecretServiceListSecretsResponse{
					Items:         []*nodev1.ManagedSecret{testManagedSecret("msec-1", "one")},
					NextPageToken: "page-2",
				}, nil
			}
			return &nodev1.ManagedSecretServiceListSecretsResponse{
				Items: []*nodev1.ManagedSecret{testManagedSecret("msec-2", "two")},
			}, nil
		},
	}
	s := newManagedSecretTestStore(t, svc)

	secrets, err := s.ListManagedSecrets("org-1")

	require.NoError(t, err)
	require.Len(t, secrets, 2)
	assert.Equal(t, "msec-1", secrets[0].GetSecretId())
	assert.Equal(t, "msec-2", secrets[1].GetSecretId())
	assert.Equal(t, []string{"org-1", "org-1"}, gotOrgIDs)
	assert.Equal(t, []string{"", "page-2"}, gotPageTokens)
}

func TestGetManagedSecret_NotFoundReturnsValidationError(t *testing.T) {
	svc := &fakeManagedSecretService{
		getSecretFn: func(_ *connect.Request[nodev1.ManagedSecretServiceGetSecretRequest]) (*nodev1.ManagedSecretServiceGetSecretResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("managed secret not found"))
		},
	}
	s := newManagedSecretTestStore(t, svc)

	_, err := s.GetManagedSecret("msec-missing")

	var ve breverrors.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, `secret "msec-missing" not found`, ve.Message)
}

func TestGetManagedSecretValue_SendsIDsAndReturnsValue(t *testing.T) {
	var gotSecretID, gotVersionID string
	svc := &fakeManagedSecretService{
		getSecretValueFn: func(req *connect.Request[nodev1.ManagedSecretServiceGetSecretValueRequest]) (*nodev1.ManagedSecretServiceGetSecretValueResponse, error) {
			gotSecretID = req.Msg.GetSecretId()
			gotVersionID = req.Msg.GetVersionId()
			return &nodev1.ManagedSecretServiceGetSecretValueResponse{Value: "s3cr3t"}, nil
		},
	}
	s := newManagedSecretTestStore(t, svc)

	value, err := s.GetManagedSecretValue("msec-1", "msecv-2")

	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", value)
	assert.Equal(t, "msec-1", gotSecretID)
	assert.Equal(t, "msecv-2", gotVersionID)
}

func TestGetManagedSecretValue_NotFoundReturnsValidationError(t *testing.T) {
	svc := &fakeManagedSecretService{
		getSecretValueFn: func(_ *connect.Request[nodev1.ManagedSecretServiceGetSecretValueRequest]) (*nodev1.ManagedSecretServiceGetSecretValueResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("managed secret not found"))
		},
	}
	s := newManagedSecretTestStore(t, svc)

	_, err := s.GetManagedSecretValue("msec-missing", "msecv-missing")

	var ve breverrors.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "managed secret not found", ve.Message)
}

func TestSetManagedSecretValue_CreatesNewVersion(t *testing.T) {
	var gotSecretID, gotValue string
	svc := &fakeManagedSecretService{
		setSecretValueFn: func(req *connect.Request[nodev1.ManagedSecretServiceSetSecretValueRequest]) (*nodev1.ManagedSecretServiceSetSecretValueResponse, error) {
			gotSecretID = req.Msg.GetSecretId()
			gotValue = req.Msg.GetValue()
			return &nodev1.ManagedSecretServiceSetSecretValueResponse{Version: testManagedSecretVersion("msecv-2", 2)}, nil
		},
	}
	s := newManagedSecretTestStore(t, svc)

	version, err := s.SetManagedSecretValue("msec-1", "rotated")

	require.NoError(t, err)
	assert.Equal(t, "msec-1", gotSecretID)
	assert.Equal(t, "rotated", gotValue)
	require.NotNil(t, version)
	assert.Equal(t, int64(2), version.GetVersionNumber())
}

func TestDeleteManagedSecret_SendsSecretID(t *testing.T) {
	var gotSecretID string
	svc := &fakeManagedSecretService{
		deleteSecretFn: func(req *connect.Request[nodev1.ManagedSecretServiceDeleteSecretRequest]) (*nodev1.ManagedSecretServiceDeleteSecretResponse, error) {
			gotSecretID = req.Msg.GetSecretId()
			return &nodev1.ManagedSecretServiceDeleteSecretResponse{}, nil
		},
	}
	s := newManagedSecretTestStore(t, svc)

	err := s.DeleteManagedSecret("msec-1")

	require.NoError(t, err)
	assert.Equal(t, "msec-1", gotSecretID)
}
