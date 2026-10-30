package api

import (
	stdcontext "context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/setup"
)

type setupExistingUsers struct{ db.UsersStore }

func (setupExistingUsers) CreateFirstAdmin(stdcontext.Context, db.CreateUserOptions) (*db.User, error) {
	return nil, db.ErrUsersExist
}

func TestSetupAdminPasswordByteLimit(t *testing.T) {
	previousUsers, previousSettings := db.Users, db.Settings
	t.Cleanup(func() { db.Users, db.Settings = previousUsers, previousSettings })

	db.Users = setupExistingUsers{}
	db.Settings = &adminSystemSettingsStore{settings: db.DefaultSystemSettings()}

	for _, tc := range []struct {
		password string
		status   int
	}{
		{strings.Repeat("é", 36), http.StatusConflict},
		{strings.Repeat("é", 37), http.StatusBadRequest},
		{strings.Repeat("密", 24), http.StatusConflict},
		{strings.Repeat("密", 25), http.StatusBadRequest},
	} {
		t.Run(fmt.Sprint(len(tc.password)), func(t *testing.T) {
			token, err := setup.NewToken()
			require.NoError(t, err)

			wizard := setup.New(setup.Options{Token: token, Stage: setup.StageAdmin, Hooks: setup.Hooks{Finish: func() {}}})
			router := flamego.New()
			router.Use(i18n.Middleware(), context.Contexter(nil))
			router.Map(wizard)
			router.Post("/admin", Setup.Authorizer, Setup.LimitBody, form.Bind(form.SetupAdmin{}), Setup.CreateAdmin)

			body := fmt.Sprintf(`{"email":"admin@example.com","userName":"Admin","password":%q}`, tc.password)
			request := httptest.NewRequest(http.MethodPost, "/admin", strings.NewReader(body))
			request.Header.Set(SetupTokenHeader, token.String())
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept-Language", "en")

			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			assert.Equal(t, tc.status, response.Code)
			if tc.status == http.StatusBadRequest {
				assert.Contains(t, response.Body.String(), "Password must be at most 72 bytes")
			}
		})
	}
}
