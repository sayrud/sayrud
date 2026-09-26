package dto

import (
	"github.com/wuhan005/sayrud/internal/db"
)

// Profile is the profile of the signed-in user.
type Profile struct {
	Email    string `json:"email"`
	EmailMd5 string `json:"emailMd5"`
	UserName string `json:"userName"`
} // @name Profile

func ToProfile(user *db.User) *Profile {
	return &Profile{
		Email:    user.Email,
		EmailMd5: user.EmailMd5,
		UserName: user.UserName,
	}
}
