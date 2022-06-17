package virtualproject

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/stretchr/testify/assert"
)

func TestNewVirtualProjectOne(t *testing.T) {
	ps := NewVirtualProjects([]entity.Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}})
	if !assert.Len(t, ps, 1) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 1, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 1)
}

func TestNewVirtualProjectNone(t *testing.T) {
	ps := NewVirtualProjects([]entity.Workspace{})
	assert.Len(t, ps, 0)
}

func TestNewVirtualProjectTwoDiffRepo(t *testing.T) {
	ps := NewVirtualProjects([]entity.Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}, {
		ID:              "2",
		Name:            "bye",
		GitRepo:         "git://bye",
		CreatedByUserID: "other",
	}})
	if !assert.Len(t, ps, 2) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 1, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 1)

	assert.Equal(t, ps[1].Name, "bye")
	assert.Equal(t, ps[1].GitURL, "git://bye")
	assert.Equal(t, 1, ps[1].GetUniqueUserCount())
	assert.Len(t, ps[1].GetUserWorkspaces("other"), 1)
}

func TestNewVirtualProjectTwoSameRepo(t *testing.T) {
	ps := NewVirtualProjects([]entity.Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}, {
		ID:              "2",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "other",
	}})
	if !assert.Len(t, ps, 1) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 2, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 1)
	assert.Len(t, ps[0].GetUserWorkspaces("other"), 1)
}

func TestNewVirtualProjectTwoSameRepoSameUser(t *testing.T) {
	ps := NewVirtualProjects([]entity.Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}, {
		ID:              "2",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}})
	if !assert.Len(t, ps, 1) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 1, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 2)
	assert.Len(t, ps[0].GetUserWorkspaces("other"), 0)
}
