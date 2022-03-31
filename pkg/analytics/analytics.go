package analytics

import (
	"encoding/json"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"gopkg.in/segmentio/analytics-go.v3"
)

type Analytics interface {
	TrackUserEvent(eventName EventName, userID string, properties Properties) error
	TrackUserWorkspaceEvent(eventName EventName, userID string, workspace *entity.Workspace, extraProps Properties) error
}

type (
	Properties map[string]interface{}
	EventName  string
)

const (
	SSHConnections  EventName = "SSH Connections"
	StopSSHSession  EventName = "All SSH Session Stop"
	StartSSHSession EventName = "All SSH Session Start"
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

func (s SegmentClient) TrackUserWorkspaceEvent(eventName EventName, userID string, workspace *entity.Workspace, extraProps Properties) error {
	workspaceProps, err := StructToMap(workspace)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	delete(workspaceProps, "id")
	workspaceProps["workspaceId"] = workspace.ID
	delete(workspaceProps, "password")

	for k, v := range workspaceProps {
		extraProps[k] = v
	}
	err = s.TrackUserEvent(eventName, userID, extraProps)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s SegmentClient) TrackUserEvent(eventName EventName, userID string, properties Properties) error {
	err := s.Client.Enqueue(
		analytics.Track{
			Event:      string(eventName),
			UserId:     userID,
			Properties: analytics.Properties(properties),
		},
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func StructToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj) // Convert to a json string
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	newMap := new(map[string]interface{})
	err = json.Unmarshal(data, newMap) // Convert to a map
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return *newMap, nil
}
