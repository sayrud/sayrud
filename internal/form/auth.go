package form

type SignUp struct {
	Email    string `json:"email" valid:"required;email;maxlen:254"`
	UserName string `json:"userName" valid:"required;maxlen:32"`
	// Password is limited to 64 characters since bcrypt only uses the first 72 bytes.
	Password string `json:"password" valid:"required;minlen:8;maxlen:64"`
} // @name SignUp

type SignIn struct {
	Email    string `json:"email" valid:"required"`
	Password string `json:"password" valid:"required"`
} // @name SignIn

type UpdateProfile struct {
	UserName string `json:"userName" valid:"required;maxlen:32"`
} // @name UpdateProfile

type UpdateUserSettings struct {
	Theme    *string `json:"theme,omitempty" enums:"light,dark,system"`
	Language *string `json:"language,omitempty" enums:"zh-CN,en-US"`
} // @name UpdateUserSettings

type DeleteAccount struct {
	Password string `json:"password" valid:"required"`
} // @name DeleteAccount

type UpdatePassword struct {
	OldPassword string `json:"oldPassword" valid:"required"`
	NewPassword string `json:"newPassword" valid:"required;minlen:8;maxlen:64"`
} // @name UpdatePassword
