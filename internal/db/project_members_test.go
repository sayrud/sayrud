package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectRoleAtLeast(t *testing.T) {
	roles := []ProjectRole{ProjectRoleViewer, ProjectRoleEditor, ProjectRoleManager, ProjectRoleOwner}
	for i, role := range roles {
		for j, min := range roles {
			require.Equal(t, i >= j, role.AtLeast(min), "%s at least %s", role, min)
		}
	}

	require.False(t, ProjectRole("").AtLeast(ProjectRoleViewer))
	require.False(t, ProjectRole("admin").AtLeast(ProjectRoleViewer))
}

func TestProjectRoleIsMemberRole(t *testing.T) {
	require.True(t, ProjectRoleManager.IsMemberRole())
	require.True(t, ProjectRoleEditor.IsMemberRole())
	require.True(t, ProjectRoleViewer.IsMemberRole())
	require.False(t, ProjectRoleOwner.IsMemberRole())
	require.False(t, ProjectRole("").IsMemberRole())
}
