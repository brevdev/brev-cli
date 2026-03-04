package portforward

import (
	"fmt"
	"testing"
)

func TestParsePortString_Valid(t *testing.T) {
	local, remote, err := parsePortString("8080:3000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local != "8080" {
		t.Errorf("expected local 8080, got %s", local)
	}
	if remote != "3000" {
		t.Errorf("expected remote 3000, got %s", remote)
	}
}

func TestParsePortString_SamePort(t *testing.T) {
	local, remote, err := parsePortString("8080:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local != "8080" || remote != "8080" {
		t.Errorf("expected 8080:8080, got %s:%s", local, remote)
	}
}

func TestParsePortString_NoColon(t *testing.T) {
	_, _, err := parsePortString("8080")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

func TestParsePortString_TooManyColons(t *testing.T) {
	_, _, err := parsePortString("8080:3000:443")
	if err == nil {
		t.Fatal("expected error for too many colons")
	}
}

func TestParsePortString_Empty(t *testing.T) {
	_, _, err := parsePortString("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestIsPortAlreadyAllocatedError_True(t *testing.T) {
	err := fmt.Errorf("skybridge API error: 400, body: Port 8080 is already allocated for this client")
	if !isPortAlreadyAllocatedError(err) {
		t.Error("expected true for 'already allocated' error")
	}
}

func TestIsPortAlreadyAllocatedError_False(t *testing.T) {
	err := fmt.Errorf("connection refused")
	if isPortAlreadyAllocatedError(err) {
		t.Error("expected false for unrelated error")
	}
}

func TestIsPortAlreadyAllocatedError_Nil(t *testing.T) {
	if isPortAlreadyAllocatedError(nil) {
		t.Error("expected false for nil error")
	}
}
