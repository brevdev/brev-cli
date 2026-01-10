package client

import (
	"net/http"

	brevapiv2connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/brev-cli/pkg/errors"
)

// Client is an alias to the generated BrevCloud agent RPC client.
type Client = brevapiv2connect.BrevCloudAgentServiceClient

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

// New constructs a generated BrevCloud agent client backed by the Connect RPC client.
func New(cfg config.Config, opts ...Option) (brevapiv2connect.BrevCloudAgentServiceClient, error) {
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

	if merged.customRPC != nil {
		return merged.customRPC, nil
	}

	return brevapiv2connect.NewBrevCloudAgentServiceClient(httpClient, cfg.BrevCloudAgentURL, merged.clientOpts...), nil
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

// ClassifyError unwraps Connect errors to return richer error types used by callers.
func ClassifyError(err error) error {
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

// BearerToken returns the HTTP Authorization header value for the provided token.
func BearerToken(token string) string {
	return "Bearer " + token
}

// ProtoString returns a protobuf-compatible optional string pointer when the value is non-empty.
func ProtoString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
