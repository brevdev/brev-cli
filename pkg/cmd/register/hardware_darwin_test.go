//go:build darwin

package register

import (
	"testing"
)

func Test_parseDiskutilSize(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want int64
	}{
		{
			"BytesFirst",
			"500107862016 Bytes (exactly 976773168 512-Byte-Units)",
			500107862016,
		},
		{
			"HumanReadableFirst",
			"1.0 TB (1,000,000,000,000 Bytes)",
			1000000000000,
		},
		{
			"GBWithParenBytes",
			"500.1 GB (500,107,862,016 Bytes)",
			500107862016,
		},
		{
			"NoParensFallback",
			"500107862016",
			500107862016,
		},
		{
			"Empty",
			"",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &StorageDevice{}
			parseDiskutilSize(tt.val, dev)
			if dev.StorageBytes != tt.want {
				t.Errorf("parseDiskutilSize(%q) = %d, want %d", tt.val, dev.StorageBytes, tt.want)
			}
		})
	}
}

func Test_isWholeDisk(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"disk0", true},
		{"disk1", true},
		{"disk0s1", false},
		{"disk2s3", false},
		{"notadisk", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWholeDisk(tt.name); got != tt.want {
				t.Errorf("isWholeDisk(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func Test_parseDiskutilSolidState(t *testing.T) {
	tests := []struct {
		val  string
		want string
	}{
		{"Yes", "SSD"},
		{"yes", "SSD"},
		{"No", "HDD"},
		{"no", "HDD"},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			dev := &StorageDevice{}
			parseDiskutilSolidState(tt.val, dev)
			if dev.StorageType != tt.want {
				t.Errorf("parseDiskutilSolidState(%q) → %q, want %q", tt.val, dev.StorageType, tt.want)
			}
		})
	}
}

func Test_parseDiskutilInfoOutput(t *testing.T) {
	output := `**********

   Device Identifier:        disk0
   Device Node:              /dev/disk0
   Whole:                    Yes
   Part of Whole:            disk0
   Disk Size:                500.1 GB (500,107,862,016 Bytes)
   Protocol:                 NVMe
   Solid State:              Yes
   Device / Media Name:      APPLE SSD AP0512Q

**********

   Device Identifier:        disk0s1
   Device Node:              /dev/disk0s1
   Whole:                    No
   Part of Whole:            disk0
   Disk Size:                524.3 MB (524,288,000 Bytes)

**********

   Device Identifier:        disk1
   Device Node:              /dev/disk1
   Whole:                    Yes
   Part of Whole:            disk1
   Disk Size:                1.0 TB (1,000,000,000,000 Bytes)
   Solid State:              No

`

	devices := parseDiskutilInfoOutput(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	if devices[0].Name != "disk0" {
		t.Errorf("device 0 name = %q, want disk0", devices[0].Name)
	}
	if devices[0].StorageBytes != 500107862016 {
		t.Errorf("device 0 bytes = %d, want 500107862016", devices[0].StorageBytes)
	}
	if devices[0].StorageType != "NVMe" {
		t.Errorf("device 0 type = %q, want NVMe", devices[0].StorageType)
	}

	if devices[1].Name != "disk1" {
		t.Errorf("device 1 name = %q, want disk1", devices[1].Name)
	}
	if devices[1].StorageBytes != 1000000000000 {
		t.Errorf("device 1 bytes = %d, want 1000000000000", devices[1].StorageBytes)
	}
	if devices[1].StorageType != "HDD" {
		t.Errorf("device 1 type = %q, want HDD", devices[1].StorageType)
	}
}

func Test_parseDiskutilInfoOutput_Empty(t *testing.T) {
	devices := parseDiskutilInfoOutput("")
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}
