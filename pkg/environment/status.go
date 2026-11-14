// Package environment provides shared operations for DevPlane environments.
package environment

import (
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/entity"
)

// Display maps environment status onto workspace status
func Display(env *devplanev1.Environment) string {
	switch env.GetStatus() {
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING:
		return entity.Running
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPED:
		return entity.Stopped
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING:
		return entity.Starting
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING:
		return entity.Stopping
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATED:
		return entity.Deleting
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST:
		return entity.Failure
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING:
		return entity.Deploying
	}
	return ""
}

func FailureMessage(env *devplanev1.Environment) string {
	if message := env.GetInstance().GetProvisionStatus().GetMessage(); message != "" {
		return message
	}
	switch env.GetStatus() {
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED:
		return "preparation failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED:
		return "provisioning failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED:
		return "build failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING_FAILED:
		return "start failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING_FAILED:
		return "stop failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING_FAILED:
		return "termination failed"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST:
		return "instance lost"
	}
	return ""
}

func IsFailed(env *devplanev1.Environment) bool {
	switch env.GetStatus() {
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING_FAILED,
		devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST:
		return true
	}
	return false
}

// SetupPhase returns the setup phase the environment is in, or "" once setup is over.
func SetupPhase(env *devplanev1.Environment) string {
	switch env.GetStatus() {
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING:
		return "pending"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING:
		return "preparing"
	case devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING:
		return "provisioning"
	}
	return ""
}
