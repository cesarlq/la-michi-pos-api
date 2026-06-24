package authapi

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
	"github.com/cesarlq/la-michi-pos-api/internal/token"
)

// fakeQuerier mockea la BD: devuelve el usuario configurado o ErrNoRows.
type fakeQuerier struct {
	user *db.User
}

func (f fakeQuerier) GetUserByEmail(_ context.Context, _ string) (db.User, error) {
	if f.user == nil {
		return db.User{}, pgx.ErrNoRows
	}
	return *f.user, nil
}

func newUser(t *testing.T, active bool) *db.User {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("michi123"), bcrypt.DefaultCost)
	return &db.User{
		ID:           "user-1",
		Name:         "Dueño",
		Email:        "dueno@lamichi.com",
		PasswordHash: string(hash),
		Role:         db.UserRoleOwner,
		Active:       active,
	}
}

func newService(q querier) *Service {
	return NewService(q, token.NewManager("secreto-de-prueba"))
}

func TestLogin_OK(t *testing.T) {
	svc := newService(fakeQuerier{user: newUser(t, true)})

	res, err := svc.Login(context.Background(), "dueno@lamichi.com", "michi123")
	if err != nil {
		t.Fatalf("Login falló: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("se esperaba un accessToken")
	}
	if res.User.Role != "owner" {
		t.Errorf("Role = %q, quería owner", res.User.Role)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc := newService(fakeQuerier{user: nil})

	_, err := svc.Login(context.Background(), "x@x.com", "michi123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, quería ErrInvalidCredentials", err)
	}
}

func TestLogin_Inactive(t *testing.T) {
	svc := newService(fakeQuerier{user: newUser(t, false)})

	_, err := svc.Login(context.Background(), "dueno@lamichi.com", "michi123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, quería ErrInvalidCredentials", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newService(fakeQuerier{user: newUser(t, true)})

	_, err := svc.Login(context.Background(), "dueno@lamichi.com", "incorrecta")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, quería ErrInvalidCredentials", err)
	}
}
