package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLocalIdentifier(t *testing.T) {
	w := Workspace{DNS: "test-rand-org.brev.sh"}
	assert.Equal(t, WorkspaceLocalID("test-rand"), w.GetLocalIdentifier())
}

func TestGetLocalIdentifierClean(t *testing.T) {
	// safest https://www.saveonhosting.com/scripts/index.php?rp=/knowledgebase/52/What-are-the-valid-characters-for-a-domain-name-and-how-long-can-a-domain-name-be.html
	correctID := WorkspaceLocalID("test-rand")
	w := Workspace{DNS: "test-rand-org.brev.sh", Name: "abc/def"}
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = "'abc"
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = "\"abc"
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = "\\abc"
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = "/abc"
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = ".abc"
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = ":abc"
	assert.Equal(t, correctID, w.GetLocalIdentifier())
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

	// same id must be returned across time
	w1CorrectID := WorkspaceLocalID("main-abc")
	w2CorrectID := WorkspaceLocalID("main-def")

	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier())

	// sometime later -- re-arranged
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier())

	// sometime later -- w2 deleted
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())

	// sometime later -- user changes name
	w1.Name = "new name"
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())
}
