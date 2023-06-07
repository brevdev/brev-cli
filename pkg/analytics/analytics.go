package analytics

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type EventData struct {
	EventName  string            `json:"eventName"`
	UserID     string            `json:"userId,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

func TrackEvent(data EventData) error {
	conf := config.NewConstants()

	url := conf.GetBrevAPIURl() + "/api/brevent"

	jsonData, err := json.Marshal(data)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	//nolint:errcheck
	defer resp.Body.Close()

	return nil
}
