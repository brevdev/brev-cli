package client

import (
	"context"
	stderrs "errors"
	"testing"

	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/stretchr/testify/require"
)

func TestNewUsesCustomRPCClient(t *testing.T) {
	custom := &stubRPC{}
	cli, err := New(config.Config{BrevCloudAgentURL: "http://example.dev"}, WithRPCClient(custom))
	require.NoError(t, err)
	require.Equal(t, custom, cli)
}

func TestClassifyErrorMapsUnauthenticated(t *testing.T) {
	raw := connect.NewError(connect.CodeUnauthenticated, stderrs.New("unauth"))
	err := ClassifyError(raw)
	require.Error(t, err)
	require.True(t, stderrs.Is(err, ErrUnauthenticated))
}

func TestClassifyErrorMapsRegistrationDetail(t *testing.T) {
	raw := connect.NewError(connect.CodeInvalidArgument, stderrs.New("bad token"))
	detail, detailErr := connect.NewErrorDetail(&brevapiv2.BrevCloudRegistrationErrorDetail{
		Reason:  brevapiv2.BrevCloudRegistrationErrorReason_BREV_CLOUD_REGISTRATION_ERROR_REASON_INVALID_TOKEN,
		Message: "token invalid",
	})
	require.NoError(t, detailErr)
	raw.AddDetail(detail)

	err := ClassifyError(raw)
	require.Error(t, err)

	var regErr *RegistrationError
	require.ErrorAs(t, err, &regErr)
	require.Equal(t, brevapiv2.BrevCloudRegistrationErrorReason_BREV_CLOUD_REGISTRATION_ERROR_REASON_INVALID_TOKEN, regErr.Reason)
	require.Equal(t, "token invalid", regErr.Error())
}

func TestBearerToken(t *testing.T) {
	require.Equal(t, "Bearer abc", BearerToken("abc"))
}

type stubRPC struct{}

func (s *stubRPC) Register(context.Context, *connect.Request[brevapiv2.RegisterRequest]) (*connect.Response[brevapiv2.RegisterResponse], error) {
	return connect.NewResponse(&brevapiv2.RegisterResponse{}), nil
}

func (s *stubRPC) Heartbeat(context.Context, *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
	return connect.NewResponse(&brevapiv2.HeartbeatResponse{}), nil
}

func (s *stubRPC) GetTunnelToken(context.Context, *connect.Request[brevapiv2.GetTunnelTokenRequest]) (*connect.Response[brevapiv2.GetTunnelTokenResponse], error) {
	return connect.NewResponse(&brevapiv2.GetTunnelTokenResponse{}), nil
}
