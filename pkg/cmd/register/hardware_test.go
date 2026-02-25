package register

import (
	"testing"
)

func Test_parseCPUInfoContent_ValidInput(t *testing.T) {
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
	count, model, err := parseCPUInfoContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 CPUs, got %d", count)
	}
	if model != "AMD EPYC 7763 64-Core Processor" {
		t.Errorf("unexpected CPU model: %s", model)
	}
}

func Test_parseCPUInfoContent_EmptyInput(t *testing.T) {
	_, _, err := parseCPUInfoContent("")
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

func Test_parseNvidiaSMIOutput(t *testing.T) {
	output := `NVIDIA GB10, 131072, 570.86.15, 00000000:01:00.0
NVIDIA GB10, 131072, 570.86.15, 00000000:02:00.0
`
	gpus := parseNvidiaSMIOutput(output)
	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(gpus))
	}
	if gpus[0].Name != "NVIDIA GB10" {
		t.Errorf("unexpected GPU name: %s", gpus[0].Name)
	}
	if gpus[0].MemoryMB != 131072 {
		t.Errorf("expected 131072 MB, got %d", gpus[0].MemoryMB)
	}
	if gpus[0].DriverVersion != "570.86.15" {
		t.Errorf("unexpected driver version: %s", gpus[0].DriverVersion)
	}
	if gpus[0].PCIBusID != "00000000:01:00.0" {
		t.Errorf("unexpected PCI bus ID: %s", gpus[0].PCIBusID)
	}
}

func Test_parseNvidiaSMIOutput_Empty(t *testing.T) {
	gpus := parseNvidiaSMIOutput("")
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(gpus))
	}
}

func Test_parseLsblkOutput(t *testing.T) {
	output := `sda  500107862016 disk
nvme0n1  1000204886016 disk
`
	devices := parseLsblkOutput(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices[0].Name != "sda" {
		t.Errorf("unexpected device name: %s", devices[0].Name)
	}
	if devices[0].Bytes != 500107862016 {
		t.Errorf("unexpected device size: %d", devices[0].Bytes)
	}
	if devices[0].Type != "disk" {
		t.Errorf("unexpected device type: %s", devices[0].Type)
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

func Test_FormatHardwareProfile(t *testing.T) {
	p := &HardwareProfile{
		CPUCount:     12,
		CPUModel:     "AMD EPYC 7763",
		RAMBytes:     137438953472, // 128 GB
		Architecture: "arm64",
		OSName:       "Ubuntu",
		OSVersion:    "24.04",
		GPUs: []GPUInfo{
			{Name: "NVIDIA GB10", MemoryMB: 131072},
		},
	}
	output := FormatHardwareProfile(p)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(output, "12 x AMD EPYC 7763") {
		t.Errorf("expected CPU info in output: %s", output)
	}
	if !contains(output, "128 GB") {
		t.Errorf("expected RAM info in output: %s", output)
	}
	if !contains(output, "NVIDIA GB10") {
		t.Errorf("expected GPU info in output: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
