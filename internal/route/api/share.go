package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/sso"
)

var Share shareRoute

type shareRoute struct{}

// publicShare identifies the root table without conflicting with the requested table mapping.
type publicShare struct {
	Table *db.SLTable
}

func (shareRoute) LimitBody(ctx context.Context) {
	r := ctx.Request().Request
	r.Body = http.MaxBytesReader(ctx.ResponseWriter(), r.Body, 4096)
}

// Get
// @Summary Get link sharing settings
// @Description Requires a project manager. The saved link password is decrypted only for this settings response.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 200 {object} dto.LinkShare
// @ID getLinkShare
// @Router /projects/{projectUID}/tables/{tableUID}/share [get]
func (shareRoute) Get(ctx context.Context, project *db.Project, table *db.SLTable) error {
	ctx.ResponseWriter().Header().Set("Cache-Control", "no-store")
	settings := dto.ToLinkShare(project, table)
	if settings.Enabled && settings.PasswordEnabled && table.SharePasswordSealed != "" {
		password, err := sso.Open(table.SharePasswordSealed)
		if err != nil {
			// Keep the settings editable if the encryption key changed or the ciphertext is damaged.
			logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to decrypt link sharing password")
		} else {
			settings.Password = string(password)
		}
	}

	return ctx.ApiSuccess(settings)
}

// Update
// @Summary Update read-only link sharing settings
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.UpdateLinkShare true "Sharing settings"
// @Success 200 {object} dto.LinkShare
// @ID updateLinkShare
// @Router /projects/{projectUID}/tables/{tableUID}/share [put]
func (shareRoute) Update(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, f form.UpdateLinkShare) error {
	hash := table.SharePasswordHash
	sealed := table.SharePasswordSealed
	if f.Password != nil {
		hash, sealed = "", ""
		if *f.Password != "" {
			if !validSharePassword(*f.Password) {
				return ctx.ApiError(http.StatusBadRequest, "share::invalid_password")
			}

			var err error
			sealed, err = sso.Seal([]byte(*f.Password))
			if err != nil {
				if errors.Is(err, sso.ErrNoSecretKey) {
					return ctx.ApiError(http.StatusBadRequest, "sso::secret_key_missing")
				}
				logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to encrypt link sharing password")
				return ctx.ApiServerError()
			}

			raw, err := bcrypt.GenerateFromPassword([]byte(*f.Password), bcrypt.DefaultCost)
			if err != nil {
				return ctx.ApiServerError()
			}
			hash = string(raw)
		}
	}

	token := table.ShareToken
	if !f.Enabled {
		// Closing a share invalidates its token and all password grants.
		token, hash, sealed = "", "", ""
	} else if token == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return ctx.ApiServerError()
		}
		token = hex.EncodeToString(raw)
	}

	if err := db.SLTables.Update(ctx.Request().Context(), table.ID, db.UpdateSLTableOptions{
		ShareToken: &token, ShareEnabled: &f.Enabled,
		ShareIncludeChildren: &f.IncludeChildren, SharePasswordHash: &hash,
		SharePasswordSealed: &sealed,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update link share")
		return ctx.ApiServerError()
	}

	table.ShareToken, table.SharePasswordHash = token, hash
	table.SharePasswordSealed = sealed
	table.ShareEnabled, table.ShareIncludeChildren = f.Enabled, f.IncludeChildren

	hub.NotifyShareChanged(project.UID, table.ID)

	return Share.Get(ctx, project, table)
}

func validSharePassword(password string) bool {
	if len(password) < 8 || len(password) > 18 {
		return false
	}

	var digit, letter, symbol bool
	for _, char := range password {
		switch {
		case char < '!' || char > '~':
			return false
		case char >= '0' && char <= '9':
			digit = true
		case char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z':
			letter = true
		default:
			symbol = true
		}
	}

	return digit && letter || digit && symbol || letter && symbol
}

// Resolve
// @Summary Resolve public access from the normal project and table URL
// @Description Returns only an enabled share token. Password verification and read scope remain enforced on all share APIs.
// @Produce json
// @Param projectUID query string true "Project UID"
// @Param tableUID query string false "Table UID; omitted to open the first shared table"
// @Success 200 {object} dto.ResolvedLinkShare
// @Failure 404 {string} string "Share unavailable"
// @ID resolveLinkShare
// @Router /shares/resolve [get]
func (shareRoute) Resolve(ctx context.Context) error {
	ctx.ResponseWriter().Header().Set("Cache-Control", "no-store")
	project, err := db.Projects.GetByUID(ctx.Request().Context(), ctx.Query("projectUID"))
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "share::unavailable")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to resolve shared project")
		return ctx.ApiServerError()
	}

	tableUID := ctx.Query("tableUID")
	exists := tableUID == ""
	var inherited *db.SLTable
	seen := 0
	for page := 1; ; page++ {
		batch, total, err := db.SLTables.Query(ctx.Request().Context(), db.QuerySLTableOptions{
			ProjectID: project.ID, Pagination: dbutil.Pagination{Page: page, PageSize: 1000},
		})
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to resolve shared table")
			return ctx.ApiServerError()
		}

		for _, table := range batch {
			if table.UID == tableUID {
				exists = true
			}
			if !table.ShareEnabled || table.ShareToken == "" {
				continue
			}
			if table.UID == tableUID {
				return ctx.ApiSuccess(dto.ResolvedLinkShare{Token: table.ShareToken})
			}
			if inherited == nil && (tableUID == "" || table.ShareIncludeChildren) {
				inherited = table
			}
		}

		seen += len(batch)
		if len(batch) == 0 || int64(seen) >= total {
			break
		}
	}

	if exists && inherited != nil {
		return ctx.ApiSuccess(dto.ResolvedLinkShare{Token: inherited.ShareToken})
	}
	return ctx.ApiError(http.StatusNotFound, "share::unavailable")
}

// Loader checks the share and its project on every public request, including password verification.
func (shareRoute) Loader(ctx context.Context) error {
	w := ctx.ResponseWriter()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	table, err := db.SLTables.GetByShareToken(ctx.Request().Context(), ctx.Param("shareToken"))
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ctx.ApiError(http.StatusNotFound, "share::unavailable")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to load link share")
		return ctx.ApiServerError()
	}

	project, err := db.Projects.GetByID(ctx.Request().Context(), table.ProjectID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "share::unavailable")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to load shared project")
		return ctx.ApiServerError()
	}

	ctx.Map(publicShare{Table: table}, project)
	return nil
}

const shareCookieName = "sayrud_share"
const shareGrantTTL = 24 * time.Hour

func shareGrant(table *db.SLTable, expires string) string {
	mac := hmac.New(sha256.New, []byte(table.SharePasswordHash))
	_, _ = mac.Write([]byte(table.ShareToken + ":" + expires))
	return expires + "." + hex.EncodeToString(mac.Sum(nil))
}

func validShareGrant(table *db.SLTable, value string) bool {
	expires, _, ok := strings.Cut(value, ".")
	until, err := strconv.ParseInt(expires, 10, 64)
	if !ok || err != nil || until <= time.Now().Unix() || until > time.Now().Add(shareGrantTTL).Unix() {
		return false
	}

	return hmac.Equal([]byte(value), []byte(shareGrant(table, expires)))
}

// RequirePassword deliberately grants only access to this share, never project membership.
func (shareRoute) RequirePassword(ctx context.Context, share publicShare) error {
	if share.Table.SharePasswordHash == "" {
		return nil
	}

	cookie, err := ctx.Request().Cookie(shareCookieName)
	if err == nil && validShareGrant(share.Table, cookie.Value) {
		return nil
	}

	return ctx.ApiError(http.StatusUnauthorized, "share::password_required")
}

// Serve
// @Summary Follow a public share over WebSocket
// @Description Receive filtered incremental changesets and scoped collaborator presence. All edits are rejected.
// @Description The share and password grant are checked at the handshake, on subscription and before delivery.
// @Param shareToken path string true "Share token"
// @Success 101 "Switching Protocols"
// @Failure 401 {string} string "Link password required"
// @Failure 404 {string} string "Share unavailable"
// @ID collaborateOnShare
// @Router /shares/{shareToken}/ws [get]
func (shareRoute) Serve(ctx context.Context, hub *collab.Hub, share publicShare, project *db.Project) error {
	session := &collab.ShareSession{
		TableID: share.Table.ID, Token: share.Table.ShareToken, PasswordHash: share.Table.SharePasswordHash,
	}
	if session.PasswordHash != "" {
		cookie, err := ctx.Request().Cookie(shareCookieName)
		if err != nil || !validShareGrant(share.Table, cookie.Value) {
			return ctx.ApiError(http.StatusUnauthorized, "share::password_required")
		}

		expires, _, _ := strings.Cut(cookie.Value, ".")
		until, _ := strconv.ParseInt(expires, 10, 64)
		session.ExpiresAt = time.Unix(until, 0)
	}

	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return ctx.ApiServerError()
	}

	if err := hub.Serve(ctx.ResponseWriter(), ctx.Request().Request, project, collab.Identity{
		MemberID: "guest" + hex.EncodeToString(raw), Name: ctx.Locale().Translate("share::visitor"),
		Color: "#86909c", Share: session,
	}, ctx.Locale()); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to serve shared WebSocket")
	}
	return nil
}

// Unlock
// @Summary Verify a public link password
// @Accept json
// @Produce json
// @Param shareToken path string true "Share token"
// @Param data body form.UnlockLinkShare true "Link password"
// @Success 200 {object} map[string]interface{}
// @ID unlockLinkShare
// @Router /shares/{shareToken}/unlock [post]
func (shareRoute) Unlock(ctx context.Context, share publicShare, f form.UnlockLinkShare) error {
	table := share.Table
	if table.SharePasswordHash == "" {
		return ctx.ApiSuccess(nil)
	}

	if err := redis.AuthAttempts.CheckSignIn(ctx.Request().Context(), "share:"+ctx.IP(), table.ShareToken+":"+ctx.IP()); err != nil {
		if errors.Is(err, redis.ErrTooManyAttempts) {
			return ctx.ApiError(http.StatusTooManyRequests, "common::too_many_attempts")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to limit link password attempts")
		return ctx.ApiServerError()
	}

	if len(f.Password) > 72 || bcrypt.CompareHashAndPassword([]byte(table.SharePasswordHash), []byte(f.Password)) != nil {
		return ctx.ApiError(http.StatusForbidden, "share::incorrect_password")
	}

	expires := time.Now().Add(shareGrantTTL)
	http.SetCookie(ctx.ResponseWriter(), &http.Cookie{
		Name: shareCookieName, Value: shareGrant(table, strconv.FormatInt(expires.Unix(), 10)),
		Path: "/_/shares/" + table.ShareToken, HttpOnly: true,
		Secure:   ctx.Request().TLS != nil || strings.EqualFold(ctx.Request().Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode, MaxAge: int(shareGrantTTL.Seconds()), Expires: expires,
	})
	return ctx.ApiSuccess(nil)
}

// Open
// @Summary Open a read-only public share
// @Produce json
// @Param shareToken path string true "Share token"
// @Success 200 {object} dto.SharedProject
// @ID openLinkShare
// @Router /shares/{shareToken} [get]
func (shareRoute) Open(ctx context.Context, share publicShare, project *db.Project) error {
	tables := []*db.SLTable{share.Table}
	if share.Table.ShareIncludeChildren {
		tables = nil
		for page := 1; ; page++ {
			batch, total, err := db.SLTables.Query(ctx.Request().Context(), db.QuerySLTableOptions{
				ProjectID: project.ID, Pagination: dbutil.Pagination{Page: page, PageSize: 1000},
			})
			if err != nil {
				logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list shared tables")
				return ctx.ApiServerError()
			}

			tables = append(tables, batch...)
			if len(batch) == 0 || int64(len(tables)) >= total {
				break
			}
		}
	}

	items := make([]*dto.TableListItem, 0, len(tables))
	for _, table := range tables {
		count, err := db.SLRecords.CountByTableID(ctx.Request().Context(), table.ID)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to count shared records")
			return ctx.ApiServerError()
		}

		items = append(items, &dto.TableListItem{Table: *dto.ToTable(project, table), Count: count})
	}

	return ctx.ApiSuccess(dto.SharedProject{
		Project: dto.ToProject(project, db.ProjectRoleViewer, nil), Tables: items,
		TableUID: share.Table.UID, IncludeChildren: share.Table.ShareIncludeChildren,
	})
}

// Tabler prevents a table UID from escaping the configured share scope.
func (shareRoute) Tabler(ctx context.Context, share publicShare, project *db.Project) error {
	if !share.Table.ShareIncludeChildren && ctx.Param("tableUID") != share.Table.UID {
		return ctx.ApiError(http.StatusNotFound, "table::not_found")
	}

	return Schemaless.Tabler(ctx, project)
}

// Snapshot
// @Summary Read a publicly shared table snapshot
// @Produce json
// @Param shareToken path string true "Share token"
// @Param tableUID path string true "Table UID"
// @Success 200 {object} dto.TableSnapshot
// @ID getSharedSnapshot
// @Router /shares/{shareToken}/tables/{tableUID}/snapshot [get]
func (shareRoute) Snapshot(ctx context.Context, tx dbutil.Transactor, project *db.Project, table *db.SLTable) error {
	return Collab.GetSnapshot(ctx, tx, project, table)
}

// sharedToken identifies public reads without requiring a share on authenticated project routes.
func sharedToken(ctx context.Context) string {
	value := ctx.Value(reflect.TypeFor[publicShare]())
	if !value.IsValid() {
		return ""
	}
	return value.Interface().(publicShare).Table.ShareToken
}

func prepareSharedFields(fields []*dto.Field, token string) {
	if token == "" {
		return
	}

	for _, field := range fields {
		field.Shortcut = nil
		field.Metadata = collab.PrepareSharedMetadata(field.Metadata)
	}
}

func prepareSharedRecords(records []*dto.Record, token string) {
	if token == "" {
		return
	}

	for _, record := range records {
		collab.PrepareSharedValues(record.Data, token, record.TableUID)
	}
}

// Changesets
// @Summary List missing changesets for a public share
// @Produce json
// @Param shareToken path string true "Share token"
// @Param tableUID path string true "Table UID"
// @Param since query int true "The latest revision the client has applied"
// @Success 200 {object} dto.ListChangesetsResp
// @ID listSharedChangesets
// @Router /shares/{shareToken}/tables/{tableUID}/changesets [get]
func (shareRoute) Changesets(ctx context.Context, table *db.SLTable) error {
	return Collab.ListChangesets(ctx, table)
}

// Fields
// @Summary List fields for a public share
// @Produce json
// @Param shareToken path string true "Share token"
// @Param tableUID path string true "Table UID"
// @Success 200 {array} dto.Field
// @ID listSharedFields
// @Router /shares/{shareToken}/tables/{tableUID}/fields [get]
func (shareRoute) Fields(ctx context.Context, table *db.SLTable) error {
	return Schemaless.ListFields(ctx, table)
}

// Views
// @Summary List views for a public share
// @Produce json
// @Param shareToken path string true "Share token"
// @Param tableUID path string true "Table UID"
// @Success 200 {array} dto.SLView
// @ID listSharedViews
// @Router /shares/{shareToken}/tables/{tableUID}/views [get]
func (shareRoute) Views(ctx context.Context, table *db.SLTable) error {
	return Collab.ListViews(ctx, table)
}

// FetchRecords
// @Summary Fetch dirty records for a public share
// @Accept json
// @Produce json
// @Param shareToken path string true "Share token"
// @Param tableUID path string true "Table UID"
// @Param data body form.FetchRecords true "Record UIDs"
// @Success 200 {array} dto.Record
// @ID fetchSharedRecords
// @Router /shares/{shareToken}/tables/{tableUID}/records/fetch [post]
func (shareRoute) FetchRecords(ctx context.Context, table *db.SLTable, f form.FetchRecords) error {
	return Collab.FetchRecords(ctx, table, f)
}

// Attachment
// @Summary Read an attachment referenced by a shared table
// @Param shareToken path string true "Share token"
// @Param tableUID path string true "Table UID"
// @Param fileUID path string true "Attachment UID"
// @Param download query boolean false "Force file download"
// @Success 200 {file} file
// @ID getSharedAttachment
// @Router /shares/{shareToken}/tables/{tableUID}/attachments/{fileUID} [get]
func (shareRoute) Attachment(ctx context.Context, project *db.Project, table *db.SLTable) error {
	referenced, err := db.SLRecords.HasAttachment(ctx.Request().Context(), table.ID, ctx.Param("fileUID"))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to check shared attachment")
		return ctx.ApiServerError()
	}
	if !referenced {
		return ctx.ApiError(http.StatusNotFound, "attachment::not_found")
	}

	return Schemaless.Attachment(ctx, project)
}
