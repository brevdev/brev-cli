package environment

import (
	"strings"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/entity"
)

// Status is the brev CLI view of a DevPlane environment's lifecycle.
type Status struct {
	Name    string
	Display string // entity workspace status constant; empty when unmapped
	Message string
	Ready   bool
	Failed  bool
	Deleted bool
}

// resolveStatus maps a DevPlane environment onto the brev CLI status model.
func resolveStatus(env *devplanev1.Environment) Status {
	if env == nil {
		return Status{}
	}

	status := Status{Name: env.GetName()}
	if instance := env.GetInstance(); instance != nil {
		status.Message = instance.GetProvisionStatus().GetMessage()
	}

	switch env.GetStatus() {
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING:
		status.Display = entity.Deploying
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST:
		status.Display = entity.Failure
		status.Failed = true
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING:
		status.Display = entity.Running
		status.Ready = true
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING_FAILED:
		status.Display = entity.Stopping
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPED:
		status.Display = entity.Stopped
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING:
		status.Display = entity.Starting
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING_FAILED:
		status.Display = entity.Deleting
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATED:
		status.Display = entity.Deleting
		status.Deleted = true
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED:
		// Leave unmapped; callers keep the status they already had.
	}

	if status.Failed && status.Message == "" {
		status.Message = failureText(env.GetStatus())
	}
	return status
}

func failureText(status devplanev1.EnvironmentStatus) string {
	switch status {
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED:
		return "preparation failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED:
		return "provisioning failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED:
		return "build failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING_FAILED:
		return "start failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST:
		return "instance lost"
	}
	return strings.ToLower(status.String())
}
