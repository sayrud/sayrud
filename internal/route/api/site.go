package api

import (
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/dto"
)

var Site siteRoute

type siteRoute struct{}

// Get
// @Summary Get the site information
// @Description Public information used by the sign-in page, such as whether signing up is allowed.
// @Produce json
// @Success 200 {object} dto.SiteInfo
// @Failure 500 {string} string "Internal server error"
// @ID getSiteInfo
// @Router /site [get]
func (siteRoute) Get(ctx context.Context) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	return ctx.ApiSuccess(dto.ToSiteInfo(settings))
}
