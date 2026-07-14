package register

import (
	"context"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

func TestSelectPortFromList_singlePortAutoSelect(t *testing.T) {
	ports := []*nodev1.Port{{PortId: "port_1", PortNumber: 22, ServerPort: 41920}}
	p, err := SelectPortFromList(context.Background(), terminal.New(), mockSelectorAlwaysFirst{}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if p.GetPortId() != "port_1" {
		t.Fatalf("got %q", p.GetPortId())
	}
}

func TestFormatPortLabel(t *testing.T) {
	label := FormatPortLabel(&nodev1.Port{PortId: "port_1", PortNumber: 11640, ServerPort: 22})
	want := "11640->22"
	if label != want {
		t.Fatalf("got %q, want %q", label, want)
	}
}

func TestFormatPortLabel_noServerPort(t *testing.T) {
	label := FormatPortLabel(&nodev1.Port{PortNumber: 22})
	if label != "22" {
		t.Fatalf("got %q", label)
	}
}

type mockSelectorAlwaysFirst struct{}

func (mockSelectorAlwaysFirst) Select(_ string, items []string) string {
	if len(items) > 0 {
		return items[0]
	}
	return ""
}
