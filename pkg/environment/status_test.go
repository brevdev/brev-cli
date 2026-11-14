package environment

import (
	"testing"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/stretchr/testify/assert"

	"github.com/brevdev/brev-cli/pkg/entity"
)

func TestStatusDisplayMapsEnvironmentStatuses(t *testing.T) {
	cases := []struct {
		name    string
		status  devplanev1.EnvironmentStatus
		display string
	}{
		{"pending", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING, entity.Deploying},
		{"preparing", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING, entity.Deploying},
		{"provisioning", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING, entity.Deploying},
		{"building", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING, entity.Deploying},
		{"running", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING, entity.Running},
		{"stopping", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING, entity.Stopping},
		{"stopped", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPED, entity.Stopped},
		{"starting", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING, entity.Starting},
		{"terminating", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING, entity.Deleting},
		{"terminated", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATED, entity.Deleting},
		{"preparing_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED, entity.Failure},
		{"provisioning_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED, entity.Failure},
		{"building_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING_FAILED, entity.Failure},
		{"starting_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING_FAILED, entity.Failure},
		{"stopping_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPING_FAILED, entity.Failure},
		{"terminating_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING_FAILED, entity.Failure},
		{"instance_lost", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST, entity.Failure},
		{"unspecified", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			display := Display(&devplanev1.Environment{
				EnvironmentId: "env-1",
				Name:          "box",
				Status:        tc.status,
			})
			assert.Equal(t, tc.display, display)
		})
	}
}

func TestSetupPhase(t *testing.T) {
	cases := []struct {
		name   string
		status devplanev1.EnvironmentStatus
		want   string
	}{
		{"pending", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING, "pending"},
		{"preparing", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING, "preparing"},
		{"provisioning", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING, "provisioning"},
		{"preparing_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING_FAILED, ""},
		{"provisioning_failed", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED, ""},
		{"building", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING, ""},
		{"starting", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STARTING, ""},
		{"running", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING, ""},
		{"unspecified", devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SetupPhase(&devplanev1.Environment{Status: tc.status}))
		})
	}
}

// TestFailureMessage covers the provider-message preference and fallbacks.
func TestFailureMessage(t *testing.T) {
	cases := []struct {
		name        string
		status      devplanev1.EnvironmentStatus
		instanceMsg string
		want        string
	}{
		{
			name:        "provider message wins",
			status:      devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
			instanceMsg: "out of quota",
			want:        "out of quota",
		},
		{
			name:   "instance lost falls back to label",
			status: devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_INSTANCE_LOST,
			want:   "instance lost",
		},
		{
			name:   "non-failure status has no message",
			status: devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING,
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := &devplanev1.Environment{
				EnvironmentId: "env-1",
				Name:          "box",
				Status:        tc.status,
			}
			if tc.instanceMsg != "" {
				env.Instance = &devplanev1.Instance{
					ProvisionStatus: &devplanev1.CreateAttemptStatus{Message: tc.instanceMsg},
				}
			}

			assert.Equal(t, tc.want, FailureMessage(env))
		})
	}
}
