package register

import (
	"strings"
	"testing"
)

func Test_parseCPUCountContent_ValidInput(t *testing.T) {
	content := `processor	: 0
vendor_id	: AuthenticAMD
model name	: AMD EPYC 7763 64-Core Processor
cpu MHz		: 2450.000

processor	: 1
vendor_id	: AuthenticAMD
model name	: AMD EPYC 7763 64-Core Processor
cpu MHz		: 2450.000

processor	: 2
vendor_id	: AuthenticAMD
model name	: AMD EPYC 7763 64-Core Processor
cpu MHz		: 2450.000
`
	count, err := parseCPUCountContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 CPUs, got %d", count)
	}
}

func Test_parseCPUCountContent_EmptyInput(t *testing.T) {
	_, err := parseCPUCountContent("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func Test_parseMemInfoContent_ValidInput(t *testing.T) {
	content := `MemTotal:       131886028 kB
MemFree:         1234567 kB
MemAvailable:   98765432 kB
`
	bytes, err := parseMemInfoContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(131886028) * 1024
	if bytes != expected {
		t.Errorf("expected %d bytes, got %d", expected, bytes)
	}
}

func Test_parseMemInfoContent_MissingMemTotal(t *testing.T) {
	content := `MemFree:         1234567 kB
MemAvailable:   98765432 kB
`
	_, err := parseMemInfoContent(content)
	if err == nil {
		t.Fatal("expected error for missing MemTotal")
	}
}

func Test_parseOSReleaseContent(t *testing.T) {
	content := `NAME="Ubuntu"
VERSION="24.04 LTS (Noble Numbat)"
ID=ubuntu
VERSION_ID="24.04"
PRETTY_NAME="Ubuntu 24.04 LTS"
`
	name, version := parseOSReleaseContent(content)
	if name != "Ubuntu" {
		t.Errorf("expected Ubuntu, got %s", name)
	}
	if version != "24.04" {
		t.Errorf("expected 24.04, got %s", version)
	}
}

func Test_parseOSReleaseContent_Unquoted(t *testing.T) {
	content := `NAME=Fedora
VERSION_ID=39
`
	name, version := parseOSReleaseContent(content)
	if name != "Fedora" {
		t.Errorf("expected Fedora, got %s", name)
	}
	if version != "39" {
		t.Errorf("expected 39, got %s", version)
	}
}

func Test_parseNvidiaSMIOutput_GroupsByModel(t *testing.T) {
	output := `NVIDIA GB10, 131072
NVIDIA GB10, 131072
`
	gpus := parseNvidiaSMIOutput(output)
	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU group, got %d", len(gpus))
	}
	if gpus[0].Model != "NVIDIA GB10" {
		t.Errorf("unexpected GPU model: %s", gpus[0].Model)
	}
	if gpus[0].Count != 2 {
		t.Errorf("expected count 2, got %d", gpus[0].Count)
	}
	expectedMem := int64(131072) * 1024 * 1024
	if gpus[0].MemoryBytes == nil || *gpus[0].MemoryBytes != expectedMem {
		t.Errorf("expected %d bytes, got %v", expectedMem, gpus[0].MemoryBytes)
	}
}

func Test_parseNvidiaSMIOutput_MultipleModels(t *testing.T) {
	output := `NVIDIA A100, 81920
NVIDIA GB10, 131072
NVIDIA A100, 81920
`
	gpus := parseNvidiaSMIOutput(output)
	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPU groups, got %d", len(gpus))
	}
	if gpus[0].Model != "NVIDIA A100" || gpus[0].Count != 2 {
		t.Errorf("expected 2x NVIDIA A100, got %dx %s", gpus[0].Count, gpus[0].Model)
	}
	if gpus[1].Model != "NVIDIA GB10" || gpus[1].Count != 1 {
		t.Errorf("expected 1x NVIDIA GB10, got %dx %s", gpus[1].Count, gpus[1].Model)
	}
}

func Test_parseNvidiaSMIOutput_Empty(t *testing.T) {
	gpus := parseNvidiaSMIOutput("")
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(gpus))
	}
}

func Test_parseStorageOutput(t *testing.T) {
	output := `nvme0n1  500107862016 disk
nvme1n1  1000204886016 disk
sda  2048 rom
`
	totalBytes, storageType := parseStorageOutput(output)
	expected := int64(500107862016 + 1000204886016)
	if totalBytes != expected {
		t.Errorf("expected %d bytes, got %d", expected, totalBytes)
	}
	if storageType != "NVMe" {
		t.Errorf("expected NVMe, got %s", storageType)
	}
}

func Test_parseStorageOutput_SDA(t *testing.T) {
	output := `sda  500107862016 disk
`
	_, storageType := parseStorageOutput(output)
	if storageType != "SSD" {
		t.Errorf("expected SSD, got %s", storageType)
	}
}

func Test_unquote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"Ubuntu"`, "Ubuntu"},
		{`Ubuntu`, "Ubuntu"},
		{`""`, ""},
		{`"a"`, "a"},
		{``, ""},
	}
	for _, tt := range tests {
		got := unquote(tt.input)
		if got != tt.want {
			t.Errorf("unquote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func Test_FormatNodeSpec(t *testing.T) {
	cpuCount := int32(12)
	ramBytes := int64(137438953472) // 128 GB
	memBytes := int64(137438953472) // 128 GB
	s := &NodeSpec{
		CPUCount:     &cpuCount,
		RAMBytes:     &ramBytes,
		Architecture: "arm64",
		OS:           "Ubuntu",
		OSVersion:    "24.04",
		GPUs: []NodeGPU{
			{Model: "NVIDIA GB10", Count: 1, MemoryBytes: &memBytes},
		},
	}
	output := FormatNodeSpec(s)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(output, "12 cores") {
		t.Errorf("expected CPU info in output: %s", output)
	}
	if !strings.Contains(output, "128 GB") {
		t.Errorf("expected RAM info in output: %s", output)
	}
	if !strings.Contains(output, "NVIDIA GB10") {
		t.Errorf("expected GPU info in output: %s", output)
	}
}
