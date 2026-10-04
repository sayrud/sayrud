package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/db"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("PGHOST") == "" {
		t.Skip("PGHOST is not set")
	}
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: "", PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "test_" + strings.ToLower(randstr.String(10))
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "search_path=" + schema, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, gormDB.AutoMigrate(&db.File{}))
	return gormDB
}

func useTestStorage(t *testing.T) *LocalStorage {
	t.Helper()
	s, _ := newTestLocal(t)
	prev := Default
	Default = s
	t.Cleanup(func() { Default = prev })
	return s
}

func TestUpload(t *testing.T) {
	ctx := context.Background()
	db.Files = db.NewFilesStore(newTestDB(t))
	s := useTestStorage(t)

	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("x", 1000)
	file, err := Upload(ctx, UploadOptions{
		Category: CategoryAvatar,
		UserID:   7,
		Name:     `C:\Users\me\My Photo.PNG`,
		Reader:   strings.NewReader(png),
		Size:     int64(len(png)),
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(file.UID, "fil"))
	assert.Equal(t, "avatars/"+time.Now().Format("2006/01")+"/"+file.UID+".png", file.Path)
	assert.Equal(t, "My Photo.PNG", file.Name)
	assert.Equal(t, "image/png", file.ContentType)
	assert.Equal(t, "local", file.Storage)
	assert.Equal(t, int64(len(png)), file.Size)
	sum := sha256.Sum256([]byte(png))
	assert.Equal(t, hex.EncodeToString(sum[:]), file.SHA256)
	assert.Equal(t, int64(7), file.UserID)

	got, err := db.Files.GetByUID(ctx, file.UID)
	require.NoError(t, err)
	assert.Equal(t, file.Path, got.Path)

	obj, err := OpenFile(ctx, got)
	require.NoError(t, err)
	b, err := io.ReadAll(obj)
	require.NoError(t, err)
	require.NoError(t, obj.Close())
	assert.Equal(t, png, string(b))

	_, err = FileURL(ctx, got)
	assert.ErrorIs(t, err, ErrURLNotSupported)

	require.NoError(t, Remove(ctx, got))
	_, err = db.Files.GetByID(ctx, got.ID)
	assert.ErrorIs(t, err, db.ErrFileNotFound)
	_, err = s.Stat(ctx, got.Path)
	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.ErrorIs(t, Remove(ctx, got), db.ErrFileNotFound)
}

func TestUpload_Failures(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t)
	db.Files = db.NewFilesStore(gormDB)
	s := useTestStorage(t)

	_, err := Upload(ctx, UploadOptions{Category: "other", Reader: strings.NewReader("x"), Size: 1})
	require.Error(t, err)

	// A mismatched length writes neither the object nor the record.
	_, err = Upload(ctx, UploadOptions{Category: CategoryAttachment, Name: "a.txt", Reader: strings.NewReader("abc"), Size: 10})
	require.Error(t, err)

	// The written object is deleted if creating the record fails.
	require.NoError(t, gormDB.Migrator().DropTable(&db.File{}))
	_, err = Upload(ctx, UploadOptions{Category: CategoryAttachment, Name: "b.txt", Reader: strings.NewReader("abc"), Size: -1})
	require.Error(t, err)

	var paths []string
	require.NoError(t, s.IterateObjects(ctx, "", func(p string, _ Object) error {
		paths = append(paths, p)
		return nil
	}))
	assert.Empty(t, paths)
}

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"report.pdf":                      "report.pdf",
		"  a b.png  ":                     "a b.png",
		"../../etc/passwd":                "passwd",
		`C:\fakepath\x.jpg`:               "x.jpg",
		"evil\r\nname.txt":                "evilname.txt",
		"..":                              "",
		"dir/":                            "",
		strings.Repeat("长", 300) + ".png": strings.Repeat("长", NameMaxLength-4) + ".png",
	} {
		assert.Equal(t, want, sanitizeName(in), in)
	}
}

func TestExtension(t *testing.T) {
	for in, want := range map[string]string{
		"a.PNG":               ".png",
		"a.tar.gz":            ".gz",
		"a":                   "",
		"a.":                  "",
		"a.p n g":             "",
		"a.verylongextension": "",
		"a.文档":                "",
	} {
		assert.Equal(t, want, extension(in), in)
	}
}
