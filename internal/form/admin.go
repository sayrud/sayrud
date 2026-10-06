package form

type AdminCreateUser struct {
	Email    string `json:"email" valid:"required;email;maxlen:254"`
	UserName string `json:"userName" valid:"required;maxlen:32"`
	Password string `json:"password" valid:"required;minlen:8;maxlen:64"`
	IsAdmin  bool   `json:"isAdmin"`
} // @name AdminCreateUser

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
} // @name UpdateSystemSettings
