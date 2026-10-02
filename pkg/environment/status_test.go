package environment

import (
	"testing"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

func TestResolveStatusMapsEnvironmentStatuses(t *testing.T) {
	cases := []struct {
		name    string
		status  devplanev1.EnvironmentStatus
		display string
		ready   bool
		failed  bool
	}{
		{"pending", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING, entity.Deploying, false, false},
		{"provisioning", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING, entity.Deploying, false, false},
		{"building", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING, entity.Deploying, false, false},
		{"running", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING, entity.Running, true, false},
		{"stopping", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING, entity.Stopping, false, false},
		{"stopped", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPED, entity.Stopped, false, false},
		{"starting", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING, entity.Starting, false, false},
		{"terminating", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING, entity.Deleting, false, false},
		{"provisioning_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED, entity.Failure, false, true},
		{"building_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED, entity.Failure, false, true},
		{"instance_lost", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST, entity.Failure, false, true},
		{"unspecified", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED, "", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := resolveStatus(&devplanev1.Environment{
				EnvironmentId: "env-1",
				Name:          "box",
				Status:        tc.status,
			})
			assert.Equal(t, tc.display, status.Display)
			assert.Equal(t, tc.ready, status.Ready)
			assert.Equal(t, tc.failed, status.Failed)
		})
	}
}

func TestResolveStatusUsesProvisionStatusMessage(t *testing.T) {
	status := resolveStatus(&devplanev1.Environment{
		EnvironmentId: "env-1",
		Name:          "box",
		Status:        devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
		Instance: &devplanev1.Instance{
			ProvisionStatus: &devplanev1.CreateAttemptStatus{Message: "out of quota"},
		},
	})

	require.True(t, status.Failed)
	assert.Equal(t, "out of quota", status.Message)
	assert.Equal(t, "box", status.Name)
}

func TestResolveStatusFallsBackToFailureText(t *testing.T) {
	status := resolveStatus(&devplanev1.Environment{
		EnvironmentId: "env-1",
		Name:          "box",
		Status:        devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST,
	})

	assert.Equal(t, "instance lost", status.Message)
}
