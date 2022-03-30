package analytics

import (
	"os/exec"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (c ConnectionLister) GetAllSSConnections(include ...string) ([]string, error) {
	out, err := c.connGetter()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	newLineSplit := strings.Split(string(out), "\n")
	resLines := []string{}
	for _, l := range newLineSplit {
		if StringIncludes(l, include) {
			resLines = append(resLines, l)
		}
	}

	return resLines, nil
}

func connGetter() ([]byte, error) {
	cmd := exec.Command("ss", "--no-header")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

// returns true if empty list provided
func StringIncludes(check string, shouldInclude []string) bool {
	if len(shouldInclude) == 0 {
		return true
	}
	for _, i := range shouldInclude {
		if strings.Contains(check, i) {
			return true
		}
	}
	return false
}

type ConnectionLister struct {
	connGetter func() ([]byte, error)
}

func NewConnLister() ConnectionLister {
	return ConnectionLister{
		connGetter: connGetter,
	}
}

func (c ConnectionLister) GetSSHConnections() ([]SSHSSRow, error) {
	res, err := c.GetAllSSConnections("ssh")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	sshRows := []SSHSSRow{}
	for _, r := range res {
		sshRows = append(sshRows, RowStrToSSRow(r))
	}
	return sshRows, nil
}

type SSHSSRow struct {
	NetID            string
	State            string
	RecvQ            string
	SendQ            string
	LocalAddressPort string
	PeerAddressPort  string
	Process          string
}

func RowStrToSSRow(row string) SSHSSRow {
	f := strings.Fields(row)
	s := SSHSSRow{NetID: f[0], State: f[1], RecvQ: f[2], SendQ: f[3], LocalAddressPort: f[4], PeerAddressPort: f[5]}
	return s
}
