package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLocalIdentifier(t *testing.T) {
	w := Workspace{DNS: "test-rand-org.brev.sh", Name: "test-rand"}
	assert.Equal(t, WorkspaceLocalID("test-rand"), w.GetLocalIdentifier(nil))
}

func TestGetLocalIdentifierClean(t *testing.T) {
	// safest https://www.saveonhosting.com/scripts/index.php?rp=/knowledgebase/52/What-are-the-valid-characters-for-a-domain-name-and-how-long-can-a-domain-name-be.html
	correctIDFromName := WorkspaceLocalID("abc-def")
	correctIDFromDNS := WorkspaceLocalID("test-rand")
	w := Workspace{DNS: "test-rand-org.brev.sh", Name: "abc/def"}
	assert.Equal(t, correctIDFromName, w.GetLocalIdentifier(nil))

	w.Name = "'abc"
	assert.Equal(t, correctIDFromDNS, w.GetLocalIdentifier(nil))

	w.Name = "\"abc"
	assert.Equal(t, correctIDFromDNS, w.GetLocalIdentifier(nil))

	w.Name = "\\abc"
	assert.Equal(t, correctIDFromDNS, w.GetLocalIdentifier(nil))

	w.Name = "/abc"
	assert.Equal(t, correctIDFromDNS, w.GetLocalIdentifier(nil))

	w.Name = ".abc"
	assert.Equal(t, correctIDFromDNS, w.GetLocalIdentifier(nil))

	w.Name = ":abc"
	assert.Equal(t, correctIDFromDNS, w.GetLocalIdentifier(nil))
}

// NADER IS SO FUCKING SORRY FOR DOING THIS TWICE BUT I HAVE NO CLUE WHERE THIS HELPER FUNCTION SHOULD GO SO ITS COPY/PASTED ELSEWHERE
// IF YOU MODIFY IT MODIFY IT EVERYWHERE OR PLEASE PUT IT IN ITS PROPER PLACE. thank you you're the best <3
func WorkspacesFromWorkspaceWithMeta(wwm []WorkspaceWithMeta) []Workspace {
	var workspaces []Workspace

	for _, v := range wwm {
		workspaces = append(workspaces, Workspace{
			ID:                v.ID,
			Name:              v.Name,
			WorkspaceGroupID:  v.WorkspaceGroupID,
			OrganizationID:    v.OrganizationID,
			WorkspaceClassID:  v.WorkspaceClassID,
			CreatedByUserID:   v.CreatedByUserID,
			DNS:               v.DNS,
			Status:            v.Status,
			Password:          v.Password,
			GitRepo:           v.GitRepo,
			Version:           v.Version,
			WorkspaceTemplate: v.WorkspaceTemplate,
		})
	}

	return workspaces
}

func TestGetLocalIdentifierDeterminism(t *testing.T) {
	w1 := WorkspaceWithMeta{Workspace: Workspace{ID: "1", DNS: "main-abc-org.brev.sh", Name: "main", CreatedByUserID: "user"}}
	w2 := WorkspaceWithMeta{Workspace: Workspace{ID: "2", DNS: "main-def-org.brev.sh", Name: "main", CreatedByUserID: "user"}}
	ws := []WorkspaceWithMeta{w1, w2}

	// same id must be returned across time
	w1CorrectID := WorkspaceLocalID("main-abc")
	w2CorrectID := WorkspaceLocalID("main-def")

	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(WorkspacesFromWorkspaceWithMeta(ws)))
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier(WorkspacesFromWorkspaceWithMeta(ws)))

	// sometime later -- re-arranged
	ws = []WorkspaceWithMeta{w2, w1}
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(WorkspacesFromWorkspaceWithMeta(ws)))
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier(WorkspacesFromWorkspaceWithMeta(ws)))

	// sometime later -- w2 deleted
	ws = []WorkspaceWithMeta{w1}
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(WorkspacesFromWorkspaceWithMeta(ws)))

	// sometime later -- user changes name
	w1.Name = "new name"
	ws = []WorkspaceWithMeta{w1, w2}
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(WorkspacesFromWorkspaceWithMeta(ws)))
}
