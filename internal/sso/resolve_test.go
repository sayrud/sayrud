package sso

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

// fakeUsers only implements the methods used by Resolve, the others panic.
type fakeUsers struct {
	db.UsersStore
	users  map[int64]*db.User
	nextID int64
}

func (f *fakeUsers) GetByID(_ context.Context, id int64) (*db.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, db.ErrUserNotFound
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (*db.User, error) {
	for _, u := range f.users {
		if u.Email == db.NormalizeEmail(email) {
			return u, nil
		}
	}
	return nil, db.ErrUserNotFound
}

func (f *fakeUsers) ClaimLegacyDefault(context.Context, db.CreateUserOptions) (*db.User, error) {
	return nil, db.ErrUserNotFound
}

func (f *fakeUsers) Create(_ context.Context, opts db.CreateUserOptions) (*db.User, error) {
	f.nextID++
	u := &db.User{Email: db.NormalizeEmail(opts.Email), UserName: opts.UserName}
	u.ID = f.nextID
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeUsers) EnsureAdmin(context.Context) error { return nil }

type fakeIdentities struct {
	db.UserIdentitiesStore
	identities []*db.UserIdentity
	touched    []int64
}

func (f *fakeIdentities) GetByProviderSubject(_ context.Context, providerID int64, subject string) (*db.UserIdentity, error) {
	for _, i := range f.identities {
		if i.ProviderID == providerID && i.Subject == subject {
			return i, nil
		}
	}
	return nil, db.ErrUserIdentityNotFound
}

func (f *fakeIdentities) Create(_ context.Context, identity *db.UserIdentity) error {
	for _, i := range f.identities {
		if i.ProviderID == identity.ProviderID && (i.Subject == identity.Subject || i.UserID == identity.UserID) {
			return db.ErrUserIdentityTaken
		}
	}
	identity.ID = int64(len(f.identities) + 1)
	f.identities = append(f.identities, identity)
	return nil
}

func (f *fakeIdentities) Touch(_ context.Context, id int64, _ string) error {
	f.touched = append(f.touched, id)
	return nil
}

func withFakeStores(t *testing.T) (*fakeUsers, *fakeIdentities) {
	t.Helper()
	users, identities := db.Users, db.UserIdentities
	t.Cleanup(func() { db.Users, db.UserIdentities = users, identities })

	fu := &fakeUsers{users: map[int64]*db.User{}, nextID: 100}
	alice := &db.User{Email: "alice@example.com", UserName: "Alice"}
	alice.ID = 1
	now := time.Now()
	disabled := &db.User{Email: "eve@example.com", UserName: "Eve", DisabledAt: &now}
	disabled.ID = 2
	fu.users[1], fu.users[2] = alice, disabled
	fi := &fakeIdentities{}
	db.Users, db.UserIdentities = fu, fi
	return fu, fi
}

func TestResolve(t *testing.T) {
	ctx := context.Background()
	provider := func(mut func(p *db.AuthProvider)) *db.AuthProvider {
		p := &db.AuthProvider{Model: dbutil.Model{ID: 7}}
		if mut != nil {
			mut(p)
		}
		return p
	}
	code := func(err error) string {
		require.Error(t, err)
		return ErrorCode(err)
	}

	t.Run("not linked", func(t *testing.T) {
		withFakeStores(t)
		_, err := Resolve(ctx, provider(nil), &Identity{Subject: "s1", Email: "alice@example.com", EmailVerified: true}, 0)
		assert.Equal(t, CodeNotLinked, code(err))
	})

	t.Run("link by verified email, then sign in by identity", func(t *testing.T) {
		_, fi := withFakeStores(t)
		p := provider(func(p *db.AuthProvider) { p.LinkByEmail = true })
		user, err := Resolve(ctx, p, &Identity{Subject: "s1", Email: "alice@example.com", EmailVerified: true}, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), user.ID)
		require.Len(t, fi.identities, 1)

		// The same user is found by the identity even if the email changes.
		user, err = Resolve(ctx, p, &Identity{Subject: "s1", Email: "new@example.com"}, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), user.ID)
		assert.Equal(t, []int64{1}, fi.touched)
	})

	t.Run("unverified email is not linked", func(t *testing.T) {
		withFakeStores(t)
		p := provider(func(p *db.AuthProvider) { p.LinkByEmail = true })
		_, err := Resolve(ctx, p, &Identity{Subject: "s1", Email: "alice@example.com"}, 0)
		assert.Equal(t, CodeNotLinked, code(err))

		p.AutoCreateUser = true
		_, err = Resolve(ctx, p, &Identity{Subject: "s1", Email: "alice@example.com"}, 0)
		assert.Equal(t, CodeEmailTaken, code(err))
	})

	t.Run("auto create", func(t *testing.T) {
		fu, fi := withFakeStores(t)
		p := provider(func(p *db.AuthProvider) { p.AutoCreateUser = true })
		user, err := Resolve(ctx, p, &Identity{Subject: "s9", Email: "bob@example.com", Name: "Bob"}, 0)
		require.NoError(t, err)
		assert.Equal(t, "bob@example.com", user.Email)
		assert.Equal(t, "Bob", user.UserName)
		assert.False(t, user.HasPassword())
		assert.Len(t, fu.users, 3)
		require.Len(t, fi.identities, 1)
		assert.Equal(t, user.ID, fi.identities[0].UserID)

		_, err = Resolve(ctx, p, &Identity{Subject: "s10"}, 0)
		assert.Equal(t, CodeEmailRequired, code(err))
	})

	t.Run("access restrictions", func(t *testing.T) {
		withFakeStores(t)
		p := provider(func(p *db.AuthProvider) {
			p.AutoCreateUser = true
			p.AllowedEmailDomains = []string{"corp.com"}
			p.AllowedGroups = []string{"staff"}
		})
		_, err := Resolve(ctx, p, &Identity{Subject: "s1", Email: "a@corp.com", Groups: []string{"guest"}}, 0)
		assert.Equal(t, CodeAccessDenied, code(err))
		_, err = Resolve(ctx, p, &Identity{Subject: "s1", Email: "a@other.com", Groups: []string{"staff"}}, 0)
		assert.Equal(t, CodeAccessDenied, code(err))
		_, err = Resolve(ctx, p, &Identity{Subject: "s1", Email: "a@corp.com", Groups: []string{"staff"}}, 0)
		require.NoError(t, err)
	})

	t.Run("disabled user", func(t *testing.T) {
		_, fi := withFakeStores(t)
		fi.identities = append(fi.identities, &db.UserIdentity{Model: dbutil.Model{ID: 1}, UserID: 2, ProviderID: 7, Subject: "eve"})
		_, err := Resolve(ctx, provider(nil), &Identity{Subject: "eve"}, 0)
		assert.Equal(t, CodeAccountDisabled, code(err))

		p := provider(func(p *db.AuthProvider) { p.LinkByEmail = true })
		_, err = Resolve(ctx, p, &Identity{Subject: "other", Email: "eve@example.com", EmailVerified: true}, 0)
		assert.Equal(t, CodeAccountDisabled, code(err))
	})

	t.Run("link mode", func(t *testing.T) {
		fu, fi := withFakeStores(t)
		user, err := Resolve(ctx, provider(nil), &Identity{Subject: "gh-1", Email: "whatever@x.com"}, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), user.ID)
		require.Len(t, fi.identities, 1)

		// Binding the same account again succeeds.
		_, err = Resolve(ctx, provider(nil), &Identity{Subject: "gh-1"}, 1)
		require.NoError(t, err)

		// The account is bound to another user.
		bob, _ := fu.Create(ctx, db.CreateUserOptions{Email: "bob@example.com"})
		_, err = Resolve(ctx, provider(nil), &Identity{Subject: "gh-1"}, bob.ID)
		assert.Equal(t, CodeIdentityTaken, code(err))

		// The user has bound another account of the provider.
		_, err = Resolve(ctx, provider(nil), &Identity{Subject: "gh-2"}, 1)
		assert.Equal(t, CodeIdentityTaken, code(err))
	})
}
