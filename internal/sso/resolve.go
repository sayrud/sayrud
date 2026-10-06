package sso

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

// Resolve maps the third-party account to a Sayrud user: a non-zero linkUserID binds it to that user, otherwise it looks up the bound identity, links by email or creates a user in order.
func Resolve(ctx context.Context, p *db.AuthProvider, id *Identity, linkUserID int64) (*db.User, error) {
	if !CheckAccess(id, p.AllowedEmailDomains, p.AllowedGroups) {
		return nil, Errorf(CodeAccessDenied, errors.Errorf("email %q or groups %v not allowed", id.Email, id.Groups))
	}

	identity, err := db.UserIdentities.GetByProviderSubject(ctx, p.ID, id.Subject)
	if err != nil && !errors.Is(err, db.ErrUserIdentityNotFound) {
		return nil, errors.Wrap(err, "get identity")
	}

	if linkUserID != 0 {
		return link(ctx, p, id, identity, linkUserID)
	}

	var user *db.User
	switch {
	case identity != nil:
		if user, err = db.Users.GetByID(ctx, identity.UserID); err != nil {
			return nil, errors.Wrap(err, "get user of identity")
		}
	default:
		if user, err = linkOrCreate(ctx, p, id); err != nil {
			return nil, err
		}
	}
	if user.Disabled() {
		return nil, Errorf(CodeAccountDisabled, nil)
	}
	if identity != nil {
		if err := db.UserIdentities.Touch(ctx, identity.ID, id.Email); err != nil {
			return nil, errors.Wrap(err, "touch identity")
		}
	}
	return user, nil
}

func link(ctx context.Context, p *db.AuthProvider, id *Identity, identity *db.UserIdentity, userID int64) (*db.User, error) {
	if identity != nil && identity.UserID != userID {
		return nil, Errorf(CodeIdentityTaken, nil)
	}
	user, err := db.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "get user")
	}
	if identity != nil {
		if err := db.UserIdentities.Touch(ctx, identity.ID, id.Email); err != nil {
			return nil, errors.Wrap(err, "touch identity")
		}
		return user, nil
	}
	if err := bind(ctx, p, id, user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

// linkOrCreate handles an unbound account: it links the existing user by the verified email or creates a user.
func linkOrCreate(ctx context.Context, p *db.AuthProvider, id *Identity) (*db.User, error) {
	var existing *db.User
	if id.Email != "" {
		user, err := db.Users.GetByEmail(ctx, id.Email)
		if err != nil && !errors.Is(err, db.ErrUserNotFound) {
			return nil, errors.Wrap(err, "get user by email")
		}
		existing = user
	}

	if existing != nil && p.LinkByEmail && id.EmailVerified {
		if existing.Disabled() {
			return nil, Errorf(CodeAccountDisabled, nil)
		}
		if err := bind(ctx, p, id, existing.ID); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !p.AutoCreateUser {
		return nil, Errorf(CodeNotLinked, nil)
	}
	if id.Email == "" {
		return nil, Errorf(CodeEmailRequired, nil)
	}
	if existing != nil {
		return nil, Errorf(CodeEmailTaken, nil)
	}

	options := db.CreateUserOptions{Email: id.Email, UserName: DeriveUserName(id)}
	user, err := db.Users.ClaimLegacyDefault(ctx, options)
	if errors.Is(err, db.ErrUserNotFound) {
		user, err = db.Users.Create(ctx, options)
	}
	if err != nil {
		if errors.Is(err, db.ErrUserAlreadyExisted) {
			return nil, Errorf(CodeEmailTaken, nil)
		}
		return nil, errors.Wrap(err, "create user")
	}
	if err := db.Users.EnsureAdmin(ctx); err != nil {
		return nil, errors.Wrap(err, "ensure admin")
	}
	if user, err = db.Users.GetByID(ctx, user.ID); err != nil {
		return nil, errors.Wrap(err, "reload user")
	}
	if err := bind(ctx, p, id, user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

func bind(ctx context.Context, p *db.AuthProvider, id *Identity, userID int64) error {
	err := db.UserIdentities.Create(ctx, &db.UserIdentity{UserID: userID, ProviderID: p.ID, Subject: id.Subject, Email: id.Email})
	if errors.Is(err, db.ErrUserIdentityTaken) {
		return Errorf(CodeIdentityTaken, nil)
	}
	return errors.Wrap(err, "create identity")
}
