package analytics

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/hashicorp/go-multierror"
)

func (c SSHMonitor) GetAllConnections(include ...string) ([]string, error) {
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

type SSHMonitor struct {
	connGetter func() ([]byte, error)
	lastStep   []SSData
}

func NewSSHMonitor() *SSHMonitor {
	return &SSHMonitor{
		connGetter: connGetter,
		lastStep:   []SSData{},
	}
}

func (c SSHMonitor) GetSSHConnections() ([]SSData, error) {
	res, err := c.GetAllConnections("ssh")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	sshRows := []SSData{}
	for _, r := range res {
		sshRows = append(sshRows, RowStrToSSRow(r))
	}
	return sshRows, nil
}

func (c *SSHMonitor) GetSSHSessionEvents() (EventName, error) {
	rows, err := c.GetSSHConnections()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	event := EventName("")
	if len(c.lastStep) == 0 && len(rows) > 0 {
		event = StartSSHSession
	}
	if len(c.lastStep) > 0 && len(rows) == 0 {
		event = StopSSHSession
	}
	c.lastStep = rows
	return event, nil
}

type SSHAnalyticsTask struct {
	SSHMonitor       *SSHMonitor
	Analytics        Analytics
	Store            SSHAnalyticsStore
	lastLocalPeerSet []string
	userID           string
}

// difference returns the elements in `a` that aren't in `b`.
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

type SSHAnalyticsStore interface {
	GetCurrentUserID() (string, error)
}

func (s *SSHAnalyticsTask) Run() error {
	fmt.Println("running ssh analytics...")
	ssData, err := s.SSHMonitor.GetSSHConnections()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	set := ssDataToLocalPeerSet(ssData)
	diffSetToLast := difference(set, s.lastLocalPeerSet)
	diffLastToSet := difference(s.lastLocalPeerSet, set)
	if len(append(diffSetToLast, diffLastToSet...)) > 0 {
		if s.userID == "" {
			userID, err1 := s.Store.GetCurrentUserID()
			if err1 != nil {
				return breverrors.WrapAndTrace(err1)
			}
			s.userID = userID
		}
		err = WriteSSHEvents(s.SSHMonitor, s.Analytics, s.userID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	s.lastLocalPeerSet = set
	return nil
}

func ssDataToLocalPeerSet(ss []SSData) []string {
	set := []string{}
	for _, s := range ss {
		set = append(set, fmt.Sprintf("%s-%s", s.LocalAddressPort, s.PeerAddressPort))
	}
	return set
}

func (s SSHAnalyticsTask) Configure() error {
	return nil
}

func (s SSHAnalyticsTask) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: "@every 2s"}
}

var _ tasks.Task = &SSHAnalyticsTask{}

func WriteSSHEvents(sshMonitor *SSHMonitor, analytics Analytics, userID string) error {
	fmt.Println("writing ssh events...")
	rows, err := sshMonitor.GetSSHConnections()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	var allError error
	err = analytics.TrackUserEvent(SSHConnections, userID, Properties{"connections": rows})
	if err != nil {
		allError = multierror.Append(allError, err)
	}

	event, err := sshMonitor.GetSSHSessionEvents()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if event != "" {
		err = analytics.TrackUserEvent(event, userID, Properties{})
		if err != nil {
			allError = multierror.Append(allError, err)
		}

		if allError != nil {
			return breverrors.WrapAndTrace(allError)
		}
	}

	return nil
}

type SSData struct {
	NetID            string `json:"netId"`
	State            string `json:"state"`
	RecvQ            string `json:"recvQ"`
	SendQ            string `json:"sendvQ"`
	LocalAddressPort string `json:"localAddressPort"`
	PeerAddressPort  string `json:"peerAddressPort"`
}

var re = regexp.MustCompile(`\s{2,}`)

func RowStrToSSRow(row string) SSData {
	cols := re.Split(row, -1)
	s := SSData{NetID: cols[0], State: cols[1], RecvQ: cols[2], SendQ: cols[3], LocalAddressPort: cols[4], PeerAddressPort: cols[5]}
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
