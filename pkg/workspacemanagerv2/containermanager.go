package workspacemanagerv2

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type DockerContainerManager struct{}

var _ ContainerManager = DockerContainerManager{}

type inspectResult struct {
	ID    string `json:"Id"`
	State state  `json:"State"`
}

type state struct {
	Status string `json:"Status"`
}

type inspectResults []inspectResult

func (c DockerContainerManager) GetContainer(ctx context.Context, containerIdentifier string) (*Container, error) {
	cmd := exec.CommandContext(ctx, "docker", "container", "inspect", containerIdentifier)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, breverrors.WrapAndTrace(fmt.Errorf(string(out)))
	}

	res := inspectResults{}
	err = json.Unmarshal(out, &res)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no results")
	}
	return &Container{
		ID:     res[0].ID,
		Status: DockerStatusToContainerStatus(res[0].State.Status),
	}, nil
}

func DockerStatusToContainerStatus(status string) ContainerStatus {
	if status == "created" || status == "exited" {
		return ContainerStopped
	}
	return ContainerStatus(status)
}

func (c DockerContainerManager) StopContainer(ctx context.Context, containerIdentifier string) error {
	cmd := exec.CommandContext(ctx, "docker", "container", "stop", containerIdentifier)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf(string(out)))
	}
	return nil
}

func (c DockerContainerManager) DeleteContainer(ctx context.Context, containerIdentifier string) error {
	cmd := exec.CommandContext(ctx, "docker", "container", "rm", containerIdentifier)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf(string(out)))
	}
	return nil
}

func (c DockerContainerManager) StartContainer(ctx context.Context, containerIdentifier string) error {
	cmd := exec.CommandContext(ctx, "docker", "container", "start", containerIdentifier)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf(string(out)))
	}
	return nil
}

func (c DockerContainerManager) DeleteVolume(ctx context.Context, volumeName string) error {
	cmd := exec.CommandContext(ctx, "docker", "volume", "rm", volumeName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf(string(out)))
	}
	return nil
}

func (c DockerContainerManager) CreateContainer(ctx context.Context, options CreateContainerOptions, image string) (string, error) {
	// TODO use official docker client
	volumes := []string{}
	for _, v := range options.Volumes {
		volumes = append(volumes, "--volume", fmt.Sprintf("%s:%s", v.GetIdentifier(), v.GetMountToPath()))
	}
	ports := []string{}
	for _, p := range options.Ports {
		ports = append(ports, "--publish", p)
	}
	portsAndVolumes := append(ports, volumes...) //nolint:gocritic // not clear why the linter doesn't like this pattern
	createArgs := append([]string{"--name", options.Name, "--privileged"}, portsAndVolumes...)
	command := append([]string{options.Command}, options.CommandArgs...)
	postOptionArgs := append([]string{image}, command...)
	dockerArgs := append([]string{"container", "create"}, append(createArgs, postOptionArgs...)...)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", breverrors.WrapAndTrace(fmt.Errorf(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// [
//     {
//         "Id": "149a3bed2b0d0595df159e4eb032fdc62f3f3f508a59bacf86cfc9e131f98910",
//         "Created": "2022-05-14T00:33:39.375942173Z",
//         "Path": "/docker-entrypoint.sh",
//         "Args": [
//             "nginx",
//             "-g",
//             "daemon off;"
//         ],
//         "State": {
//             "Status": "exited",
//             "Running": false,
//             "Paused": false,
//             "Restarting": false,
//             "OOMKilled": false,
//             "Dead": false,
//             "Pid": 0,
//             "ExitCode": 0,
//             "Error": "",
//             "StartedAt": "2022-05-14T00:33:40.468926916Z",
//             "FinishedAt": "2022-05-14T00:33:42.19311363Z"
//         },
//         "Image": "sha256:7425d3a7c478efbeb75f0937060117343a9a510f72f5f7ad9f14b1501a36940c",
//         "ResolvConfPath": "/var/lib/docker/containers/149a3bed2b0d0595df159e4eb032fdc62f3f3f508a59bacf86cfc9e131f98910/resolv.conf",
//         "HostnamePath": "/var/lib/docker/containers/149a3bed2b0d0595df159e4eb032fdc62f3f3f508a59bacf86cfc9e131f98910/hostname",
//         "HostsPath": "/var/lib/docker/containers/149a3bed2b0d0595df159e4eb032fdc62f3f3f508a59bacf86cfc9e131f98910/hosts",
//         "LogPath": "/var/lib/docker/containers/149a3bed2b0d0595df159e4eb032fdc62f3f3f508a59bacf86cfc9e131f98910/149a3bed2b0d0595df159e4eb032fdc62f3f3f508a59bacf86cfc9e131f98910-json.log",
//         "Name": "/ecstatic_lichterman",
//         "RestartCount": 0,
//         "Driver": "overlay2",
//         "Platform": "linux",
//         "MountLabel": "",
//         "ProcessLabel": "",
//         "AppArmorProfile": "",
//         "ExecIDs": null,
//         "HostConfig": {
//             "Binds": null,
//             "ContainerIDFile": "",
//             "LogConfig": {
//                 "Type": "json-file",
//                 "Config": {}
//             },
//             "NetworkMode": "default",
//             "PortBindings": {},
//             "RestartPolicy": {
//                 "Name": "no",
//                 "MaximumRetryCount": 0
//             },
//             "AutoRemove": false,
//             "VolumeDriver": "",
//             "VolumesFrom": null,
//             "CapAdd": null,
//             "CapDrop": null,
//             "CgroupnsMode": "host",
//             "Dns": [],
//             "DnsOptions": [],
//             "DnsSearch": [],
//             "ExtraHosts": null,
//             "GroupAdd": null,
//             "IpcMode": "private",
//             "Cgroup": "",
//             "Links": null,
//             "OomScoreAdj": 0,
//             "PidMode": "",
//             "Privileged": false,
//             "PublishAllPorts": false,
//             "ReadonlyRootfs": false,
//             "SecurityOpt": null,
//             "UTSMode": "",
//             "UsernsMode": "",
//             "ShmSize": 67108864,
//             "Runtime": "runc",
//             "ConsoleSize": [
//                 0,
//                 0
//             ],
//             "Isolation": "",
//             "CpuShares": 0,
//             "Memory": 0,
//             "NanoCpus": 0,
//             "CgroupParent": "",
//             "BlkioWeight": 0,
//             "BlkioWeightDevice": [],
//             "BlkioDeviceReadBps": null,
//             "BlkioDeviceWriteBps": null,
//             "BlkioDeviceReadIOps": null,
//             "BlkioDeviceWriteIOps": null,
//             "CpuPeriod": 0,
//             "CpuQuota": 0,
//             "CpuRealtimePeriod": 0,
//             "CpuRealtimeRuntime": 0,
//             "CpusetCpus": "",
//             "CpusetMems": "",
//             "Devices": [],
//             "DeviceCgroupRules": null,
//             "DeviceRequests": null,
//             "KernelMemory": 0,
//             "KernelMemoryTCP": 0,
//             "MemoryReservation": 0,
//             "MemorySwap": 0,
//             "MemorySwappiness": null,
//             "OomKillDisable": false,
//             "PidsLimit": null,
//             "Ulimits": null,
//             "CpuCount": 0,
//             "CpuPercent": 0,
//             "IOMaximumIOps": 0,
//             "IOMaximumBandwidth": 0,
//             "MaskedPaths": [
//                 "/proc/asound",
//                 "/proc/acpi",
//                 "/proc/kcore",
//                 "/proc/keys",
//                 "/proc/latency_stats",
//                 "/proc/timer_list",
//                 "/proc/timer_stats",
//                 "/proc/sched_debug",
//                 "/proc/scsi",
//                 "/sys/firmware"
//             ],
//             "ReadonlyPaths": [
//                 "/proc/bus",
//                 "/proc/fs",
//                 "/proc/irq",
//                 "/proc/sys",
//                 "/proc/sysrq-trigger"
//             ]
//         },
//         "GraphDriver": {
//             "Data": {
//                 "LowerDir": "/var/lib/docker/overlay2/30f56bebc7a574013a60253618494c668f918f929098d60e476f6d1c07fb8f03-init/diff:/var/lib/docker/overlay2/fa7b0f78f551c8f7a4319a58cc5c98ece716afcbc496724caa399191d4222733/diff:/var/lib/docker/overlay2/773f813086e1c74c4dcad7970b449ae57372885528aa66beb94a28cda63cac71/diff:/var/lib/docker/overlay2/dc66da211f30d5cb69975ec8ffac758637b4c6a5f114d6fb876f2ac308f518ee/diff:/var/lib/docker/overlay2/4baccdbaebab6fce8fa33b1fb821f77c27967b2ed575b4aeee6ac545fb987189/diff:/var/lib/docker/overlay2/47f4b9cb4ee4ede5bf0cca6f40a84203288e0d14fa5cdfdf518347fe5f02a5f9/diff:/var/lib/docker/overlay2/aaea9506d207f44e3ab00538eb1f32a74c4df7b4d29d96f96611326840671597/diff",
//                 "MergedDir": "/var/lib/docker/overlay2/30f56bebc7a574013a60253618494c668f918f929098d60e476f6d1c07fb8f03/merged",
//                 "UpperDir": "/var/lib/docker/overlay2/30f56bebc7a574013a60253618494c668f918f929098d60e476f6d1c07fb8f03/diff",
//                 "WorkDir": "/var/lib/docker/overlay2/30f56bebc7a574013a60253618494c668f918f929098d60e476f6d1c07fb8f03/work"
//             },
//             "Name": "overlay2"
//         },
//         "Mounts": [],
//         "Config": {
//             "Hostname": "149a3bed2b0d",
//             "Domainname": "",
//             "User": "",
//             "AttachStdin": false,
//             "AttachStdout": true,
//             "AttachStderr": true,
//             "ExposedPorts": {
//                 "80/tcp": {}
//             },
//             "Tty": false,
//             "OpenStdin": false,
//             "StdinOnce": false,
//             "Env": [
//                 "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
//                 "NGINX_VERSION=1.21.6",
//                 "NJS_VERSION=0.7.2",
//                 "PKG_RELEASE=1~bullseye"
//             ],
//             "Cmd": [
//                 "nginx",
//                 "-g",
//                 "daemon off;"
//             ],
//             "Image": "nginx",
//             "Volumes": null,
//             "WorkingDir": "",
//             "Entrypoint": [
//                 "/docker-entrypoint.sh"
//             ],
//             "OnBuild": null,
//             "Labels": {
//                 "maintainer": "NGINX Docker Maintainers <docker-maint@nginx.com>"
//             },
//             "StopSignal": "SIGQUIT"
//         },
//         "NetworkSettings": {
//             "Bridge": "",
//             "SandboxID": "e3e9ef1e5e64793e242e6f7e25dc82f0c66fd182ef16fe03e9bcdd0f3f28123c",
//             "HairpinMode": false,
//             "LinkLocalIPv6Address": "",
//             "LinkLocalIPv6PrefixLen": 0,
//             "Ports": {},
//             "SandboxKey": "/var/run/docker/netns/e3e9ef1e5e64",
//             "SecondaryIPAddresses": null,
//             "SecondaryIPv6Addresses": null,
//             "EndpointID": "",
//             "Gateway": "",
//             "GlobalIPv6Address": "",
//             "GlobalIPv6PrefixLen": 0,
//             "IPAddress": "",
//             "IPPrefixLen": 0,
//             "IPv6Gateway": "",
//             "MacAddress": "",
//             "Networks": {
//                 "bridge": {
//                     "IPAMConfig": null,
//                     "Links": null,
//                     "Aliases": null,
//                     "NetworkID": "4110f5ecbb596b3368fc6c02dc61d50933f5296f124e1f1cc17c287b87baf43d",
//                     "EndpointID": "",
//                     "Gateway": "",
//                     "IPAddress": "",
//                     "IPPrefixLen": 0,
//                     "IPv6Gateway": "",
//                     "GlobalIPv6Address": "",
//                     "GlobalIPv6PrefixLen": 0,
//                     "MacAddress": "",
//                     "DriverOpts": null
//                 }
//             }
//         }
//     }
// ]
