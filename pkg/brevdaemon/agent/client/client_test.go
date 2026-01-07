package client

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/dev-plane/internal/brevcloud/provider"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRegisterSuccess(t *testing.T) {
	var captured *brevapiv2.RegisterRequest
	svc := &testBrevCloudAgentService{
		registerFn: func(_ context.Context, req *connect.Request[brevapiv2.RegisterRequest]) (*connect.Response[brevapiv2.RegisterResponse], error) {
			captured = req.Msg
			return connect.NewResponse(&brevapiv2.RegisterResponse{
				BrevCloudNodeId:   "fn-123",
				DeviceToken:       "device-token",
				HeartbeatInterval: durationpb.New(45 * time.Second),
				DisplayName:       "stored-name",
				CloudName:         "stored-cloud",
				CloudCredId:       "cc-123",
				DeviceFingerprint: "scoped-device-fp",
			}), nil
		},
	}

	client := newTestClient(t, svc)

	params := RegisterParams{
		RegistrationToken:     "reg-token",
		DisplayName:           "node-one",
		CloudName:             "private-cloud",
		AgentVersion:          "v1.2.3",
		Capabilities:          []string{"docker"},
		HardwareFingerprint:   "shape-fp",
		DeviceFingerprintHash: "device-fp-hash",
		Hardware: &HardwareInfo{
			CPUCount:     16,
			RAMBytes:     64 << 30,
			Architecture: "amd64",
			GPUs: []GPUInfo{
				{Model: "A100", MemoryBytes: 40 << 30, Count: 2},
			},
			Storage: []StorageInfo{
				{Name: "nvme0n1", Capacity: 1 << 40, Type: "nvme"},
			},
		},
	}

	result, err := client.Register(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, "fn-123", result.BrevCloudNodeID)
	require.Equal(t, "device-token", result.DeviceToken)
	require.Equal(t, 45*time.Second, result.HeartbeatInterval)
	require.Equal(t, "stored-name", result.DisplayName)
	require.Equal(t, "stored-cloud", result.CloudName)
	require.Equal(t, "cc-123", result.CloudCredID)
	require.Equal(t, "scoped-device-fp", result.DeviceFingerprint)

	require.Equal(t, "node-one", captured.GetDisplayName())
	require.Equal(t, "private-cloud", captured.GetCloudName())
	require.Equal(t, "reg-token", captured.GetRegistrationToken())
	require.Equal(t, []string{"docker"}, captured.GetCapabilities())
	require.Equal(t, "shape-fp", captured.GetHardwareFingerprint())
	require.Equal(t, "device-fp-hash", captured.GetDeviceFingerprintHash())
	require.Equal(t, int32(16), captured.GetHardware().GetCpuCount())
	require.Equal(t, int64(64<<30), captured.GetHardware().GetRamBytes().GetValue())
	require.Len(t, captured.GetHardware().GetGpus(), 1)
	require.Equal(t, "A100", captured.GetHardware().GetGpus()[0].GetModel())
	require.Equal(t, int64(40<<30), captured.GetHardware().GetGpus()[0].GetMemoryBytes().GetValue())
	require.Equal(t, int32(2), captured.GetHardware().GetGpus()[0].GetCount())
	require.Equal(t, "amd64", captured.GetHardware().GetArchitecture())
	require.Len(t, captured.GetHardware().GetStorage(), 1)
	require.Equal(t, "nvme0n1", captured.GetHardware().GetStorage()[0].GetName())
	require.Equal(t, int64(1<<40), captured.GetHardware().GetStorage()[0].GetCapacity().GetValue())
	require.Equal(t, "nvme", captured.GetHardware().GetStorage()[0].GetType())
	require.Equal(t, "v1.2.3", captured.GetAgent().GetVersion())
}

func TestHeartbeatSendsAuthorizationAndPayload(t *testing.T) {
	var (
		capturedHeader string
		capturedReq    *brevapiv2.HeartbeatRequest
	)
	now := time.Now().UTC().Truncate(time.Second)
	lastTransition := now.Add(-time.Hour)
	temp := float32(65.5)

	svc := &testBrevCloudAgentService{
		heartbeatFn: func(_ context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
			capturedHeader = req.Header().Get("Authorization")
			capturedReq = req.Msg
			return connect.NewResponse(&brevapiv2.HeartbeatResponse{
				ServerTime:            timestamppb.New(now.Add(5 * time.Second)),
				NextHeartbeatInterval: durationpb.New(90 * time.Second),
			}), nil
		},
	}

	client := newTestClient(t, svc)

	params := HeartbeatParams{
		BrevCloudNodeID:       "fn-1",
		DeviceToken:           "device-token",
		ObservedAt:            now,
		AgentVersion:          "v2.0.0",
		DisplayName:           "node-one",
		CloudName:             "edge",
		DeviceFingerprintHash: "device-fp-hash",
		HardwareFingerprint:   "shape-fp",
		Status: &HeartbeatStatus{
			Phase:              NodePhaseActive,
			Detail:             "running",
			LastTransitionTime: &lastTransition,
		},
		Utilization: &UtilizationInfo{
			CPUPercent:       25.5,
			MemoryUsedBytes:  4 << 30,
			MemoryTotalBytes: 16 << 30,
			DiskPercent:      50,
			DiskUsedBytes:    100 << 30,
			DiskTotalBytes:   200 << 30,
			GPUs: []GPUUtilization{
				{
					Index:              0,
					Model:              "A100",
					UtilizationPercent: 80,
					MemoryUsedBytes:    10 << 30,
					MemoryTotalBytes:   40 << 30,
					TemperatureCelsius: &temp,
				},
			},
		},
	}

	result, err := client.Heartbeat(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, "Bearer device-token", capturedHeader)
	require.Equal(t, "fn-1", capturedReq.GetBrevCloudNodeId())
	require.Equal(t, now, capturedReq.GetObservedAt().AsTime())
	require.Equal(t, "v2.0.0", capturedReq.GetAgent().GetVersion())
	require.Equal(t, brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE, capturedReq.GetStatus().GetPhase())
	require.Equal(t, "running", capturedReq.GetStatus().GetDetail())
	require.Equal(t, lastTransition.UTC(), capturedReq.GetStatus().GetLastTransitionTime().AsTime())
	require.Equal(t, float32(25.5), capturedReq.GetUtilization().GetCpuPercent())
	require.Equal(t, int64(4<<30), capturedReq.GetUtilization().GetMemoryUsed().GetValue())
	require.Equal(t, "node-one", capturedReq.GetDisplayName())
	require.Equal(t, "edge", capturedReq.GetCloudName())
	require.Equal(t, "device-fp-hash", capturedReq.GetDeviceFingerprintHash())
	require.Equal(t, "shape-fp", capturedReq.GetHardwareFingerprint())

	require.Equal(t, now.Add(5*time.Second), result.ServerTime)
	require.Equal(t, 90*time.Second, result.NextHeartbeatInterval)
}

func TestGetTunnelTokenSendsPortsAndAuth(t *testing.T) {
	var capturedReq *brevapiv2.GetTunnelTokenRequest
	svc := &testBrevCloudAgentService{
		tunnelFn: func(_ context.Context, req *connect.Request[brevapiv2.GetTunnelTokenRequest]) (*connect.Response[brevapiv2.GetTunnelTokenResponse], error) {
			capturedReq = req.Msg
			require.Equal(t, "Bearer device-token", req.Header().Get("Authorization"))
			return connect.NewResponse(&brevapiv2.GetTunnelTokenResponse{
				Token:    "tunnel-token",
				Endpoint: "example.dev:443",
				Ttl:      durationpb.New(2 * time.Minute),
			}), nil
		},
	}

	client := newTestClient(t, svc)
	result, err := client.GetTunnelToken(context.Background(), TunnelTokenParams{
		BrevCloudNodeID: "fn-1",
		DeviceToken:     "device-token",
		TunnelName:      "default",
		Ports: []brevcloud.TunnelPortMapping{
			{LocalPort: 22},
			{RemotePort: 8080},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "fn-1", capturedReq.GetBrevCloudNodeId())
	require.Equal(t, "default", capturedReq.GetTunnelName())
	require.Len(t, capturedReq.GetRequestedPorts(), 2)
	require.Equal(t, int32(22), capturedReq.GetRequestedPorts()[0].GetLocalPort())
	require.Equal(t, int32(22), capturedReq.GetRequestedPorts()[0].GetRemotePort())
	require.Equal(t, int32(8080), capturedReq.GetRequestedPorts()[1].GetRemotePort())

	require.Equal(t, "tunnel-token", result.Token)
	require.Equal(t, "example.dev:443", result.Endpoint)
	require.Equal(t, 2*time.Minute, result.TTL)
}

func TestRegisterMapsRegistrationError(t *testing.T) {
	svc := &testBrevCloudAgentService{
		registerFn: func(context.Context, *connect.Request[brevapiv2.RegisterRequest]) (*connect.Response[brevapiv2.RegisterResponse], error) {
			connectErr := connect.NewError(connect.CodeInvalidArgument, stderrors.New("bad token"))
			detail, detailErr := connect.NewErrorDetail(&brevapiv2.BrevCloudRegistrationErrorDetail{
				Reason:  brevapiv2.BrevCloudRegistrationErrorReason_BREV_CLOUD_REGISTRATION_ERROR_REASON_INVALID_TOKEN,
				Message: "token invalid",
			})
			require.NoError(t, detailErr)
			connectErr.AddDetail(detail)
			return nil, connectErr
		},
	}

	client := newTestClient(t, svc)
	_, err := client.Register(context.Background(), RegisterParams{
		RegistrationToken:     "token",
		DeviceFingerprintHash: "device-fp-hash",
		HardwareFingerprint:   "shape-fp",
	})
	require.Error(t, err)

	var regErr *RegistrationError
	require.True(t, stderrors.As(err, &regErr))
	require.Equal(t, brevapiv2.BrevCloudRegistrationErrorReason_BREV_CLOUD_REGISTRATION_ERROR_REASON_INVALID_TOKEN, regErr.Reason)
	require.Equal(t, "token invalid", regErr.Error())
}

func TestHeartbeatUnauthenticated(t *testing.T) {
	svc := &testBrevCloudAgentService{
		heartbeatFn: func(context.Context, *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
			return nil, connect.NewError(connect.CodeUnauthenticated, stderrors.New("invalid token"))
		},
	}

	client := newTestClient(t, svc)
	_, err := client.Heartbeat(context.Background(), HeartbeatParams{
		BrevCloudNodeID: "fn-1",
		DeviceToken:     "bad-token",
	})
	require.Error(t, err)
	require.True(t, stderrors.Is(err, ErrUnauthenticated))
}

type testBrevCloudAgentService struct {
	registerFn  func(ctx context.Context, req *connect.Request[brevapiv2.RegisterRequest]) (*connect.Response[brevapiv2.RegisterResponse], error)
	heartbeatFn func(ctx context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error)
	tunnelFn    func(ctx context.Context, req *connect.Request[brevapiv2.GetTunnelTokenRequest]) (*connect.Response[brevapiv2.GetTunnelTokenResponse], error)
}

func (s *testBrevCloudAgentService) Register(ctx context.Context, req *connect.Request[brevapiv2.RegisterRequest]) (*connect.Response[brevapiv2.RegisterResponse], error) {
	if s.registerFn != nil {
		return s.registerFn(ctx, req)
	}
	return connect.NewResponse(&brevapiv2.RegisterResponse{}), nil
}

func (s *testBrevCloudAgentService) Heartbeat(ctx context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
	if s.heartbeatFn != nil {
		return s.heartbeatFn(ctx, req)
	}
	return connect.NewResponse(&brevapiv2.HeartbeatResponse{}), nil
}

func (s *testBrevCloudAgentService) GetTunnelToken(ctx context.Context, req *connect.Request[brevapiv2.GetTunnelTokenRequest]) (*connect.Response[brevapiv2.GetTunnelTokenResponse], error) {
	if s.tunnelFn != nil {
		return s.tunnelFn(ctx, req)
	}
	return connect.NewResponse(&brevapiv2.GetTunnelTokenResponse{}), nil
}

func newTestClient(t *testing.T, handler brevapiv2connect.BrevCloudAgentServiceHandler) BrevCloudAgentClient {
	t.Helper()

	path, h := brevapiv2connect.NewBrevCloudAgentServiceHandler(handler)
	mux := http.NewServeMux()
	mux.Handle(path, h)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := New(config.Config{
		BrevCloudAgentURL: server.URL,
	})
	require.NoError(t, err)
	return client
}
