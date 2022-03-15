package autostartconf

import (
	"errors"
	"fmt"
	"os/exec"
	"path"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type DarwinServiceType string

const (
	System     DarwinServiceType = "system"
	SingleUser DarwinServiceType = "singleuser"
)

type DarwinPlistConfigurer struct {
	Store           AutoStartStore
	ValueConfigFile string
	ServiceName     string
	ServiceType     DarwinServiceType
}

func (dpc DarwinPlistConfigurer) UnInstall() error { return nil }
func (dpc DarwinPlistConfigurer) Install() error {
	_ = dpc.UnInstall()
	err := dpc.Store.CopyBin(targetBin)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	destination, err := dpc.GetDestination()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = dpc.Store.WriteString(destination, dpc.ValueConfigFile)
	execCommand, err := dpc.GetExecCommand()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	out, err := exec.Command("launchctl", execCommand...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running launchctl load %s: %v, %s", destination, err, out)
	}
	return nil
}

func (dpc DarwinPlistConfigurer) GetDestinationDirectory() (string, error) {
	switch dpc.ServiceType {
	case System:
		return "/Library/LaunchDaemons", nil
	case SingleUser:
		homeDir, err := dpc.Store.UserHomeDir()
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		return path.Join(homeDir, "/Library/LaunchDaemons"), nil

	}
	return "", errors.New("invalid service type")
}

func (dpc DarwinPlistConfigurer) GetDestination() (string, error) {
	destinationDirectory, err := dpc.GetDestinationDirectory()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	destination := path.Join(destinationDirectory, dpc.ServiceName)
	return destination, nil
}

func (dpc DarwinPlistConfigurer) GetExecCommand() ([]string, error) {
	destination, err := dpc.GetDestination()
	if err != nil {
		return []string{}, breverrors.WrapAndTrace(err)
	}
	switch dpc.ServiceType {
	case System:
		return []string{"bootstrap", "system/" + dpc.ServiceName, destination}, nil
	case SingleUser:
		return []string{"bootstrap", "gui/" + dpc.Store.GetOSUser(), destination}, nil

	}
	return []string{}, errors.New("invalid service type")
}
