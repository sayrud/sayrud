package form

import (
	"mime/multipart"
	"time"
)

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

type UploadAvatar struct {
	File *multipart.FileHeader `form:"file" validate:"required"`
} // @name UploadAvatar

type UpdateUserSettings struct {
	Theme    *string `json:"theme,omitempty" enums:"light,dark,system"`
	Language *string `json:"language,omitempty" enums:"en,zh-CN,zh-TW,ja,ko,es,pt-BR,fr,de,ru"`
} // @name UpdateUserSettings

type DeleteAccount struct {
	// Password is the current password of users with one.
	Password string `json:"password,omitempty"`
	// ConfirmEmail is the own email entered by users without a password.
	ConfirmEmail string `json:"confirmEmail,omitempty"`
} // @name DeleteAccount

type LDAPSignIn struct {
	UserName string `json:"username" valid:"required;maxlen:254"`
	Password string `json:"password" valid:"required;maxlen:256"`
} // @name LDAPSignIn

type CreateAccessToken struct {
	Name string `json:"name" valid:"required;maxlen:64"`
	// Scopes are granted to the token, at least one.
	Scopes []string `json:"scopes" enums:"read,write,events"`
	// Projects are the UIDs of the projects the token is restricted to, empty to follow the memberships of the user.
	Projects []string `json:"projects"`
	// ExpiresAt is in the future, the token never expires if it is empty.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
} // @name CreateAccessToken

type UpdatePassword struct {
	OldPassword string `json:"oldPassword" valid:"required"`
	NewPassword string `json:"newPassword" valid:"required;minlen:8;maxlen:64"`
} // @name UpdatePassword
