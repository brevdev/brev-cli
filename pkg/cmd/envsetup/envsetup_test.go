package envsetup

import "testing"

func Test_appendLogToFile(t *testing.T) {
	t.Skip()
	err := appendLogToFile("test", "test")
	if err != nil {
		t.Errorf("error appending to file %s", err)
	}
}

func Test_MOTDExists(t *testing.T) {
	if motd == "" {
		t.Errorf("motd is empty")
	}
}

func Test_SpeedtestExists(t *testing.T) {
	if speedtest == "" {
		t.Errorf("speedtest is empty")
	}
}