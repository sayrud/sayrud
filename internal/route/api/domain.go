// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
)

var Domain domainRoute

type domainRoute struct{}

func (domainRoute) List(ctx context.Context, project *db.Project) error {
	projectID := project.ID

	domains, err := db.Domains.ListByProjectID(ctx.Request().Context(), projectID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list domains")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(domains)
}

func (domainRoute) Create(ctx context.Context, project *db.Project, f form.CreateDomain) error {
	domain := f.Domain

	// TODO: Check the domain tld.

	if err := db.Domains.Create(ctx.Request().Context(), db.CreateDomainOptions{
		ProjectID: project.ID,
		Domain:    domain,
	}); err != nil {
		if errors.Is(err, db.ErrDomainAlreadyExists) {
			return ctx.ApiError(http.StatusBadRequest, "域名已存在")
		}

		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create domain")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

func (domainRoute) Domainer(ctx context.Context, project *db.Project) error {
	domain, err := db.Domains.GetByDomain(ctx.Request().Context(), ctx.Param("domain"))
	if err != nil {
		if errors.Is(err, db.ErrDomainNotFound) {
			return ctx.ApiError(http.StatusNotFound, "域名不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get domain by domain")
		return ctx.ApiServerError()
	}

	if domain.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "域名不存在")
	}

	ctx.Map(domain)
	return nil
}

func (domainRoute) Delete(ctx context.Context, domain *db.Domain) error {
	if err := db.Domains.DeleteByID(ctx.Request().Context(), domain.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete domain")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

func (domainRoute) Verify(ctx context.Context, domain *db.Domain) error {
	if domain.Verified {
		return ctx.ApiSuccess("域名已验证，无需重复验证")
	}

	// Check the domain's CNAME point to the correct value.
	cname, err := net.LookupCNAME(domain.Domain)
	if err != nil {
		note := "查询域名 CNAME 解析失败"
		if err := db.Domains.UpdateVerified(ctx.Request().Context(), domain.ID, false, note); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update domain verified")
		}
		return ctx.ApiError(http.StatusBadRequest, note)
	}

	if cname != os.Getenv("CNAME_DOMAIN") {
		note := fmt.Sprintf("域名 CNAME 未解析到 %s，当前解析：%s", os.Getenv("CNAME_DOMAIN"), cname)
		if err := db.Domains.UpdateVerified(ctx.Request().Context(), domain.ID, false, note); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update domain verified")
		}
		return ctx.ApiError(http.StatusBadRequest, note)
	}

	note := "域名验证成功"
	if err := db.Domains.UpdateVerified(ctx.Request().Context(), domain.ID, true, note); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update domain verified")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(note)
}

func (domainRoute) createDomainIngressIfNotExists(ctx context.Context, domain string) error {
	return nil
}

func (domainRoute) deleteDomainIngressIfExists(ctx context.Context, domain string) error {
	return nil
}
