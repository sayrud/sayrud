// Package manifest holds the rules shared by the manifests of the workflow node plugins and the field shortcuts,
// so that the same property is named, typed and validated the same way in both.
package manifest

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/wuhan005/sayrud/internal/i18n"
)

// Error is an invalid manifest; Message is shown to the admin as is.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// Errorf returns an *Error with the formatted message.
func Errorf(format string, args ...interface{}) error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

// Limits shared by the manifests.
const (
	MaxSections     = 20
	MaxFields       = 60
	MaxOptions      = 100
	MaxText         = 500
	MaxHosts        = 50
	MaxTranslations = 20
)

var (
	idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)
	// VersionPattern is the version of a manifest, e.g. 1.0.0.
	VersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+\-]{0,31}$`)
	// KeyPattern is the key of a form field, an output or a credential.
	KeyPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	langPattern = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
)

// CheckID checks the source package identifier, independently of a shortcut installation's UID.
func CheckID(id string) error {
	if !idPattern.MatchString(id) {
		return Errorf("id must be 2–40 lowercase letters, digits or underscores starting with a letter")
	}
	return nil
}

// CheckHomepage checks the optional project website shared by both kinds of modules.
func CheckHomepage(homepage string) error {
	if homepage == "" {
		return nil
	}
	u, err := url.Parse(homepage)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || len(homepage) > 500 {
		return Errorf("homepage must be an http or https URL")
	}
	return nil
}

// CheckVersion checks the required version.
func CheckVersion(version string) error {
	if !VersionPattern.MatchString(version) {
		return Errorf("version is required, e.g. 1.0.0")
	}
	return nil
}

// CheckText checks the translations of the text named what, the English one must not be blank if required.
func CheckText(t i18n.LocalText, what string, required bool) error {
	if required && strings.TrimSpace(t.In("en")) == "" {
		return Errorf("%s is required", what)
	}
	if len(t) > MaxTranslations {
		return Errorf("%s has too many translations", what)
	}
	for lang, s := range t {
		if lang != "" && !langPattern.MatchString(lang) {
			return Errorf("%s has an invalid language %q", what, lang)
		}
		if utf8.RuneCountInString(s) > MaxText {
			return Errorf("%s is longer than %d characters", what, MaxText)
		}
	}
	return nil
}

// CheckKey checks the key of the field named what, and records it in keys to report the duplicated ones.
func CheckKey(keys map[string]bool, key, what string) error {
	switch {
	case !KeyPattern.MatchString(key):
		return Errorf("%s %q has an invalid key", what, key)
	case keys[key]:
		return Errorf("%s %q is duplicated", what, key)
	}
	keys[key] = true
	return nil
}

// Timeout returns the timeout in seconds, def if it is 0.
func Timeout(seconds, def, max int) (int, error) {
	if seconds == 0 {
		return def, nil
	}
	if seconds < 1 || seconds > max {
		return 0, Errorf("timeout must be 1–%d seconds", max)
	}
	return seconds, nil
}

// NormalizeHosts checks the hosts allowed to fetch: host names or IP addresses including their subdomains, *.host for the same, or * for any public host.
// The hosts are returned lowercase, sorted and without duplicates.
func NormalizeHosts(hosts []string) ([]string, error) {
	if len(hosts) > MaxHosts {
		return nil, Errorf("permissions.fetch has more than %d hosts", MaxHosts)
	}
	normalized := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h == "*" {
			normalized = append(normalized, h)
			continue
		}
		d, ok := NormalizeHost(strings.TrimPrefix(h, "*."))
		if !ok {
			return nil, Errorf("permissions.fetch has an invalid host %q", h)
		}
		normalized = append(normalized, d)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

var hostPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizeHost returns the lowercase host name without the trailing dot, ok is false if it is not a host name or an IP address.
func NormalizeHost(host string) (string, bool) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if d == "" || len(d) > 253 {
		return "", false
	}
	if _, err := netip.ParseAddr(d); err == nil {
		return d, true
	}
	return d, hostPattern.MatchString(d)
}

// HostAllowed reports whether the host equals one of the allowed hosts or is a subdomain of it; * allows any host.
func HostAllowed(host string, hosts []string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	for _, d := range hosts {
		if d == "*" || host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
