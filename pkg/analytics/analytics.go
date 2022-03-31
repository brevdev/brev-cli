package analytics

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"gopkg.in/segmentio/analytics-go.v3"
)

type Analytics interface {
	TrackUserEvent(eventName EventName, userID string, properties Properties) error
}

type (
	Properties map[string]interface{}
	EventName  string
)

const (
	SSHConnections  EventName = "SSH Connections"
	StopSSHSession  EventName = "SSH Session Stop"
	StartSSHSession EventName = "SSH Session Start"
)

type SegmentClient struct {
	Client analytics.Client
}

func NewSegmentClient(writeAPIKey string) SegmentClient {
	return SegmentClient{
		Client: analytics.New(writeAPIKey),
	}
}

var _ Analytics = SegmentClient{}

func (s SegmentClient) TrackUserEvent(eventName EventName, userID string, properties Properties) error {
	err := s.Client.Enqueue(
		analytics.Track{
			Event:      string(eventName),
			UserId:     userID,
			Properties: analytics.Properties(properties),
		},
	)
	return breverrors.WrapAndTrace(err)
}
