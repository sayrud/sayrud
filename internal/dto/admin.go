package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// AdminUser is a user in the member list of the admin console.
type AdminUser struct {
	ID                int64      `json:"id"`
	Email             string     `json:"email"`
	EmailMd5          string     `json:"emailMd5"`
	UserName          string     `json:"userName"`
	Color             string     `json:"color"`
	IsAdmin           bool       `json:"isAdmin"`
	Disabled          bool       `json:"disabled"`
	OwnedProjectCount int64      `json:"ownedProjectCount"`
	LastSignInAt      *time.Time `json:"lastSignInAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	// Providers are the sign-in methods bound by the user.
	Providers []*AuthProviderBrief `json:"providers"`
} // @name AdminUser

func ToAdminUser(user *db.User, ownedProjectCount int64, providers []*AuthProviderBrief) *AdminUser {
	if providers == nil {
		providers = []*AuthProviderBrief{}
	}
	return &AdminUser{
		Providers:         providers,
		ID:                user.ID,
		Email:             user.Email,
		EmailMd5:          user.EmailMd5,
		UserName:          user.UserName,
		Color:             UserColor(user.ID),
		IsAdmin:           user.IsAdmin,
		Disabled:          user.Disabled(),
		OwnedProjectCount: ownedProjectCount,
		LastSignInAt:      user.LastSignInAt,
		CreatedAt:         user.CreatedAt,
	}
}

type ListAdminUsersResp struct {
	Users []*AdminUser `json:"users"`
	Total int64        `json:"total"`
} // @name ListAdminUsersResp

// AdminProject is a project in the project list of the admin console.
type AdminProject struct {
	UID   string     `json:"uid"`
	Name  string     `json:"name"`
	Owner *UserBrief `json:"owner"`
	// MemberCount is the number of collaborators, excluding the owner.
	MemberCount int64     `json:"memberCount"`
	TableCount  int64     `json:"tableCount"`
	CreatedAt   time.Time `json:"createdAt"`
} // @name AdminProject

type ListAdminProjectsResp struct {
	Projects []*AdminProject `json:"projects"`
	Total    int64           `json:"total"`
} // @name ListAdminProjectsResp

type AdminUserStats struct {
	Total        int64 `json:"total"`
	Disabled     int64 `json:"disabled"`
	Admins       int64 `json:"admins"`
	NewLast7Days int64 `json:"newLast7Days"`
} // @name AdminUserStats

type AdminSystemInfo struct {
	BuildCommit string             `json:"buildCommit"`
	GoVersion   string             `json:"goVersion"`
	Settings    *db.SystemSettings `json:"settings"`
} // @name AdminSystemInfo

// AdminOverview is the overview of the admin console.
type AdminOverview struct {
	Users          AdminUserStats  `json:"users"`
	Projects       int64           `json:"projects"`
	Tables         int64           `json:"tables"`
	Records        int64           `json:"records"`
	ActiveSessions int64           `json:"activeSessions"`
	RecentUsers    []*AdminUser    `json:"recentUsers"`
	System         AdminSystemInfo `json:"system"`
} // @name AdminOverview
