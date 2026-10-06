package api

import (
	"bytes"
	stdcontext "context"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/flamego/binding"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/storage"
)

const (
	// AvatarMaxSize is the maximum number of bytes in an uploaded avatar image.
	AvatarMaxSize      = 2 << 20
	avatarMaxDimension = 4096
	avatarMaxPixels    = 16 << 20
	avatarOutputSize   = 512
	avatarFormOverhead = 64 << 10
)

// LimitAvatarUpload bounds the body before binding and cleans up multipart temporary files after the handlers finish.
func (authRoute) LimitAvatarUpload(ctx context.Context) {
	r := ctx.Request().Request
	r.Body = http.MaxBytesReader(ctx.ResponseWriter(), r.Body, AvatarMaxSize+avatarFormOverhead)
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	ctx.Next()
}

var (
	errInvalidAvatar    = errors.New("invalid avatar image")
	errAvatarDimensions = errors.New("avatar image dimensions exceed the limit")
)

// UploadAvatar
// @Summary Upload the avatar of the signed-in user
// @Description Accept a PNG, JPEG or GIF up to 2 MiB and 4096 pixels per edge. Store a static PNG with its longest edge at most 512 pixels.
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Avatar image"
// @Success 200 {object} dto.Profile
// @Failure 400 {string} string "Invalid image or multipart body"
// @Failure 401 {string} string "Not signed in"
// @Failure 413 {string} string "Image is too large"
// @Failure 500 {string} string "Internal server error"
// @ID uploadAvatar
// @Router /auth/avatar [post]
func (authRoute) UploadAvatar(ctx context.Context, user *db.User, hub *collab.Hub, f form.UploadAvatar, errs binding.Errors) error {
	r := ctx.Request().Request
	for _, err := range errs {
		var tooLarge *http.MaxBytesError
		if errors.As(err.Err, &tooLarge) {
			return ctx.ApiError(http.StatusRequestEntityTooLarge, "auth::avatar_too_large")
		}

		if err.Category == binding.ErrorCategoryDeserialization {
			return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
		}
	}

	if len(errs) > 0 {
		return ctx.ApiError(http.StatusBadRequest, "auth::avatar_invalid")
	}

	if f.File.Size > AvatarMaxSize {
		return ctx.ApiError(http.StatusRequestEntityTooLarge, "auth::avatar_too_large")
	}

	file, err := f.File.Open()
	if err != nil {
		logrus.WithContext(r.Context()).WithError(err).Error("Failed to open uploaded avatar")
		return ctx.ApiServerError()
	}
	defer func() { _ = file.Close() }()

	input, err := io.ReadAll(io.LimitReader(file, AvatarMaxSize+1))
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "auth::avatar_invalid")
	}
	if len(input) > AvatarMaxSize {
		return ctx.ApiError(http.StatusRequestEntityTooLarge, "auth::avatar_too_large")
	}

	content, err := normalizeAvatar(input)
	if err != nil {
		if errors.Is(err, errAvatarDimensions) {
			return ctx.ApiError(http.StatusBadRequest, "auth::avatar_dimensions")
		}

		return ctx.ApiError(http.StatusBadRequest, "auth::avatar_invalid")
	}

	avatar, err := storage.Upload(r.Context(), storage.UploadOptions{
		Category: storage.CategoryAvatar,
		UserID:   user.ID,
		Name:     "avatar.png",
		Reader:   bytes.NewReader(content),
		Size:     int64(len(content)),
	})
	if err != nil {
		logrus.WithContext(r.Context()).WithError(err).Error("Failed to upload avatar")
		return ctx.ApiServerError()
	}

	updated, previous, err := db.Users.SetAvatar(r.Context(), user.ID, avatar.UID)
	if err != nil {
		if cleanupErr := storage.Remove(stdcontext.WithoutCancel(r.Context()), avatar); cleanupErr != nil {
			logrus.WithContext(r.Context()).WithError(cleanupErr).Warn("Failed to remove unassigned avatar")
		}

		logrus.WithContext(r.Context()).WithError(err).Error("Failed to update avatar")
		return ctx.ApiServerError()
	}

	removePreviousAvatar(r.Context(), user.ID, previous)
	hub.UpdateUserAvatar(updated.ID, dto.UserAvatarURL(updated), updated.UpdatedAt)

	return ctx.ApiSuccess(dto.ToProfile(updated))
}

// normalizeAvatar decodes before storing, strips metadata and trailing content, and uses only the first frame of a GIF.
func normalizeAvatar(input []byte) ([]byte, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil || (format != "png" && format != "jpeg" && format != "gif") {
		return nil, errInvalidAvatar
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > avatarMaxDimension || config.Height > avatarMaxDimension || int64(config.Width)*int64(config.Height) > avatarMaxPixels {
		return nil, errAvatarDimensions
	}

	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, errInvalidAvatar
	}

	bounds := img.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	width, height := sourceWidth, sourceHeight
	if max(width, height) > avatarOutputSize {
		width = max(1, width*avatarOutputSize/max(sourceWidth, sourceHeight))
		height = max(1, height*avatarOutputSize/max(sourceWidth, sourceHeight))

		resized := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			y0, y1 := y*sourceHeight/height, (y+1)*sourceHeight/height
			for x := 0; x < width; x++ {
				x0, x1 := x*sourceWidth/width, (x+1)*sourceWidth/width
				var red, green, blue, alpha uint64
				for sy := y0; sy < y1; sy++ {
					for sx := x0; sx < x1; sx++ {
						r, g, b, a := img.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
						red, green, blue, alpha = red+uint64(r), green+uint64(g), blue+uint64(b), alpha+uint64(a)
					}
				}

				count := uint64((x1 - x0) * (y1 - y0))
				// Average premultiplied colors so transparent pixels do not create dark edges.
				resized.Set(x, y, color.RGBA64{R: uint16(red / count), G: uint16(green / count), B: uint16(blue / count), A: uint16(alpha / count)})
			}
		}
		img = resized
	}

	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		return nil, errInvalidAvatar
	}

	return output.Bytes(), nil
}

// RemoveAvatar
// @Summary Restore the default avatar of the signed-in user
// @Produce json
// @Success 200 {object} dto.Profile
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID removeAvatar
// @Router /auth/avatar [delete]
func (authRoute) RemoveAvatar(ctx context.Context, user *db.User, hub *collab.Hub) error {
	c := ctx.Request().Context()
	updated, previous, err := db.Users.SetAvatar(c, user.ID, "")
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to remove avatar")
		return ctx.ApiServerError()
	}

	removePreviousAvatar(c, user.ID, previous)
	hub.UpdateUserAvatar(updated.ID, "", updated.UpdatedAt)

	return ctx.ApiSuccess(dto.ToProfile(updated))
}

// removePreviousAvatar is best effort after committing the replacement, so cleanup failure never discards the new avatar.
func removePreviousAvatar(ctx stdcontext.Context, userID int64, uid string) {
	if uid == "" {
		return
	}

	ctx = stdcontext.WithoutCancel(ctx)
	file, err := db.Files.GetByUID(ctx, uid)
	if errors.Is(err, db.ErrFileNotFound) {
		return
	}

	if err == nil && file.Category == string(storage.CategoryAvatar) && file.UserID == userID {
		err = storage.Remove(ctx, file)
	}
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Warn("Failed to remove previous avatar")
	}
}

// Avatar
// @Summary Get a current uploaded avatar
// @Produce image/png
// @Param fileUID path string true "Avatar file UID"
// @Success 200 {file} file
// @Failure 404 {string} string "Avatar not found"
// @Failure 500 {string} string "Internal server error"
// @ID getAvatar
// @Router /avatars/{fileUID} [get]
func (authRoute) Avatar(ctx context.Context) error {
	c := ctx.Request().Context()
	file, err := db.Files.GetByUID(c, ctx.Param("fileUID"))
	if err != nil {
		if errors.Is(err, db.ErrFileNotFound) {
			return ctx.Status(http.StatusNotFound)
		}

		logrus.WithContext(c).WithError(err).Error("Failed to get avatar")
		return ctx.ApiServerError()
	}
	if file.Category != string(storage.CategoryAvatar) || file.ContentType != "image/png" {
		return ctx.Status(http.StatusNotFound)
	}

	user, err := db.Users.GetByID(c, file.UserID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return ctx.Status(http.StatusNotFound)
		}

		logrus.WithContext(c).WithError(err).Error("Failed to get avatar owner")
		return ctx.ApiServerError()
	}
	if user.AvatarFileUID != file.UID {
		return ctx.Status(http.StatusNotFound)
	}

	object, err := storage.OpenFile(c, file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ctx.Status(http.StatusNotFound)
		}

		logrus.WithContext(c).WithError(err).Error("Failed to open avatar")
		return ctx.ApiServerError()
	}
	defer func() { _ = object.Close() }()

	w := ctx.ResponseWriter()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "inline; filename=avatar.png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+file.UID+`"`)

	http.ServeContent(w, ctx.Request().Request, "avatar.png", file.CreatedAt, object)
	return nil
}
