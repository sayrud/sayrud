package dto

import (
	"strconv"
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Profile is the profile of the signed-in user.
type Profile struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	EmailMd5 string `json:"emailMd5"`
	UserName string `json:"userName"`
	// Color is the avatar color of the user.
	Color string `json:"color"`
	// IsAdmin reports whether the user is a system admin.
	IsAdmin   bool      `json:"isAdmin"`
	CreatedAt time.Time `json:"createdAt"`
} // @name Profile

func ToProfile(user *db.User) *Profile {
	return &Profile{
		ID:        user.ID,
		Email:     user.Email,
		EmailMd5:  user.EmailMd5,
		UserName:  user.UserName,
		Color:     UserColor(user.ID),
		IsAdmin:   user.IsAdmin,
		CreatedAt: user.CreatedAt,
	}
}

// UserBrief is the public information of a user shown to the collaborators.
type UserBrief struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	EmailMd5 string `json:"emailMd5"`
	UserName string `json:"userName"`
	Color    string `json:"color"`
} // @name UserBrief

func ToUserBrief(user *db.User) *UserBrief {
	return &UserBrief{
		ID:       user.ID,
		Email:    user.Email,
		EmailMd5: user.EmailMd5,
		UserName: user.UserName,
		Color:    UserColor(user.ID),
	}
}

var userColors = []string{"#3370ff", "#f54a45", "#ff8800", "#14c0a7", "#7f3bf5", "#f5319d", "#00b2d6", "#8fac02"}

// UserColor returns the stable avatar color of the user.
func UserColor(userID int64) string {
	return userColors[userID%int64(len(userColors))]
}

// MemberID returns the collaborator identity of the user in the WebSocket presence.
func MemberID(userID int64) string {
	return "usr" + strconv.FormatInt(userID, 10)
}
