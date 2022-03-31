package analytics

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
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
	workspace        *entity.Workspace
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
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func (s *SSHAnalyticsTask) Run() error {
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
		if s.workspace == nil || s.workspace.Status == "DEPLOYING" {
			workspaceID, err1 := s.Store.GetCurrentWorkspaceID()
			if err1 != nil {
				return breverrors.WrapAndTrace(err1)
			}
			workspace, err1 := s.Store.GetWorkspace(workspaceID)
			if err1 != nil {
				return breverrors.WrapAndTrace(err1)
			}
			s.workspace = workspace
		}
		err = WriteSSHEvents(s.SSHMonitor, s.Analytics, s.userID, s.workspace)
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

func WriteSSHEvents(sshMonitor *SSHMonitor, analytics Analytics, userID string, workspace *entity.Workspace) error {
	if workspace.Status == "DEPLOYING" {
		fmt.Println("not writing ssh since DEPLOYING")
		return nil
	}
	fmt.Println("writing ssh events...")
	rows, err := sshMonitor.GetSSHConnections()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	var allError error
	err = analytics.TrackUserWorkspaceEvent(SSHConnections, userID, workspace, Properties{"connections": rows})
	if err != nil {
		allError = multierror.Append(allError, err)
	}

	event, err := sshMonitor.GetSSHSessionEvents()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if event != "" {
		err = analytics.TrackUserWorkspaceEvent(event, userID, workspace, Properties{})
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

var moreThanTwoWhitespace = regexp.MustCompile(`\s{2,}`)

func RowStrToSSRow(row string) SSData {
	cols := moreThanTwoWhitespace.Split(row, -1)
	s := SSData{NetID: cols[0], State: cols[1], RecvQ: cols[2], SendQ: cols[3], LocalAddressPort: cols[4], PeerAddressPort: cols[5]}
	return s
}
