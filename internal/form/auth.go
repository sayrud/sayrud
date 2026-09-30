package form

type SignUp struct {
	Email    string `json:"email" valid:"required;email;maxlen:254" label:"邮箱"`
	UserName string `json:"userName" valid:"required;maxlen:32" label:"用户名"`
	// Password is limited to 64 characters since bcrypt only uses the first 72 bytes.
	Password string `json:"password" valid:"required;minlen:8;maxlen:64" label:"密码"`
} // @name SignUp

type SignIn struct {
	Email    string `json:"email" valid:"required" label:"邮箱"`
	Password string `json:"password" valid:"required" label:"密码"`
} // @name SignIn

type UpdateProfile struct {
	UserName string `json:"userName" valid:"required;maxlen:32" label:"用户名"`
} // @name UpdateProfile

type UpdatePassword struct {
	OldPassword string `json:"oldPassword" valid:"required" label:"当前密码"`
	NewPassword string `json:"newPassword" valid:"required;minlen:8;maxlen:64" label:"新密码"`
} // @name UpdatePassword
