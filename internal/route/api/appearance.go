package api

import (
	"regexp"
	"strings"
)

var appearanceIconPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// validAppearanceUpdate checks the optional name, icon identifier and palette color.
func validAppearanceUpdate(name, icon, color *string) bool {
	if name != nil && strings.TrimSpace(*name) == "" {
		return false
	}

	if icon != nil && *icon != "" && (len(*icon) > 64 || !appearanceIconPattern.MatchString(*icon)) {
		return false
	}

	if color != nil {
		switch *color {
		case "", "orange", "coral", "pink", "purple", "indigo", "blue", "teal", "green":
		default:
			return false
		}
	}

	return true
}
