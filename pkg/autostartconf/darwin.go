package autostartconf

import (
	"errors"
	"os/exec"
	"path"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/hashicorp/go-multierror"
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

func (dpc DarwinPlistConfigurer) UnInstall() error {
	destination, err := dpc.GetDestination()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var allError error
	switch dpc.ServiceType {
	case System:
		_, err = exec.Command("launchctl", "unload", "-w", destination).CombinedOutput() // #nosec G204
		if err != nil {
			allError = multierror.Append(allError, err)
		}
	case SingleUser:
		_, err = exec.Command("launchctl", "bootout", "guui/"+dpc.Store.GetOSUser(), destination).CombinedOutput() // #nosec G204
		if err != nil {
			allError = multierror.Append(allError, err)
		}
	}

	err = dpc.Store.Remove(destination)
	if err != nil {
		allError = multierror.Append(allError, err)
	}
	// err = dpc.Store.Remove(targetBin)
	// if err != nil {
	// 	return breverrors.WrapAndTrace(err)
	// }
	if allError != nil {
		return breverrors.WrapAndTrace(allError)
	}
	return nil
}

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
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	commands, err := dpc.GetExecCommand()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = ExecCommands(commands)
	if err != nil {
		return breverrors.WrapAndTrace(err)
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
	destination := path.Join(destinationDirectory, dpc.ServiceName+".plist")
	return destination, nil
}

func (dpc DarwinPlistConfigurer) GetExecCommand() ([][]string, error) {
	destination, err := dpc.GetDestination()
	if err != nil {
		return [][]string{}, breverrors.WrapAndTrace(err)
	}
	switch dpc.ServiceType {
	case System:
		return [][]string{
			{"launchctl", "load", "-w", destination},
		}, nil
	case SingleUser: // todo still not sure if this works
		return [][]string{{"launchctl", "bootstrap", "gui/" + dpc.Store.GetOSUser(), destination}}, nil

	}
	return [][]string{}, errors.New("invalid service type")
}
