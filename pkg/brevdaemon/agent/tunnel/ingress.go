package tunnel

import (
	"context"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/dev-plane/pkg/brevcloud/appaccess"
	"github.com/brevdev/dev-plane/pkg/errors"
)

const (
	dashboardServiceName = "dgx-dashboard.service"
	loopbackHost         = "127.0.0.1"
)

type (
	probeFunc         func(ctx context.Context, host string, port int, timeout time.Duration) error
	httpProbeFunc     func(ctx context.Context, host string, port int, protocol string, timeout time.Duration) error
	systemdStatusFunc func(ctx context.Context, service string, timeout time.Duration) (active bool, supported bool, err error)
)

func detectInstanceTypeFromHardware(hw telemetry.HardwareInfo) appaccess.InstanceType {
	model := strings.ToUpper(hw.MachineModel)
	if strings.Contains(model, "DGX") {
		return appaccess.InstanceTypeDGXSpark
	}
	for _, gpu := range hw.GPUs {
		if strings.Contains(strings.ToUpper(gpu.Model), "DGX") {
			return appaccess.InstanceTypeDGXSpark
		}
	}
	return appaccess.InstanceTypeUnknown
}

func buildAppIngresses(ctx context.Context, cfg appaccess.Config, tcpProbe probeFunc, httpProbe httpProbeFunc, systemdCheck systemdStatusFunc, timeout time.Duration, instanceType appaccess.InstanceType) []client.AppIngress {
	if instanceType != appaccess.InstanceTypeDGXSpark {
		return nil
	}
	apps := cfg.AllowedAppsForInstance(appaccess.InstanceTypeDGXSpark)
	if len(apps) == 0 {
		return nil
	}
	if tcpProbe == nil {
		tcpProbe = defaultIngressProbe
	}
	if httpProbe == nil {
		httpProbe = defaultHTTPProbe
	}
	if systemdCheck == nil {
		systemdCheck = defaultSystemdStatus
	}

	timeout = clampProbeTimeout(timeout)
	ingresses := make([]client.AppIngress, 0, len(apps))
	for _, spec := range apps {
		if spec.DefaultPort <= 0 {
			continue
		}

		switch spec.ID {
		case appaccess.AppIDDGXDashboard:
			if dashboardHealthy(ctx, systemdCheck, httpProbe, spec.Protocol, spec.DefaultPort, timeout) {
				ingresses = append(ingresses, appIngressFromSpec(spec))
			}
		case appaccess.AppIDJupyter:
			if err := httpProbe(ctx, loopbackHost, spec.DefaultPort, spec.Protocol, timeout); err == nil {
				ingresses = append(ingresses, appIngressFromSpec(spec))
			}
		default:
			if err := tcpProbe(ctx, loopbackHost, spec.DefaultPort, timeout); err == nil {
				ingresses = append(ingresses, appIngressFromSpec(spec))
			}
		}
	}
	if len(ingresses) == 0 {
		return nil
	}
	return ingresses
}

func appIngressFromSpec(spec appaccess.AppSpec) client.AppIngress {
	pathPrefix := spec.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}
	return client.AppIngress{
		AppID:          string(spec.ID),
		Protocol:       spec.Protocol,
		LocalPort:      spec.DefaultPort,
		HostnamePrefix: string(spec.ID),
		PathPrefix:     pathPrefix,
		ForceHTTPS:     spec.ForceHTTPS,
	}
}

func dashboardHealthy(ctx context.Context, systemdCheck systemdStatusFunc, httpProbe httpProbeFunc, protocol string, port int, timeout time.Duration) bool {
	active, supported, err := systemdCheck(ctx, dashboardServiceName, timeout)
	if err != nil {
		return false
	}
	if supported {
		return active
	}
	if httpProbe == nil {
		return false
	}
	return httpProbe(ctx, loopbackHost, port, protocol, timeout) == nil
}

func defaultSystemdStatus(ctx context.Context, service string, timeout time.Duration) (bool, bool, error) {
	if service == "" {
		return false, false, errors.Errorf("service name is required")
	}
	timeout = clampProbeTimeout(timeout)
	statusCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(statusCtx, "systemctl", "show", "-p", "ActiveState", "--value", service)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, false, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.ToLower(string(exitErr.Stderr))
			if strings.Contains(stderr, "system has not been booted with systemd") || strings.Contains(stderr, "failed to connect to bus") {
				return false, false, nil
			}
			return false, true, nil
		}
		return false, false, errors.WrapAndTrace(err)
	}

	state := strings.TrimSpace(string(out))
	return strings.EqualFold(state, "active"), true, nil
}

func defaultHTTPProbe(ctx context.Context, host string, port int, protocol string, timeout time.Duration) error {
	if host == "" {
		host = loopbackHost
	}
	if port <= 0 {
		return errors.Errorf("invalid port %d", port)
	}

	if protocol == "" {
		protocol = "http"
	}

	timeout = clampProbeTimeout(timeout)
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, protocol+"://"+net.JoinHostPort(host, strconv.Itoa(port)), nil)
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return errors.WrapAndTrace(resp.Body.Close())
}

func defaultIngressProbe(ctx context.Context, host string, port int, timeout time.Duration) error {
	if host == "" {
		host = loopbackHost
	}
	if port <= 0 {
		return errors.Errorf("invalid port %d", port)
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return errors.WrapAndTrace(conn.Close())
}

func clampProbeTimeout(timeout time.Duration) time.Duration {
	switch {
	case timeout <= 0:
		return 750 * time.Millisecond
	case timeout > 10*time.Second:
		return 10 * time.Second
	default:
		return timeout
	}
}
