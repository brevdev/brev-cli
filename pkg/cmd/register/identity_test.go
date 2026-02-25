package register

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"testing"
)

// Test_ComputeHardwareFingerprint_Deterministic verifies that the same input
// always produces the same fingerprint.
func Test_ComputeHardwareFingerprint_Deterministic(t *testing.T) {
	desc := HardwareDescriptor{
		GPUs: []GPUDescriptor{
			{Model: "NVIDIA GB10", Memory: 137438953472},
		},
		RAM:  137438953472,
		CPUs: 12,
	}

	fp1, err := ComputeHardwareFingerprint(desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fp2, err := ComputeHardwareFingerprint(desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("fingerprints differ: %s != %s", fp1, fp2)
	}
}

// Test_ComputeHardwareFingerprint_GPUOrderIndependent verifies that GPU order
// does not affect the fingerprint.
func Test_ComputeHardwareFingerprint_GPUOrderIndependent(t *testing.T) {
	desc1 := HardwareDescriptor{
		GPUs: []GPUDescriptor{
			{Model: "NVIDIA A100", Memory: 85899345920},
			{Model: "NVIDIA GB10", Memory: 137438953472},
		},
		RAM:  274877906944,
		CPUs: 64,
	}
	desc2 := HardwareDescriptor{
		GPUs: []GPUDescriptor{
			{Model: "NVIDIA GB10", Memory: 137438953472},
			{Model: "NVIDIA A100", Memory: 85899345920},
		},
		RAM:  274877906944,
		CPUs: 64,
	}

	fp1, err := ComputeHardwareFingerprint(desc1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fp2, err := ComputeHardwareFingerprint(desc2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("fingerprints should be identical regardless of GPU order: %s != %s", fp1, fp2)
	}
}

// Test_ComputeHardwareFingerprint_ByteIdenticalToDevPlane verifies that our
// fingerprint is byte-identical to what dev-plane would produce. We replicate
// the dev-plane logic inline to prove equivalence.
func Test_ComputeHardwareFingerprint_ByteIdenticalToDevPlane(t *testing.T) {
	desc := HardwareDescriptor{
		GPUs: []GPUDescriptor{
			{Model: "NVIDIA GB10", Memory: 137438953472},
			{Model: "NVIDIA A100", Memory: 85899345920},
		},
		RAM:  274877906944,
		CPUs: 64,
	}

	// Compute using our function
	got, err := ComputeHardwareFingerprint(desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Replicate dev-plane logic exactly
	gpus := make([]GPUDescriptor, len(desc.GPUs))
	copy(gpus, desc.GPUs)
	sort.Slice(gpus, func(i, j int) bool {
		if gpus[i].Model == gpus[j].Model {
			return gpus[i].Memory < gpus[j].Memory
		}
		return gpus[i].Model < gpus[j].Model
	})
	// Build the same struct shape dev-plane uses
	type devPlaneGPU struct {
		Model  string `json:"model"`
		Memory int64  `json:"memory_bytes"`
	}
	type devPlaneDesc struct {
		GPUs []devPlaneGPU `json:"gpus"`
		RAM  int64         `json:"ram_bytes"`
		CPUs int           `json:"cpus"`
	}
	dpGPUs := make([]devPlaneGPU, len(gpus))
	for i, g := range gpus {
		dpGPUs[i] = devPlaneGPU{Model: g.Model, Memory: g.Memory}
	}
	dpDesc := devPlaneDesc{GPUs: dpGPUs, RAM: desc.RAM, CPUs: desc.CPUs}
	payload, err := json.Marshal(dpDesc)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	sum := sha256.Sum256(payload)
	want := hex.EncodeToString(sum[:])

	if got != want {
		t.Errorf("fingerprint mismatch with dev-plane logic:\ngot:  %s\nwant: %s", got, want)
	}
}

// Test_ComputeHardwareFingerprint_NoGPUs verifies fingerprinting works with
// no GPUs present.
func Test_ComputeHardwareFingerprint_NoGPUs(t *testing.T) {
	desc := HardwareDescriptor{
		GPUs: nil,
		RAM:  8589934592,
		CPUs: 4,
	}
	fp, err := ComputeHardwareFingerprint(desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp == "" {
		t.Fatal("expected non-empty fingerprint")
	}
}

// Test_ComputeHardwareFingerprint_ValidationErrors verifies validation.
func Test_ComputeHardwareFingerprint_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		desc HardwareDescriptor
	}{
		{
			name: "zero CPUs",
			desc: HardwareDescriptor{RAM: 1024, CPUs: 0},
		},
		{
			name: "zero RAM",
			desc: HardwareDescriptor{RAM: 0, CPUs: 1},
		},
		{
			name: "GPU with empty model",
			desc: HardwareDescriptor{
				RAM:  1024,
				CPUs: 1,
				GPUs: []GPUDescriptor{{Model: "", Memory: 1024}},
			},
		},
		{
			name: "GPU with zero memory",
			desc: HardwareDescriptor{
				RAM:  1024,
				CPUs: 1,
				GPUs: []GPUDescriptor{{Model: "test", Memory: 0}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ComputeHardwareFingerprint(tt.desc)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// Test_HardwareProfileToDescriptor verifies the conversion.
func Test_HardwareProfileToDescriptor(t *testing.T) {
	profile := &HardwareProfile{
		CPUCount: 12,
		RAMBytes: 137438953472,
		GPUs: []GPUInfo{
			{Name: "NVIDIA GB10", MemoryMB: 131072},
		},
	}

	desc := HardwareProfileToDescriptor(profile)
	if desc.CPUs != 12 {
		t.Errorf("expected 12 CPUs, got %d", desc.CPUs)
	}
	if desc.RAM != 137438953472 {
		t.Errorf("expected RAM 137438953472, got %d", desc.RAM)
	}
	if len(desc.GPUs) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(desc.GPUs))
	}
	if desc.GPUs[0].Model != "NVIDIA GB10" {
		t.Errorf("unexpected GPU model: %s", desc.GPUs[0].Model)
	}
	// 131072 MB = 131072 * 1024 * 1024 bytes
	expectedMem := int64(131072) * 1024 * 1024
	if desc.GPUs[0].Memory != expectedMem {
		t.Errorf("expected GPU memory %d, got %d", expectedMem, desc.GPUs[0].Memory)
	}
}
