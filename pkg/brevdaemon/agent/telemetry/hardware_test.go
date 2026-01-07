package telemetry

import (
	"context"
	"os/exec"
	"runtime"
	"testing"

	"github.com/brevdev/dev-plane/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDetectHardwareWithoutGPU(t *testing.T) {
	origGOOS := runtimeGOOS
	runtimeGOOS = goosLinux
	t.Cleanup(func() { runtimeGOOS = origGOOS })

	origReadFile := readFile
	readFile = func(path string) ([]byte, error) {
		switch path {
		case "/proc/meminfo":
			return []byte("MemTotal: 2048 kB\nMemAvailable: 1024 kB\n"), nil
		case "/sys/devices/virtual/dmi/id/product_name":
			return []byte("DGX-SYSTEM\n"), nil
		default:
			return nil, errors.Errorf("unexpected path %s", path)
		}
	}
	t.Cleanup(func() { readFile = origReadFile })

	origLookup := lookupExecutable
	lookupExecutable = func(cmd string) (string, error) {
		if cmd == "lsblk" {
			return "/bin/lsblk", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookupExecutable = origLookup })

	origExec := execCommand
	execCommand = func(_ context.Context, name string, _ ...string) ([]byte, error) {
		require.Equal(t, "lsblk", name)
		return []byte("NAME TYPE SIZE\nsda disk 102400000000\n"), nil
	}
	t.Cleanup(func() { execCommand = origExec })

	hw, err := DetectHardware(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2048*1024), hw.RAMBytes)
	require.Empty(t, hw.GPUs)
	require.Equal(t, "DGX-SYSTEM", hw.MachineModel)
	require.Len(t, hw.Storage, 1)
	require.Equal(t, int64(102400000000), hw.Storage[0].SizeBytes)
	require.Equal(t, "sda", hw.Storage[0].Name)
	require.Equal(t, "disk", hw.Storage[0].Type)
	require.Equal(t, runtime.GOARCH, hw.Architecture)
}

func TestParseGPUHardwareAggregatesCounts(t *testing.T) {
	raw := `
A100-SXM4-40GB, 40960
A100-SXM4-40GB, 40960
RTX 6000, 24576
`
	gpus, err := parseGPUHardware(raw)
	require.NoError(t, err)
	require.Len(t, gpus, 2)

	results := map[string]GPUInfo{}
	for _, gpu := range gpus {
		results[gpu.Model] = gpu
	}

	require.Equal(t, 2, results["A100-SXM4-40GB"].Count)
	require.Equal(t, int64(40960*1024*1024), results["A100-SXM4-40GB"].MemoryBytes)
	require.Equal(t, 1, results["RTX 6000"].Count)
}

func TestHardwareToClientConversion(t *testing.T) {
	info := HardwareInfo{
		CPUCount:     8,
		RAMBytes:     32 << 30,
		Architecture: "arm64",
		GPUs: []GPUInfo{
			{Model: "A100", MemoryBytes: 40 << 30, Count: 2},
		},
		Storage: []StorageInfo{
			{Name: "nvme0n1", SizeBytes: 512 << 30, Type: "nvme"},
		},
	}
	clientInfo := info.ToClient()
	require.Equal(t, 8, clientInfo.CPUCount)
	require.Equal(t, int64(32<<30), clientInfo.RAMBytes)
	require.Equal(t, "arm64", clientInfo.Architecture)
	require.Len(t, clientInfo.GPUs, 1)
	require.Equal(t, 2, clientInfo.GPUs[0].Count)
	require.Len(t, clientInfo.Storage, 1)
	require.Equal(t, "nvme0n1", clientInfo.Storage[0].Name)
	require.Equal(t, int64(512<<30), clientInfo.Storage[0].Capacity)
	require.Equal(t, "nvme", clientInfo.Storage[0].Type)
}
