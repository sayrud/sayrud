package form

import "time"

type AdminCreateUser struct {
	Email    string `json:"email" valid:"required;email;maxlen:254"`
	UserName string `json:"userName" valid:"required;maxlen:32"`
	Password string `json:"password" valid:"required;minlen:8;maxlen:64"`
	IsAdmin  bool   `json:"isAdmin"`
} // @name AdminCreateUser

type CreateBot struct {
	UserName string `json:"userName" valid:"required;maxlen:32"`
	// OwnerID is the member owning the bot, only taken from admins and defaults to the admin.
	OwnerID int64 `json:"ownerId,omitempty"`
	// Scopes are granted to the only access token of the bot, at least one.
	Scopes []string `json:"scopes" enums:"read,write,events"`
	// Projects are the UIDs of the projects the bot is restricted to, empty to follow its memberships.
	Projects []string `json:"projects"`
	// ExpiresAt is in the future, the token never expires if it is empty.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
} // @name CreateBot

type SetBotOwner struct {
	OwnerID int64 `json:"ownerId" valid:"required"`
} // @name SetBotOwner

type UpdateBotAccess struct {
	// Scopes are granted to the only access token of the bot, at least one.
	Scopes []string `json:"scopes" enums:"read,write,events"`
	// Projects are the UIDs of the projects the bot is restricted to, empty to follow its memberships.
	Projects []string `json:"projects"`
	// ExpiresAt is in the future, the token never expires if it is empty.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
} // @name UpdateBotAccess

type UpdateBot struct {
	UserName string `json:"userName" valid:"required;maxlen:32"`
} // @name UpdateBot

type SetBotStatus struct {
	Disabled bool `json:"disabled"`
} // @name SetBotStatus

type AdminUpdateUser struct {
	UserName string `json:"userName" valid:"required;maxlen:32"`
} // @name AdminUpdateUser

type AdminSetUserAdmin struct {
	IsAdmin bool `json:"isAdmin"`
} // @name AdminSetUserAdmin

type AdminSetUserStatus struct {
	Disabled bool `json:"disabled"`
} // @name AdminSetUserStatus

type AdminResetPassword struct {
	Password string `json:"password" valid:"required;minlen:8;maxlen:64"`
} // @name AdminResetPassword

type AdminTransferProject struct {
	UserID int64 `json:"userID" valid:"required"`
} // @name AdminTransferProject

type UpdateSystemSettings struct {
	SiteName          string `json:"siteName" valid:"required"`
	AllowSignUp       bool   `json:"allowSignUp"`
	PasswordMinLength int    `json:"passwordMinLength"`
	SessionTTLDays    int    `json:"sessionTTLDays"`
	// ExternalURL is the external URL of the site, empty if not set.
	ExternalURL         string `json:"externalURL"`
	AllowPasswordSignIn bool   `json:"allowPasswordSignIn"`
	// LoginNotice is the plain text shown above the sign-in form, empty if hidden.
	LoginNotice string `json:"loginNotice"`
	// NetworkAllowlist permits shortcut fetches to these otherwise blocked IP addresses or CIDRs.
	NetworkAllowlist []string `json:"networkAllowlist"`
	// AllowMemberBots reports whether the members other than admins can create their own bots.
	AllowMemberBots bool `json:"allowMemberBots"`
	// MemberBotLimit is the maximum number of the own bots of each member other than admins.
	MemberBotLimit int `json:"memberBotLimit"`
} // @name UpdateSystemSettings
