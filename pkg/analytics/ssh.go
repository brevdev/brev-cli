package analytics

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (c ConnectionLister) GetAllConnections(include ...string) ([]string, error) {
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
	res, err := c.GetAllConnections("ssh")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	sshRows := []SSHSSRow{}
	for _, r := range res {
		sshRows = append(sshRows, RowStrToSSRow(r))
	}
	return sshRows, nil
}

// func (c ConnectionLister) GetAnalyticsEvents()

type SSHSSRow struct {
	NetID            string
	State            string
	RecvQ            string
	SendQ            string
	LocalAddressPort string
	PeerAddressPort  string
}

var re = regexp.MustCompile("\\s{2,}")

func RowStrToSSRow(row string) SSHSSRow {
	cols := re.Split(row, -1)
	s := SSHSSRow{NetID: cols[0], State: cols[1], RecvQ: cols[2], SendQ: cols[3], LocalAddressPort: cols[4], PeerAddressPort: cols[5]}
	return s
}

func StructToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj) // Convert to a json string
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	newMap := new(map[string]interface{})
	err = json.Unmarshal(data, newMap) // Convert to a map
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return *newMap, nil
}
