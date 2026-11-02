package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// AdminUser is a user in the member list of the admin console.
type AdminUser struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	EmailMd5  string `json:"emailMd5"`
	UserName  string `json:"userName"`
	AvatarURL string `json:"avatarUrl"`
	Color     string `json:"color"`
	IsAdmin   bool   `json:"isAdmin"`
	IsBot     bool   `json:"isBot"`
	// OwnerID is the member owning the bot, 0 for the humans.
	OwnerID           int64 `json:"ownerId"`
	Disabled          bool  `json:"disabled"`
	OwnedProjectCount int64 `json:"ownedProjectCount"`
	// OwnedBotCount is the number of its bots, which are deleted along with it.
	OwnedBotCount int64      `json:"ownedBotCount"`
	LastSignInAt  *time.Time `json:"lastSignInAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	// Providers are the sign-in methods bound by the user.
	Providers []*AuthProviderBrief `json:"providers"`
} // @name AdminUser

func ToAdminUser(user *db.User, ownedProjectCount, ownedBotCount int64, providers []*AuthProviderBrief) *AdminUser {
	if providers == nil {
		providers = []*AuthProviderBrief{}
	}
	return &AdminUser{
		Providers:         providers,
		ID:                user.ID,
		Email:             user.Email,
		EmailMd5:          user.EmailMd5,
		UserName:          user.UserName,
		AvatarURL:         UserAvatarURL(user),
		Color:             UserColor(user.ID),
		IsAdmin:           user.IsAdmin,
		IsBot:             user.IsBot(),
		OwnerID:           user.OwnerID,
		Disabled:          user.Disabled(),
		OwnedProjectCount: ownedProjectCount,
		OwnedBotCount:     ownedBotCount,
		LastSignInAt:      user.LastSignInAt,
		CreatedAt:         user.CreatedAt,
	}
}

type ListAdminUsersResp struct {
	Users []*AdminUser `json:"users"`
	Total int64        `json:"total"`
} // @name ListAdminUsersResp

// Bot is a bot in the bot lists of the admin console and the account settings.
type Bot struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	UserName  string `json:"userName"`
	AvatarURL string `json:"avatarUrl"`
	Color     string `json:"color"`
	Disabled  bool   `json:"disabled"`
	// Owner is the member owning the bot, whose access the bot never exceeds.
	Owner *UserBrief `json:"owner"`
	// Token is the only access token of the bot without its secret, nil if it has none.
	Token     *AccessToken `json:"token"`
	CreatedAt time.Time    `json:"createdAt"`
} // @name Bot

func ToBot(user, owner *db.User, token *AccessToken) *Bot {
	bot := &Bot{
		ID:        user.ID,
		Email:     user.Email,
		UserName:  user.UserName,
		AvatarURL: UserAvatarURL(user),
		Color:     UserColor(user.ID),
		Disabled:  user.Disabled(),
		Token:     token,
		CreatedAt: user.CreatedAt,
	}
	if owner != nil {
		bot.Owner = ToUserBrief(owner)
	}
	return bot
}

// CreateBotResp returns the new bot and the secret of its access token.
type CreateBotResp struct {
	Bot *Bot `json:"bot"`
	// Secret is the plaintext of the token, it can not be retrieved again.
	Secret string `json:"secret"`
} // @name CreateBotResp

type ListAdminBotsResp struct {
	Bots  []*Bot `json:"bots"`
	Total int64  `json:"total"`
} // @name ListAdminBotsResp

// ListMyBotsResp lists the bots of the signed-in member.
type ListMyBotsResp struct {
	Bots []*Bot `json:"bots"`
	// Limit is the maximum number of the own bots, 0 if not limited for admins.
	Limit int `json:"limit"`
	// CanCreate reports whether the member may create another bot now.
	CanCreate bool `json:"canCreate"`
} // @name ListMyBotsResp

// AdminProject is a project in the project list of the admin console.
type AdminProject struct {
	UID   string     `json:"uid"`
	Name  string     `json:"name"`
	Icon  string     `json:"icon"`
	Color string     `json:"color"`
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
	BuildVersion string             `json:"buildVersion"`
	BuildCommit  string             `json:"buildCommit"`
	GoVersion    string             `json:"goVersion"`
	Settings     *db.SystemSettings `json:"settings"`
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
