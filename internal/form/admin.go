package form

type AdminCreateUser struct {
	Email    string `json:"email" valid:"required;email;maxlen:254" label:"邮箱"`
	UserName string `json:"userName" valid:"required;maxlen:32" label:"用户名"`
	Password string `json:"password" valid:"required;minlen:8;maxlen:64" label:"密码"`
	IsAdmin  bool   `json:"isAdmin"`
} // @name AdminCreateUser

type AdminUpdateUser struct {
	UserName string `json:"userName" valid:"required;maxlen:32" label:"用户名"`
} // @name AdminUpdateUser

type AdminSetUserAdmin struct {
	IsAdmin bool `json:"isAdmin"`
} // @name AdminSetUserAdmin

type AdminSetUserStatus struct {
	Disabled bool `json:"disabled"`
} // @name AdminSetUserStatus

type AdminResetPassword struct {
	Password string `json:"password" valid:"required;minlen:8;maxlen:64" label:"新密码"`
} // @name AdminResetPassword

type AdminTransferProject struct {
	UserID int64 `json:"userID" valid:"required" label:"新所有者"`
} // @name AdminTransferProject

type UpdateSystemSettings struct {
	SiteName          string `json:"siteName" valid:"required" label:"站点名称"`
	AllowSignUp       bool   `json:"allowSignUp"`
	PasswordMinLength int    `json:"passwordMinLength"`
	SessionTTLDays    int    `json:"sessionTTLDays"`
} // @name UpdateSystemSettings
