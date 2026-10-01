package form

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/i18n"
)

func bindMessage(t *testing.T, lang, body string) string {
	t.Helper()
	f := flamego.New()
	f.Use(i18n.Middleware())
	f.Post("/", Bind(SignIn{}), func() string { return "ok" })

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: i18n.CookieName, Value: lang})
	f.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var out struct{ Msg string }
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	return out.Msg
}

func TestBind_Language(t *testing.T) {
	assert.Contains(t, bindMessage(t, i18n.LangEn, `{"password":"x"}`), "Email")
	assert.Contains(t, bindMessage(t, i18n.LangZhCN, `{"password":"x"}`), "邮箱")
	assert.Equal(t, "Invalid request body", bindMessage(t, i18n.LangEn, `{`))
}
