package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLocalIdentifier(t *testing.T) {
	w := Workspace{DNS: "test-rand-org.brev.sh"}
	assert.Equal(t, WorkspaceLocalID("test-rand"), w.GetLocalIdentifier(nil))
}

func TestGetLocalIdentifierClean(t *testing.T) {
	// safest https://www.saveonhosting.com/scripts/index.php?rp=/knowledgebase/52/What-are-the-valid-characters-for-a-domain-name-and-how-long-can-a-domain-name-be.html
	w := Workspace{DNS: "test-rand-org.brev.sh", Name: "abc/def"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))

	w = Workspace{DNS: "test-rand-org.brev.sh", Name: "'abc"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))

	w = Workspace{DNS: "test-rand-org.brev.sh", Name: "\"abc"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))

	w = Workspace{DNS: "test-rand-org.brev.sh", Name: "\\abc"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))

	w = Workspace{DNS: "test-rand-org.brev.sh", Name: "/abc"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))

	w = Workspace{DNS: "test-rand-org.brev.sh", Name: ".abc"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))

	w = Workspace{DNS: "test-rand-org.brev.sh", Name: ":abc"}
	assert.Equal(t, WorkspaceLocalID("abc"), w.GetLocalIdentifier(nil))
}

func TestGetLocalIdentifierDeterminism(t *testing.T) {
	w1 := WorkspaceWithMeta{Workspace: Workspace{ID: "1", DNS: "main-abc-org.brev.sh", Name: "main", CreatedByUserID: "user"}}
	w2 := WorkspaceWithMeta{Workspace: Workspace{ID: "2", DNS: "main-def-org.brev.sh", Name: "main", CreatedByUserID: "user"}}
	ws := []WorkspaceWithMeta{w1, w2}

	// same id must be returned across time
	w1CorrectID := WorkspaceLocalID("main-abc")
	w2CorrectID := WorkspaceLocalID("main-def")

	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(ws))
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier(ws))

	// sometime later -- re-arranged
	ws = []WorkspaceWithMeta{w2, w1}
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(ws))
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier(ws))

	// sometime later -- w2 deleted
	ws = []WorkspaceWithMeta{w1}
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(ws))

	// // sometime later -- user changes name
	// w1.Name = "new name"
	// ws = []WorkspaceWithMeta{w1, w2}
	// assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier(ws))
}
