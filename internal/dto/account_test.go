package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestToSiteInfo_LoginNotice(t *testing.T) {
	info := ToSiteInfo(&db.SystemSettings{
		SiteName:            "Sayrud",
		AllowSignUp:         true,
		AllowPasswordSignIn: true,
		LoginNotice:         "Scheduled maintenance\nPlease try again later",
	}, nil)
	require.Equal(t, "Scheduled maintenance\nPlease try again later", info.LoginNotice)
	require.Empty(t, ToSiteInfo(&db.SystemSettings{SiteName: "Sayrud", AllowPasswordSignIn: true}, nil).LoginNotice)
}
