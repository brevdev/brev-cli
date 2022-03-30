package analytics

import (
	"gopkg.in/segmentio/analytics-go.v3"
)

type Analytics interface {
	TrackUserEvent(eventName EventName, userID string, properties Properties) error
}

type (
	Properties map[string]interface{}
	EventName  string
)

type SegmentClient struct {
	Client analytics.Client
}

func (s SegmentClient) TrackUserEvent(eventName EventName, userID string, properties Properties) error {
	return s.Client.Enqueue(
		analytics.Track{
			Event:      string(eventName),
			UserId:     userID,
			Properties: analytics.Properties(properties),
		},
	)
}
