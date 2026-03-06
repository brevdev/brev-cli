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
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantVersion string
	}{
		{
			"Quoted",
			"NAME=\"Ubuntu\"\nVERSION=\"24.04 LTS (Noble Numbat)\"\nID=ubuntu\nVERSION_ID=\"24.04\"\nPRETTY_NAME=\"Ubuntu 24.04 LTS\"\n",
			"Ubuntu", "24.04",
		},
		{
			"Unquoted",
			"NAME=Fedora\nVERSION_ID=39\n",
			"Fedora", "39",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := parseOSReleaseContent(tt.input)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
		})
	}
}

func Test_parseStorageOutput(t *testing.T) {
	output := `nvme0n1  500107862016 disk 0
nvme1n1  1000204886016 disk 0
sda  2048 rom 1
`
	devices := parseStorageOutput(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices[0].StorageBytes != 500107862016 {
		t.Errorf("expected 500107862016, got %d", devices[0].StorageBytes)
	}
	if devices[0].StorageType != "SSD" {
		t.Errorf("expected SSD, got %s", devices[0].StorageType)
	}
	if devices[0].Name != "nvme0n1" {
		t.Errorf("expected name nvme0n1, got %s", devices[0].Name)
	}
	if devices[1].StorageBytes != 1000204886016 {
		t.Errorf("expected 1000204886016, got %d", devices[1].StorageBytes)
	}
	if devices[1].StorageType != "SSD" {
		t.Errorf("expected SSD, got %s", devices[1].StorageType)
	}
}

func Test_parseStorageOutput_SDA(t *testing.T) {
	output := `sda  500107862016 disk 1
`
	devices := parseStorageOutput(output)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].StorageBytes != 500107862016 {
		t.Errorf("expected 500107862016 bytes, got %d", devices[0].StorageBytes)
	}
	if devices[0].StorageType != "HDD" {
		t.Errorf("expected HDD, got %s", devices[0].StorageType)
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
	cpuCount := int32(12)
	ramBytes := int64(137438953472) // 128 GB
	memBytes := int64(137438953472) // 128 GB
	s := &HardwareProfile{
		CPUCount:     &cpuCount,
		RAMBytes:     &ramBytes,
		Architecture: "arm64",
		OS:           "Ubuntu",
		OSVersion:    "24.04",
		ProductName:  "DGX Spark",
		GPUs: []GPU{
			{Model: "NVIDIA GB10", Architecture: "Blackwell", Count: 1, MemoryBytes: &memBytes},
		},
		Interconnects: []Interconnect{
			{Type: "NVLink", Device: "GPU 0", ActiveLinks: 4, Version: 4},
		},
	}
	output := FormatHardwareProfile(s)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(output, "12 cores") {
		t.Errorf("expected CPU info in output: %s", output)
	}
	if !strings.Contains(output, "128.0 GB") {
		t.Errorf("expected RAM info in output: %s", output)
	}
	if !strings.Contains(output, "NVIDIA GB10") {
		t.Errorf("expected GPU info in output: %s", output)
	}
	if !strings.Contains(output, "Blackwell") {
		t.Errorf("expected GPU arch in output: %s", output)
	}
	if !strings.Contains(output, "DGX Spark") {
		t.Errorf("expected product name in output: %s", output)
	}
	if !strings.Contains(output, "NVLink") {
		t.Errorf("expected interconnect in output: %s", output)
	}
}

func Test_FormatHardwareProfile_Nil(t *testing.T) {
	output := FormatHardwareProfile(nil)
	if output != "" {
		t.Errorf("expected empty string for nil input, got: %q", output)
	}
}

func Test_FormatHardwareProfile_PCIeInterconnect(t *testing.T) {
	s := &HardwareProfile{
		Architecture: "amd64",
		Interconnects: []Interconnect{
			{Type: "PCIe", Device: "GPU 0", Generation: 4, Width: 16},
		},
	}
	output := FormatHardwareProfile(s)
	if !strings.Contains(output, "PCIe Gen4 x16") {
		t.Errorf("expected 'PCIe Gen4 x16' in output, got: %s", output)
	}
	if !strings.Contains(output, "(GPU 0)") {
		t.Errorf("expected '(GPU 0)' in output, got: %s", output)
	}
}

func Test_FormatHardwareProfile_MinimalFields(t *testing.T) {
	s := &HardwareProfile{
		GPUs: []GPU{
			{Model: "NVIDIA GB10", Count: 1},
		},
		Architecture: "arm64",
	}
	output := FormatHardwareProfile(s)
	if strings.Contains(output, "CPU:") {
		t.Errorf("should not contain CPU when nil: %s", output)
	}
	if strings.Contains(output, "RAM:") {
		t.Errorf("should not contain RAM when nil: %s", output)
	}
	if !strings.Contains(output, "NVIDIA GB10") {
		t.Errorf("expected GPU info: %s", output)
	}
	if !strings.Contains(output, "arm64") {
		t.Errorf("expected arch info: %s", output)
	}
}

func Test_FormatHardwareProfile_WithStorage(t *testing.T) {
	s := &HardwareProfile{
		Architecture: "amd64",
		Storage: []StorageDevice{
			{Name: "nvme0n1", StorageBytes: 500107862016, StorageType: "SSD"},
			{Name: "sda", StorageBytes: 1000204886016, StorageType: "HDD"},
		},
	}
	output := FormatHardwareProfile(s)
	if !strings.Contains(output, "Storage:") {
		t.Errorf("expected storage in output: %s", output)
	}
	if !strings.Contains(output, "SSD") {
		t.Errorf("expected SSD in output: %s", output)
	}
	if !strings.Contains(output, "HDD") {
		t.Errorf("expected HDD in output: %s", output)
	}
	if !strings.Contains(output, "nvme0n1") {
		t.Errorf("expected device name in output: %s", output)
	}
}

func Test_parseStorageOutput_NoValidDevices(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Empty", ""},
		{"NoDiskDevices", "sr0  1073741312 rom 1\nloop0  123456 loop 0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devices := parseStorageOutput(tt.input)
			if len(devices) != 0 {
				t.Errorf("expected 0 devices, got %d", len(devices))
			}
		})
	}
}

// mockHardwareProfiler implements HardwareProfiler for tests.
type mockHardwareProfiler struct {
	profile *HardwareProfile
	err     error
}

func (m *mockHardwareProfiler) Profile() (*HardwareProfile, error) {
	return m.profile, m.err
}
