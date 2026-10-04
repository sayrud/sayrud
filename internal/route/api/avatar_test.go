package api

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flamego/binding"
	"github.com/flamego/flamego"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/storage"
)

type avatarUsersStore struct {
	db.UsersStore
	user   *db.User
	setErr error
}

func (s *avatarUsersStore) GetByID(_ stdcontext.Context, id int64) (*db.User, error) {
	if s.user == nil || s.user.ID != id {
		return nil, db.ErrUserNotFound
	}

	copy := *s.user
	return &copy, nil
}

func (s *avatarUsersStore) SetAvatar(_ stdcontext.Context, id int64, uid string) (*db.User, string, error) {
	if s.setErr != nil {
		return nil, "", s.setErr
	}

	previous := s.user.AvatarFileUID
	s.user.AvatarFileUID = uid
	s.user.UpdatedAt = time.Now()
	copy := *s.user
	return &copy, previous, nil
}

type avatarFilesStore struct {
	db.FilesStore
	files                map[string]*db.File
	createErr, deleteErr error
}

func (s *avatarFilesStore) Create(_ stdcontext.Context, opts db.CreateFileOptions) (*db.File, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}

	file := &db.File{
		Model: dbutil.Model{ID: int64(len(s.files) + 1), CreatedAt: time.Now()},
		UID:   opts.UID, Category: opts.Category, Storage: opts.Storage, Path: opts.Path,
		Name: opts.Name, Size: opts.Size, ContentType: opts.ContentType, SHA256: opts.SHA256, UserID: opts.UserID,
	}
	s.files[file.UID] = file
	return file, nil
}

func (s *avatarFilesStore) GetByUID(_ stdcontext.Context, uid string) (*db.File, error) {
	if file, ok := s.files[uid]; ok {
		return file, nil
	}
	return nil, db.ErrFileNotFound
}

func (s *avatarFilesStore) DeleteByID(_ stdcontext.Context, id int64) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	for uid, file := range s.files {
		if file.ID == id {
			delete(s.files, uid)
			return nil
		}
	}

	return db.ErrFileNotFound
}

type avatarObjectStorage struct {
	storage.ObjectStorage
	saveErr, openErr error
	saveCalls        int
}

func (s *avatarObjectStorage) Save(ctx stdcontext.Context, path string, r io.Reader, size int64) (int64, error) {
	s.saveCalls++
	if s.saveErr != nil {
		return 0, s.saveErr
	}
	return s.ObjectStorage.Save(ctx, path, r, size)
}

func (s *avatarObjectStorage) Open(ctx stdcontext.Context, path string) (storage.Object, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	return s.ObjectStorage.Open(ctx, path)
}

func avatarTestRouter(t *testing.T) (*flamego.Flame, *avatarUsersStore, *avatarFilesStore, *avatarObjectStorage) {
	t.Helper()

	previousUsers, previousFiles, previousStorage := db.Users, db.Files, storage.Default
	t.Cleanup(func() { db.Users, db.Files, storage.Default = previousUsers, previousFiles, previousStorage })

	users := &avatarUsersStore{user: &db.User{Model: dbutil.Model{ID: 1}, UserName: "Alice", Email: "alice@example.com"}}
	files := &avatarFilesStore{files: make(map[string]*db.File)}
	local, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	objects := &avatarObjectStorage{ObjectStorage: local}
	db.Users, db.Files, storage.Default = users, files, objects

	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Map(users.user, collab.NewHub(nil))
	f.Post("/auth/avatar", Auth.LimitAvatarUpload,
		binding.MultipartForm(form.UploadAvatar{}, binding.Options{MaxMemory: AvatarMaxSize}), Auth.UploadAvatar)
	f.Delete("/auth/avatar", Auth.RemoveAvatar)
	f.Get("/avatars/{fileUID}", Auth.Avatar)

	return f, users, files, objects
}

func avatarImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})

	var content bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&content, img)
	case "jpeg":
		err = jpeg.Encode(&content, img, nil)
	case "gif":
		frame := image.NewPaletted(img.Bounds(), color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}})
		frame.SetColorIndex(0, 0, 1)
		err = gif.EncodeAll(&content, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{1, 1}})
	}
	require.NoError(t, err)
	return content.Bytes()
}

func avatarRequest(t *testing.T, router *flamego.Flame, content []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	file, err := w.CreateFormFile("file", "untrusted.svg")
	require.NoError(t, err)
	_, err = file.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	r := httptest.NewRequest(http.MethodPost, "/auth/avatar", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Accept-Language", "en")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, r)
	return response
}

func TestNormalizeAvatar(t *testing.T) {
	for _, format := range []string{"png", "jpeg", "gif"} {
		t.Run(format, func(t *testing.T) {
			input := append(avatarImage(t, format, 1024, 512), []byte("<script>untrusted trailing content</script>")...)
			output, err := normalizeAvatar(input)
			require.NoError(t, err)

			config, format, err := image.DecodeConfig(bytes.NewReader(output))
			require.NoError(t, err)
			require.Equal(t, "png", format)
			require.Equal(t, 512, config.Width)
			require.Equal(t, 256, config.Height)
			require.NotContains(t, string(output), "<script>")
		})
	}

	for _, input := range [][]byte{nil, []byte("<svg></svg>"), []byte("\x89PNG\r\n\x1a\nnot an image"), avatarImage(t, "png", 2, 2)[:30]} {
		_, err := normalizeAvatar(input)
		require.ErrorIs(t, err, errInvalidAvatar)
	}

	_, err := normalizeAvatar(avatarImage(t, "png", 4097, 1))
	require.ErrorIs(t, err, errAvatarDimensions)

	output, err := normalizeAvatar(avatarImage(t, "png", 1, 1024))
	require.NoError(t, err)
	config, _, err := image.DecodeConfig(bytes.NewReader(output))
	require.NoError(t, err)
	require.Equal(t, 1, config.Width)
	require.Equal(t, 512, config.Height)
}

func TestNormalizeAvatarAveragesColorsAndTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 2))
	for x := 0; x < 1024; x++ {
		img.SetNRGBA(x, 0, color.NRGBA{R: 255, A: 255})
		img.SetNRGBA(x, 1, color.NRGBA{B: 255})
	}

	var input bytes.Buffer
	require.NoError(t, png.Encode(&input, img))

	output, err := normalizeAvatar(input.Bytes())
	require.NoError(t, err)
	got, err := png.Decode(bytes.NewReader(output))
	require.NoError(t, err)
	pixel := color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA)

	require.Equal(t, uint8(255), pixel.R)
	require.Equal(t, uint8(0), pixel.B)
	require.Equal(t, uint8(127), pixel.A)
}

func TestAvatarLifecycle(t *testing.T) {
	router, users, files, objects := avatarTestRouter(t)
	input := avatarImage(t, "png", 20, 10)

	response := avatarRequest(t, router, input)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	var body struct {
		Data dto.Profile `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	uid := users.user.AvatarFileUID
	require.Equal(t, "/_/avatars/"+uid, body.Data.AvatarURL)
	require.Equal(t, body.Data.AvatarURL, dto.ToUserBrief(users.user).AvatarURL)
	require.Equal(t, body.Data.AvatarURL, dto.ToAdminUser(users.user, 0, nil).AvatarURL)

	file := files.files[uid]
	require.Equal(t, "avatar.png", file.Name)
	require.Equal(t, "image/png", file.ContentType)
	require.Equal(t, "avatars", file.Category)

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/avatars/"+uid, nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "image/png", response.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	require.Contains(t, response.Header().Get("Content-Security-Policy"), "sandbox")
	require.Contains(t, response.Header().Get("Cache-Control"), "immutable")
	_, err := png.Decode(response.Body)
	require.NoError(t, err)

	conditional := httptest.NewRequest(http.MethodGet, "/avatars/"+uid, nil)
	conditional.Header.Set("If-None-Match", response.Header().Get("ETag"))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, conditional)
	require.Equal(t, http.StatusNotModified, response.Code)

	response = avatarRequest(t, router, avatarImage(t, "jpeg", 10, 10))
	require.Equal(t, http.StatusOK, response.Code)
	require.NotEqual(t, uid, users.user.AvatarFileUID)
	require.Len(t, files.files, 1)
	_, err = objects.Stat(stdcontext.Background(), file.Path)
	require.ErrorIs(t, err, fs.ErrNotExist)

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/avatars/"+uid, nil))
	require.Equal(t, http.StatusNotFound, response.Code)

	for i := 0; i < 2; i++ {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/auth/avatar", nil))
		require.Equal(t, http.StatusOK, response.Code)
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		require.Empty(t, body.Data.AvatarURL)
		require.Empty(t, users.user.AvatarFileUID)
		require.Empty(t, files.files)
	}
}

func TestAvatarUploadRejectsInvalidBody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []byte
		status  int
		message string
	}{
		{"invalid image", []byte("<svg></svg>"), http.StatusBadRequest, "Avatar must be a PNG, JPEG or GIF image"},
		{"oversize file", make([]byte, AvatarMaxSize+1), http.StatusRequestEntityTooLarge, "Avatar must be at most 2 MiB"},
		{"oversize body", make([]byte, AvatarMaxSize+avatarFormOverhead), http.StatusRequestEntityTooLarge, "Avatar must be at most 2 MiB"},
		{"oversize dimensions", avatarImage(t, "png", 4097, 1), http.StatusBadRequest, "Avatar dimensions must not exceed 4096 × 4096 pixels"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, users, files, objects := avatarTestRouter(t)
			response := avatarRequest(t, router, tc.content)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			var body struct {
				Msg string `json:"msg"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, tc.message, body.Msg)
			require.Empty(t, users.user.AvatarFileUID)
			require.Empty(t, files.files)
			require.Zero(t, objects.saveCalls)
		})
	}

	t.Run("missing file", func(t *testing.T) {
		router, _, files, objects := avatarTestRouter(t)

		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		require.NoError(t, w.WriteField("file", "not a file"))
		require.NoError(t, w.Close())

		r := httptest.NewRequest(http.MethodPost, "/auth/avatar", &body)
		r.Header.Set("Content-Type", w.FormDataContentType())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Empty(t, files.files)
		require.Zero(t, objects.saveCalls)
	})

	t.Run("text and file collision", func(t *testing.T) {
		router, users, files, objects := avatarTestRouter(t)
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file", "avatar.png")
		require.NoError(t, err)
		_, err = part.Write(avatarImage(t, "png", 2, 2))
		require.NoError(t, err)
		require.NoError(t, w.WriteField("file", "not a file"))
		require.NoError(t, w.Close())

		r := httptest.NewRequest(http.MethodPost, "/auth/avatar", &body)
		r.Header.Set("Content-Type", w.FormDataContentType())
		r.Header.Set("Accept-Language", "en")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "Avatar must be a PNG, JPEG or GIF image")
		require.Empty(t, users.user.AvatarFileUID)
		require.Empty(t, files.files)
		require.Zero(t, objects.saveCalls)
	})

	for _, contentType := range []string{"application/x-www-form-urlencoded", "application/json", "multipart/form-data; boundary=missing"} {
		t.Run(contentType, func(t *testing.T) {
			router, _, files, objects := avatarTestRouter(t)
			r := httptest.NewRequest(http.MethodPost, "/auth/avatar", strings.NewReader("file=invalid"))
			r.Header.Set("Content-Type", contentType)
			r.Header.Set("Accept-Language", "en")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, r)
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Contains(t, response.Body.String(), "Invalid request body")
			require.Empty(t, files.files)
			require.Zero(t, objects.saveCalls)
		})
	}
}

func TestAvatarBindingUsesFirstFile(t *testing.T) {
	router, _, files, objects := avatarTestRouter(t)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i, field := range []string{"file", "file", "other"} {
		part, err := w.CreateFormFile(field, "avatar.png")
		require.NoError(t, err)
		content := []byte("not an image")
		if i == 0 {
			content = avatarImage(t, "png", 2, 2)
		}
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	r := httptest.NewRequest(http.MethodPost, "/auth/avatar", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, r)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Len(t, files.files, 1)
	require.Equal(t, 1, objects.saveCalls)
}

func TestAvatarBindingRemovesSpilledFiles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []byte
		status  int
	}{
		{"success", avatarImage(t, "png", 2, 2), http.StatusOK},
		{"invalid image", []byte("<svg></svg>"), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, _, _, _ := avatarTestRouter(t)
			router.Post("/spill", Auth.LimitAvatarUpload,
				binding.MultipartForm(form.UploadAvatar{}, binding.Options{MaxMemory: 1}), Auth.UploadAvatar)

			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			part, err := w.CreateFormFile("file", "avatar.png")
			require.NoError(t, err)
			_, err = part.Write(tc.content)
			require.NoError(t, err)
			require.NoError(t, w.Close())

			r := httptest.NewRequest(http.MethodPost, "/spill", &body)
			r.Header.Set("Content-Type", w.FormDataContentType())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, r)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			require.NotNil(t, r.MultipartForm)
			require.Len(t, r.MultipartForm.File["file"], 1)
			for _, header := range r.MultipartForm.File["file"] {
				file, err := header.Open()
				if file != nil {
					_ = file.Close()
				}

				require.ErrorIs(t, err, fs.ErrNotExist)
			}
		})
	}
}

func TestAvatarUploadFailureKeepsPreviousAvatar(t *testing.T) {
	for _, failure := range []string{"save", "metadata", "assign"} {
		t.Run(failure, func(t *testing.T) {
			router, users, files, objects := avatarTestRouter(t)
			input := avatarImage(t, "png", 2, 2)
			require.Equal(t, http.StatusOK, avatarRequest(t, router, input).Code)

			previous := users.user.AvatarFileUID
			err := errors.New("simulated failure")
			switch failure {
			case "save":
				objects.saveErr = err
			case "metadata":
				files.createErr = err
			case "assign":
				users.setErr = err
			}

			require.Equal(t, http.StatusInternalServerError, avatarRequest(t, router, input).Code)
			require.Equal(t, previous, users.user.AvatarFileUID)
			require.Len(t, files.files, 1)

			var paths []string
			require.NoError(t, objects.IterateObjects(stdcontext.Background(), "", func(path string, _ storage.Object) error { paths = append(paths, path); return nil }))
			require.Len(t, paths, 1)
		})
	}

	t.Run("previous cleanup failure", func(t *testing.T) {
		router, users, files, _ := avatarTestRouter(t)
		input := avatarImage(t, "png", 2, 2)
		require.Equal(t, http.StatusOK, avatarRequest(t, router, input).Code)

		previous := users.user.AvatarFileUID
		files.deleteErr = errors.New("simulated cleanup failure")
		require.Equal(t, http.StatusOK, avatarRequest(t, router, input).Code)
		require.NotEqual(t, previous, users.user.AvatarFileUID)

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/avatars/"+previous, nil))
		require.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestAvatarServingBoundary(t *testing.T) {
	for _, invalid := range []string{"attachment", "stale", "missing owner", "missing object", "storage failure"} {
		t.Run(invalid, func(t *testing.T) {
			router, users, files, objects := avatarTestRouter(t)
			require.Equal(t, http.StatusOK, avatarRequest(t, router, avatarImage(t, "png", 2, 2)).Code)

			uid := users.user.AvatarFileUID
			status := http.StatusNotFound
			switch invalid {
			case "attachment":
				files.files[uid].Category = "attachments"
			case "stale":
				users.user.AvatarFileUID = "different"
			case "missing owner":
				users.user = nil
			case "missing object":
				require.NoError(t, objects.Delete(stdcontext.Background(), files.files[uid].Path))
			case "storage failure":
				objects.openErr = os.ErrPermission
				status = http.StatusInternalServerError
			}

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/avatars/"+uid, nil))
			require.Equal(t, status, response.Code, response.Body.String())
		})
	}
}
