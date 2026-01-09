package telemetry

import (
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestUtilizationToProtoConversion(t *testing.T) {
	temp := float32(65)
	util := UtilizationInfo{
		CPUPercent:       55.5,
		MemoryUsedBytes:  4 << 30,
		MemoryTotalBytes: 16 << 30,
		DiskPercent:      25,
		DiskUsedBytes:    100 << 30,
		DiskTotalBytes:   400 << 30,
		GPUs: []GPUUtilization{
			{
				Index:              0,
				Model:              "A100",
				UtilizationPercent: 80,
				MemoryUsedBytes:    10 << 30,
				MemoryTotalBytes:   40 << 30,
				TemperatureCelsius: &temp,
			},
		},
	}

	clientUtil := util.ToProto()
	require.Equal(t, util.CPUPercent, clientUtil.GetCpuPercent())
	require.Equal(t, util.MemoryUsedBytes, clientUtil.GetMemoryUsed().GetValue())
	require.Len(t, clientUtil.GetGpus(), 1)
	require.Equal(t, temp, clientUtil.GetGpus()[0].GetTemperatureCelsius())
}

func TestParseMeminfoFallbacks(t *testing.T) {
	data := []byte(`MemTotal:       2048 kB
MemFree:        512 kB
Buffers:        256 kB
Cached:         256 kB
`)
	total, available, err := parseMeminfo(data)
	require.NoError(t, err)
	require.Equal(t, int64(2048*1024), total)
	require.Equal(t, int64(1024*1024), available)
}

func TestSampleUtilizationLinux(t *testing.T) {
	origGOOS := runtimeGOOS
	runtimeGOOS = goosLinux
	t.Cleanup(func() { runtimeGOOS = origGOOS })

	statReads := atomic.Int32{}

	origRead := readFile
	readFile = func(path string) ([]byte, error) {
		switch path {
		case "/proc/meminfo":
			return []byte("MemTotal: 2048 kB\nMemAvailable: 1024 kB\n"), nil
		case "/proc/stat":
			if statReads.Add(1) == 1 {
				return []byte("cpu  100 0 100 200 0 0 0 0 0 0\n"), nil
			}
			return []byte("cpu  150 0 100 250 0 0 0 0 0 0\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	}
	t.Cleanup(func() { readFile = origRead })

	origStatfs := statfsFunc
	statfsFunc = func(_ string, st *unix.Statfs_t) error {
		st.Blocks = 100
		st.Bsize = 1024
		st.Bfree = 20
		return nil
	}
	t.Cleanup(func() { statfsFunc = origStatfs })

	origLookup := lookupExecutable
	lookupExecutable = func(string) (string, error) {
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookupExecutable = origLookup })

	origWait := cpuSampleWait
	cpuSampleWait = 0
	t.Cleanup(func() { cpuSampleWait = origWait })

	util, err := SampleUtilization(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1024*1024), util.MemoryUsedBytes)
	require.Equal(t, int64(2048*1024), util.MemoryTotalBytes)
	require.InEpsilon(t, 50, util.CPUPercent, 0.01)
	require.Equal(t, int64((100-20)*1024), util.DiskUsedBytes)
	require.Equal(t, int64(100*1024), util.DiskTotalBytes)
	require.InEpsilon(t, 80, util.DiskPercent, 0.01)
}

func TestParseGPUUtilization(t *testing.T) {
	raw := `
0, A100, 80, 1024, 4096, 65
1, A100, 60, 2048, 4096, 0
`
	gpus, err := parseGPUUtilization(raw)
	require.NoError(t, err)
	require.Len(t, gpus, 2)
	require.Equal(t, 0, gpus[0].Index)
	require.Equal(t, float32(80), gpus[0].UtilizationPercent)
	require.Equal(t, int64(1024*1024*1024), gpus[0].MemoryUsedBytes)
	require.NotNil(t, gpus[0].TemperatureCelsius)
}
