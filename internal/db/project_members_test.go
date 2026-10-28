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

func TestEffectiveRole(t *testing.T) {
	for _, tc := range []struct {
		role    ProjectRole
		isAdmin bool
		want    ProjectRole
	}{
		{"", false, ""},
		{ProjectRoleViewer, false, ProjectRoleViewer},
		{ProjectRoleManager, false, ProjectRoleManager},
		{"", true, ProjectRoleManager},
		{ProjectRoleViewer, true, ProjectRoleManager},
		{ProjectRoleEditor, true, ProjectRoleManager},
		{ProjectRoleManager, true, ProjectRoleManager},
		{ProjectRoleOwner, true, ProjectRoleOwner},
	} {
		require.Equal(t, tc.want, EffectiveRole(tc.role, tc.isAdmin), "role %q, admin %v", tc.role, tc.isAdmin)
	}
}

func TestProjectRoleIsMemberRole(t *testing.T) {
	require.True(t, ProjectRoleManager.IsMemberRole())
	require.True(t, ProjectRoleEditor.IsMemberRole())
	require.True(t, ProjectRoleViewer.IsMemberRole())
	require.False(t, ProjectRoleOwner.IsMemberRole())
	require.False(t, ProjectRole("").IsMemberRole())
}
