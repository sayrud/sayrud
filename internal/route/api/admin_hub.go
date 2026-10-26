package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/appconst"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/hub"
	"github.com/wuhan005/sayrud/internal/i18n"
)

// downloadFromHub downloads and verifies a release of Sayrud Hub, errors to show are *i18n.Error.
func downloadFromHub(ctx context.Context, kind, id, version string, maxSize int) (*hub.Package, error) {
	p, err := hub.Download(ctx.Request().Context(), kind, id, version, requestLang(ctx), maxSize)
	if err == nil {
		return p, nil
	}
	var verifyErr *hub.VerifyError
	var hubErr *hub.Error
	switch {
	case errors.Is(err, hub.ErrNotFound):
		return nil, i18n.Errorf("hub::not_found")
	case errors.As(err, &verifyErr):
		return nil, i18n.Errorf("hub::invalid_signature", verifyErr.Error())
	case errors.As(err, &hubErr):
		return nil, i18n.Errorf("hub::unavailable", hubErr.Message)
	}
	return nil, err
}

// hubRelease returns the release of Sayrud Hub to inspect: the latest one of hubID, or the one a Hub source URL
// downloads. It returns nil for the other sources, which are downloaded as usual.
func hubRelease(ctx context.Context, kind, hubID, sourceURL string, maxSize int) (*hub.Package, error) {
	if id := strings.TrimSpace(hubID); id != "" {
		return downloadFromHub(ctx, kind, id, hub.LatestVersion, maxSize)
	}
	urlKind, id, version, ok := hub.ParseSourceURL(sourceURL)
	if !ok {
		return nil, nil
	}
	if urlKind != kind {
		return nil, i18n.Errorf("hub::wrong_kind")
	}
	return downloadFromHub(ctx, kind, id, version, maxSize)
}

// hubSignature returns the Sayrud Hub signature to store with the saved source: the given one must describe it,
// otherwise the stored one is kept while it still does, so that editing the source detaches it from the Hub.
func hubSignature(given json.RawMessage, stored string, e hub.Expect, source string) (string, error) {
	if len(given) > 0 && string(given) != "null" {
		var compact bytes.Buffer
		if err := json.Compact(&compact, given); err != nil {
			return "", i18n.Errorf("hub::invalid_signature", err.Error())
		}
		if _, err := hub.VerifyRelease(compact.String(), e, source); err != nil {
			return "", i18n.Errorf("hub::invalid_signature", err.Error())
		}
		return compact.String(), nil
	}
	if stored != "" {
		if _, err := hub.VerifyRelease(stored, e, source); err == nil {
			return stored, nil
		}
	}
	return "", nil
}

// inspectedRelease describes the release being inspected, with its changelog.
func inspectedRelease(p *hub.Package) *dto.HubOrigin {
	origin := dto.ToHubOrigin(p.Signature)
	if origin != nil {
		origin.Changelog = p.Release.Changelog
	}
	return origin
}

// appVersion is the Sayrud version sent to the Hub, without the "v" prefix of the tags.
func appVersion() string {
	return strings.TrimPrefix(appconst.BuildVersion, "v")
}

// CheckHubUpdates
// @Summary Check the packages installed from Sayrud Hub
// @Description Requires the admin. Only the packages installed from the Hub are sent to it, the result is reused for an hour unless refresh is true.
// @Description The packages without an update, a yank or a delisting are omitted.
// @Produce json
// @Param refresh query bool false "Ask the Hub again instead of reusing the last result"
// @Success 200 {object} dto.HubUpdatesResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @Failure 502 {string} string "Sayrud Hub can not be reached"
// @ID checkAdminHubUpdates
// @Router /admin/hub/updates [get]
func (adminRoute) CheckHubUpdates(ctx context.Context) error {
	c := ctx.Request().Context()
	plugins, err := db.WorkflowPlugins.List(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list workflow plugins")
		return ctx.ApiServerError()
	}
	shortcuts, err := db.FieldShortcuts.List(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list field shortcuts")
		return ctx.ApiServerError()
	}

	installed := []hub.Installed{}
	signatures := make([]string, 0, len(plugins)+len(shortcuts))
	for _, p := range plugins {
		signatures = append(signatures, p.HubSignature)
	}
	for _, s := range shortcuts {
		signatures = append(signatures, s.HubSignature)
	}
	for _, signature := range signatures {
		if st := hub.Origin(signature); st != nil {
			installed = append(installed, hub.Installed{Kind: st.Kind, ID: st.ID, Version: st.Version})
		}
	}

	updates, err := hub.CheckUpdates(c, appVersion(), installed, ctx.Query("refresh") == "true")
	if err != nil {
		var hubErr *hub.Error
		if errors.As(err, &hubErr) {
			return ctx.ApiError(http.StatusBadGateway, "hub::unavailable", hubErr.Message)
		}
		logrus.WithContext(c).WithError(err).Error("Failed to check Sayrud Hub updates")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToHubUpdatesResp(updates))
}
