package client

import (
	"context"
	"math"
	"net/http"
	"time"

	brevapiv2connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/dev-plane/pkg/brevcloud/tunnel"
	"github.com/brevdev/brev-cli/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BrevCloudAgentClient defines the BrevCloud agent RPCs exercised by the brev-agent binary.
type BrevCloudAgentClient interface {
	Register(ctx context.Context, req RegisterParams) (RegisterResult, error)
	Heartbeat(ctx context.Context, req HeartbeatParams) (HeartbeatResult, error)
	GetTunnelToken(ctx context.Context, req TunnelTokenParams) (TunnelTokenResult, error)
}

// RegisterParams carries the payload for BrevCloudAgentService.Register.
type RegisterParams struct {
	RegistrationToken     string
	DisplayName           string
	CloudName             string
	Capabilities          []string
	Hardware              *HardwareInfo
	AgentVersion          string
	HardwareFingerprint   string
	DeviceFingerprintHash string
}

// RegisterResult captures the subset of response fields the agent cares about.
type RegisterResult struct {
	BrevCloudNodeID   string
	DeviceToken       string
	HeartbeatInterval time.Duration
	DisplayName       string
	CloudName         string
	CloudCredID       string
	DeviceFingerprint string
}

// HeartbeatParams drives BrevCloudAgentService.Heartbeat.
type HeartbeatParams struct {
	BrevCloudNodeID       string
	DeviceToken           string
	ObservedAt            time.Time
	Status                *HeartbeatStatus
	Utilization           *UtilizationInfo
	AgentVersion          string
	DisplayName           string
	CloudName             string
	DeviceFingerprintHash string
	HardwareFingerprint   string
}

// HeartbeatStatus mirrors the minimal metadata fields surfaced by the proto.
type HeartbeatStatus struct {
	Phase              NodePhase
	Detail             string
	LastTransitionTime *time.Time
}

// HeartbeatResult returns server guidance after a heartbeat.
type HeartbeatResult struct {
	ServerTime            time.Time
	NextHeartbeatInterval time.Duration
	NodeConfig            *brevapiv2.BrevCloudNodeConfig
	Commands              []*brevapiv2.BrevCloudCommand
}

// TunnelTokenParams drives BrevCloudAgentService.GetTunnelToken.
type TunnelTokenParams struct {
	BrevCloudNodeID string
	DeviceToken     string
	TunnelName      string
	Ports           []tunnel.TunnelPortMapping
	AppIngresses    []AppIngress
}

// AppIngress describes a single HTTP ingress request from the agent to the control plane.
type AppIngress struct {
	AppID          string
	Protocol       string
	LocalPort      int
	RemotePort     int
	HostnamePrefix string
	PathPrefix     string
	ForceHTTPS     bool
}

// TunnelTokenResult returns tunnel connection metadata.
type TunnelTokenResult struct {
	Token        string
	Endpoint     string
	TTL          time.Duration
	ExpiresAt    *time.Time
	SecondsToExp *time.Duration
	PortMappings []*brevapiv2.TunnelPortMapping
}

// HardwareInfo is the DTO the agent uses when registering.
type HardwareInfo struct {
	CPUCount     int
	RAMBytes     int64
	GPUs         []GPUInfo
	MachineModel string
	Architecture string
	Storage      []StorageInfo
}

// GPUInfo captures high-level GPU specs.
type GPUInfo struct {
	Model       string
	MemoryBytes int64
	Count       int
}

// StorageInfo captures block device capacity for registration.
type StorageInfo struct {
	Name     string
	Capacity int64
	Type     string
}

// UtilizationInfo wraps the runtime metrics included in heartbeats.
type UtilizationInfo struct {
	CPUPercent       float32
	MemoryUsedBytes  int64
	MemoryTotalBytes int64
	DiskPercent      float32
	DiskUsedBytes    int64
	DiskTotalBytes   int64
	GPUs             []GPUUtilization
}

// GPUUtilization mirrors the proto payload for GPU metrics.
type GPUUtilization struct {
	Index              int
	Model              string
	UtilizationPercent float32
	MemoryUsedBytes    int64
	MemoryTotalBytes   int64
	TemperatureCelsius *float32
}

// NodePhase aligns with devplaneapi.v1.BrevCloudNodePhase without exposing proto enums to callers.
type NodePhase int

const (
	NodePhaseUnspecified NodePhase = iota
	NodePhaseWaitingForRegistration
	NodePhaseActive
	NodePhaseOffline
	NodePhaseStopped
	NodePhaseError
)

// Option configures client construction.
type Option func(*options)

type options struct {
	httpClient connect.HTTPClient
	clientOpts []connect.ClientOption
	customRPC  brevapiv2connect.BrevCloudAgentServiceClient
}

// WithHTTPClient overrides the HTTP client used for RPCs.
func WithHTTPClient(httpClient connect.HTTPClient) Option {
	return func(o *options) {
		o.httpClient = httpClient
	}
}

// WithClientOptions forwards raw connect client options to the underlying RPC client.
func WithClientOptions(opts ...connect.ClientOption) Option {
	return func(o *options) {
		o.clientOpts = append(o.clientOpts, opts...)
	}
}

// WithRPCClient injects a pre-built BrevCloudAgentService client. Primarily used for tests.
func WithRPCClient(rpc brevapiv2connect.BrevCloudAgentServiceClient) Option {
	return func(o *options) {
		o.customRPC = rpc
	}
}

// ErrUnauthenticated indicates the control plane rejected a token.
var ErrUnauthenticated = errors.New("brevcloudagent: unauthenticated request")

// RegistrationError provides structured context when registration is rejected.
type RegistrationError struct {
	Reason brevapiv2.BrevCloudRegistrationErrorReason
	Msg    string
}

// Error satisfies the error interface.
func (r *RegistrationError) Error() string {
	if r == nil {
		return "registration error"
	}
	return r.Msg
}

// New constructs a BrevCloudAgentClient backed by the Connect RPC client.
func New(cfg config.Config, opts ...Option) (BrevCloudAgentClient, error) {
	if cfg.BrevCloudAgentURL == "" {
		return nil, errors.Errorf("brevcloud agent URL is required")
	}

	merged := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&merged)
		}
	}
	httpClient := merged.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	var rpcClient brevapiv2connect.BrevCloudAgentServiceClient
	if merged.customRPC != nil {
		rpcClient = merged.customRPC
	} else {
		rpcClient = brevapiv2connect.NewBrevCloudAgentServiceClient(httpClient, cfg.BrevCloudAgentURL, merged.clientOpts...)
	}

	return &brevcloudAgentClient{
		rpc: rpcClient,
	}, nil
}

type brevcloudAgentClient struct {
	rpc brevapiv2connect.BrevCloudAgentServiceClient
}

func (c *brevcloudAgentClient) Register(ctx context.Context, params RegisterParams) (RegisterResult, error) {
	if params.RegistrationToken == "" {
		return RegisterResult{}, errors.Errorf("registration token is required")
	}
	if params.DeviceFingerprintHash == "" {
		return RegisterResult{}, errors.Errorf("device fingerprint hash is required")
	}

	req := &brevapiv2.RegisterRequest{
		RegistrationToken:     params.RegistrationToken,
		Capabilities:          params.Capabilities,
		Hardware:              hardwareInfoToProto(params.Hardware),
		DeviceFingerprintHash: params.DeviceFingerprintHash,
		HardwareFingerprint:   params.HardwareFingerprint,
	}
	if params.DisplayName != "" {
		req.DisplayName = protoString(params.DisplayName)
	}
	if params.CloudName != "" {
		req.CloudName = protoString(params.CloudName)
	}
	if params.AgentVersion != "" {
		req.Agent = &brevapiv2.AgentInfo{
			Version: params.AgentVersion,
		}
	}

	resp, err := c.rpc.Register(ctx, connect.NewRequest(req))
	if err != nil {
		return RegisterResult{}, classifyError(err)
	}

	result := RegisterResult{
		BrevCloudNodeID:   resp.Msg.GetBrevCloudNodeId(),
		DeviceToken:       resp.Msg.GetDeviceToken(),
		DisplayName:       resp.Msg.GetDisplayName(),
		CloudName:         resp.Msg.GetCloudName(),
		CloudCredID:       resp.Msg.GetCloudCredId(),
		DeviceFingerprint: resp.Msg.GetDeviceFingerprint(),
	}
	if interval := resp.Msg.GetHeartbeatInterval(); interval != nil {
		result.HeartbeatInterval = interval.AsDuration()
	}
	return result, nil
}

func (c *brevcloudAgentClient) Heartbeat(ctx context.Context, params HeartbeatParams) (HeartbeatResult, error) {
	if params.BrevCloudNodeID == "" {
		return HeartbeatResult{}, errors.Errorf("brevcloud node id is required")
	}
	if params.DeviceToken == "" {
		return HeartbeatResult{}, errors.Errorf("device token is required")
	}

	req := &brevapiv2.HeartbeatRequest{
		BrevCloudNodeId: params.BrevCloudNodeID,
		Utilization:     utilizationToProto(params.Utilization),
	}

	if !params.ObservedAt.IsZero() {
		req.ObservedAt = timestamppbNew(params.ObservedAt)
	}
	if params.AgentVersion != "" {
		req.Agent = &brevapiv2.AgentInfo{
			Version: params.AgentVersion,
		}
	}
	if params.Status != nil {
		req.Status = heartbeatStatusToProto(params.Status)
	}
	if params.DisplayName != "" {
		req.DisplayName = protoString(params.DisplayName)
	}
	if params.CloudName != "" {
		req.CloudName = protoString(params.CloudName)
	}
	if params.DeviceFingerprintHash != "" {
		req.DeviceFingerprintHash = params.DeviceFingerprintHash
	}
	if params.HardwareFingerprint != "" {
		req.HardwareFingerprint = params.HardwareFingerprint
	}

	connectReq := connect.NewRequest(req)
	connectReq.Header().Set("Authorization", bearerToken(params.DeviceToken))

	resp, err := c.rpc.Heartbeat(ctx, connectReq)
	if err != nil {
		return HeartbeatResult{}, classifyError(err)
	}

	result := HeartbeatResult{
		NodeConfig: resp.Msg.GetNodeConfig(),
		Commands:   resp.Msg.GetCommands(),
	}
	if ts := resp.Msg.GetServerTime(); ts != nil {
		result.ServerTime = ts.AsTime()
	}
	if interval := resp.Msg.GetNextHeartbeatInterval(); interval != nil {
		result.NextHeartbeatInterval = interval.AsDuration()
	}
	return result, nil
}

func (c *brevcloudAgentClient) GetTunnelToken(ctx context.Context, params TunnelTokenParams) (TunnelTokenResult, error) {
	if params.BrevCloudNodeID == "" {
		return TunnelTokenResult{}, errors.Errorf("brevcloud node id is required")
	}
	if params.DeviceToken == "" {
		return TunnelTokenResult{}, errors.Errorf("device token is required")
	}

	req := &brevapiv2.GetTunnelTokenRequest{
		BrevCloudNodeId: params.BrevCloudNodeID,
		RequestedPorts:  tunnelPortsToProto(params.Ports),
	}
	if params.TunnelName != "" {
		req.TunnelName = protoString(params.TunnelName)
	}
	if ingresses := appIngressesToProto(params.AppIngresses); len(ingresses) > 0 {
		req.AppIngresses = ingresses
	}

	connectReq := connect.NewRequest(req)
	connectReq.Header().Set("Authorization", bearerToken(params.DeviceToken))

	resp, err := c.rpc.GetTunnelToken(ctx, connectReq)
	if err != nil {
		return TunnelTokenResult{}, classifyError(err)
	}

	result := TunnelTokenResult{
		Token:        resp.Msg.GetToken(),
		Endpoint:     resp.Msg.GetEndpoint(),
		PortMappings: resp.Msg.GetPortMappings(),
	}
	if ttl := resp.Msg.GetTtl(); ttl != nil {
		result.TTL = ttl.AsDuration()
	}
	if expires := resp.Msg.GetExpiresAt(); expires != nil {
		t := expires.AsTime()
		result.ExpiresAt = &t
		if now := time.Now(); t.After(now) {
			d := t.Sub(now)
			result.SecondsToExp = &d
		}
	}
	return result, nil
}

func hardwareInfoToProto(info *HardwareInfo) *brevapiv2.HardwareInfo {
	if info == nil {
		return nil
	}

	cpuCount := clampToInt32(info.CPUCount)
	out := &brevapiv2.HardwareInfo{
		CpuCount: cpuCount,
	}
	if info.RAMBytes > 0 {
		out.RamBytes = bytesValue(info.RAMBytes)
	}
	if info.MachineModel != "" {
		out.SystemModel = protoString(info.MachineModel)
	}
	if info.Architecture != "" {
		out.Architecture = protoString(info.Architecture)
	}
	if len(info.Storage) > 0 {
		out.Storage = make([]*brevapiv2.StorageInfo, 0, len(info.Storage))
		for _, s := range info.Storage {
			entry := &brevapiv2.StorageInfo{
				Name: s.Name,
				Type: s.Type,
			}
			if s.Capacity > 0 {
				entry.Capacity = bytesValue(s.Capacity)
			}
			out.Storage = append(out.Storage, entry)
		}
	}
	if len(info.GPUs) > 0 {
		out.Gpus = make([]*brevapiv2.GPUInfo, 0, len(info.GPUs))
		for _, gpu := range info.GPUs {
			out.Gpus = append(out.Gpus, &brevapiv2.GPUInfo{
				Model:       gpu.Model,
				Count:       clampToInt32(gpu.Count),
				MemoryBytes: bytesValue(gpu.MemoryBytes),
			})
		}
	}
	return out
}

func utilizationToProto(info *UtilizationInfo) *brevapiv2.ResourceUtilization {
	if info == nil {
		return nil
	}
	out := &brevapiv2.ResourceUtilization{
		CpuPercent:  info.CPUPercent,
		DiskPercent: info.DiskPercent,
	}
	if info.MemoryUsedBytes > 0 {
		out.MemoryUsed = bytesValue(info.MemoryUsedBytes)
	}
	if info.MemoryTotalBytes > 0 {
		out.MemoryTotal = bytesValue(info.MemoryTotalBytes)
	}
	if info.DiskUsedBytes > 0 {
		out.DiskUsed = bytesValue(info.DiskUsedBytes)
	}
	if info.DiskTotalBytes > 0 {
		out.DiskTotal = bytesValue(info.DiskTotalBytes)
	}
	if len(info.GPUs) > 0 {
		out.Gpus = make([]*brevapiv2.GPUUtilization, 0, len(info.GPUs))
		for _, gpu := range info.GPUs {
			out.Gpus = append(out.Gpus, gpuUtilizationToProto(gpu))
		}
	}
	return out
}

func heartbeatStatusToProto(status *HeartbeatStatus) *brevapiv2.BrevCloudNodeStatus {
	if status == nil {
		return nil
	}
	out := &brevapiv2.BrevCloudNodeStatus{
		Phase:  convertNodePhase(status.Phase),
		Detail: status.Detail,
	}
	if status.LastTransitionTime != nil && !status.LastTransitionTime.IsZero() {
		out.LastTransitionTime = timestamppbNew(*status.LastTransitionTime)
	}
	return out
}

func gpuUtilizationToProto(gpu GPUUtilization) *brevapiv2.GPUUtilization {
	out := &brevapiv2.GPUUtilization{
		Index:              clampToInt32(gpu.Index),
		Model:              gpu.Model,
		UtilizationPercent: gpu.UtilizationPercent,
	}
	if gpu.MemoryUsedBytes > 0 {
		out.MemoryUsed = bytesValue(gpu.MemoryUsedBytes)
	}
	if gpu.MemoryTotalBytes > 0 {
		out.MemoryTotal = bytesValue(gpu.MemoryTotalBytes)
	}
	if gpu.TemperatureCelsius != nil {
		out.TemperatureCelsius = gpu.TemperatureCelsius
	}
	return out
}

func tunnelPortsToProto(ports []tunnel.TunnelPortMapping) []*brevapiv2.TunnelPortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]*brevapiv2.TunnelPortMapping, 0, len(ports))
	for _, port := range ports {
		lp := port.LocalPort
		rp := port.RemotePort
		if lp <= 0 && rp <= 0 {
			continue
		}
		if rp <= 0 {
			rp = lp
		}
		if lp <= 0 {
			lp = rp
		}
		if rp > math.MaxInt32 || lp > math.MaxInt32 || lp < math.MinInt32 || rp < math.MinInt32 {
			continue
		}
		out = append(out, &brevapiv2.TunnelPortMapping{
			LocalPort:  int32(lp),
			RemotePort: int32(rp),
			Protocol:   port.Protocol,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appIngressesToProto(ingresses []AppIngress) []*brevapiv2.AppIngressRequest {
	if len(ingresses) == 0 {
		return nil
	}

	out := make([]*brevapiv2.AppIngressRequest, 0, len(ingresses))
	for _, ingress := range ingresses {
		lp := ingress.LocalPort
		rp := ingress.RemotePort

		if lp <= 0 || lp > math.MaxInt32 || rp < 0 || rp > math.MaxInt32 {
			continue
		}

		out = append(out, &brevapiv2.AppIngressRequest{
			AppId:          ingress.AppID,
			Protocol:       ingress.Protocol,
			LocalPort:      int32(lp), //nolint:gosec // G115: range checked above.
			RemotePort:     int32(rp),
			HostnamePrefix: ingress.HostnamePrefix,
			PathPrefix:     ingress.PathPrefix,
			ForceHttps:     ingress.ForceHTTPS,
		})
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func bytesValue(v int64) *devplaneapiv1.Bytes {
	if v <= 0 {
		return nil
	}
	return &devplaneapiv1.Bytes{Value: v}
}

func convertNodePhase(phase NodePhase) brevapiv2.BrevCloudNodePhase {
	switch phase {
	case NodePhaseWaitingForRegistration:
		return brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_WAITING_FOR_REGISTRATION
	case NodePhaseActive:
		return brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE
	case NodePhaseOffline:
		return brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_OFFLINE
	case NodePhaseStopped:
		return brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_STOPPED
	case NodePhaseError:
		return brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ERROR
	default:
		return brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_UNSPECIFIED
	}
}

func classifyError(err error) error {
	if err == nil {
		return nil
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		if regErr := registrationErrorFromConnect(connectErr); regErr != nil {
			return errors.WrapAndTrace(regErr)
		}
		if connectErr.Code() == connect.CodeUnauthenticated {
			return errors.WrapAndTrace(errors.Join(ErrUnauthenticated, err))
		}
	}
	return errors.WrapAndTrace(err)
}

func registrationErrorFromConnect(err *connect.Error) error {
	if err == nil {
		return nil
	}
	for _, detail := range err.Details() {
		msg, detailErr := detail.Value()
		if detailErr != nil {
			continue
		}
		regDetail, ok := msg.(*brevapiv2.BrevCloudRegistrationErrorDetail)
		if !ok {
			continue
		}
		return &RegistrationError{
			Reason: regDetail.GetReason(),
			Msg:    regDetail.GetMessage(),
		}
	}
	return nil
}

func protoString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timestamppbNew(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func bearerToken(token string) string {
	return "Bearer " + token
}

func clampToInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
