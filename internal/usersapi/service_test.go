package usersapi

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

type fakeQuerier struct {
	users   []db.User
	pwCalls int
}

func (f *fakeQuerier) ListUsers(_ context.Context) ([]db.User, error) {
	return f.users, nil
}

func (f *fakeQuerier) GetUserByEmail(_ context.Context, email string) (db.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return db.User{}, pgx.ErrNoRows
}

func (f *fakeQuerier) CreateUser(_ context.Context, arg db.CreateUserParams) (db.User, error) {
	u := db.User{
		ID: "new", Name: arg.Name, Email: arg.Email, PasswordHash: arg.PasswordHash,
		Role: arg.Role, BranchID: arg.BranchID, Active: true,
	}
	f.users = append(f.users, u)
	return u, nil
}

func (f *fakeQuerier) UpdateUser(_ context.Context, arg db.UpdateUserParams) (db.User, error) {
	for i, u := range f.users {
		if u.ID == arg.ID {
			if arg.Name != nil {
				f.users[i].Name = *arg.Name
			}
			if arg.Role != nil {
				f.users[i].Role = *arg.Role
			}
			f.users[i].BranchID = arg.BranchID
			return f.users[i], nil
		}
	}
	return db.User{}, pgx.ErrNoRows
}

func (f *fakeQuerier) UpdateUserPassword(_ context.Context, _ db.UpdateUserPasswordParams) error {
	f.pwCalls++
	return nil
}

func (f *fakeQuerier) SetUserActive(_ context.Context, arg db.SetUserActiveParams) (db.User, error) {
	for i, u := range f.users {
		if u.ID == arg.ID {
			f.users[i].Active = arg.Active
			return f.users[i], nil
		}
	}
	return db.User{}, pgx.ErrNoRows
}

func branchPtr(s string) *string { return &s }

func seed() *fakeQuerier {
	return &fakeQuerier{users: []db.User{
		{ID: "u1", Name: "Dueño", Email: "dueno@lamichi.com", Role: db.UserRoleOwner, Active: true},
	}}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_OK(t *testing.T) {
	f := seed()
	svc := NewService(f)

	u, err := svc.Create(context.Background(), CreateInput{
		Name: "Ana", Email: "ANA@lamichi.com", Password: "secreto123",
		Role: "manager", BranchID: branchPtr("b1"),
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if u.Email != "ana@lamichi.com" {
		t.Errorf("Email = %q, quería normalizado a minúsculas", u.Email)
	}
	// El hash guardado debe validar contra la contraseña original.
	if bcrypt.CompareHashAndPassword([]byte(f.users[len(f.users)-1].PasswordHash), []byte("secreto123")) != nil {
		t.Error("el password no se hasheó correctamente")
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	svc := NewService(seed())
	_, err := svc.Create(context.Background(), CreateInput{
		Name: "Otro", Email: "dueno@lamichi.com", Password: "secreto123", Role: "manager", BranchID: branchPtr("b1"),
	})
	if !errors.Is(err, ErrEmailtaken) {
		t.Fatalf("err = %v, quería ErrEmailtaken", err)
	}
}

func TestCreate_ShortPassword(t *testing.T) {
	svc := NewService(seed())
	_, err := svc.Create(context.Background(), CreateInput{
		Name: "Ana", Email: "ana@x.com", Password: "123", Role: "employee", BranchID: branchPtr("b1"),
	})
	if !errors.Is(err, ErrShortPassword) {
		t.Fatalf("err = %v, quería ErrShortPassword", err)
	}
}

func TestCreate_ManagerWithoutBranch(t *testing.T) {
	svc := NewService(seed())
	_, err := svc.Create(context.Background(), CreateInput{
		Name: "Ana", Email: "ana@x.com", Password: "secreto123", Role: "manager", BranchID: nil,
	})
	if !errors.Is(err, ErrBranchNeeded) {
		t.Fatalf("err = %v, quería ErrBranchNeeded", err)
	}
}

func TestCreate_OwnerIgnoresBranch(t *testing.T) {
	svc := NewService(seed())
	u, err := svc.Create(context.Background(), CreateInput{
		Name: "Dueño 2", Email: "dueno2@x.com", Password: "secreto123", Role: "owner", BranchID: branchPtr("b1"),
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if u.BranchID != nil {
		t.Errorf("BranchID = %v, quería nil (owner es global)", *u.BranchID)
	}
}

func TestCreate_InvalidRole(t *testing.T) {
	svc := NewService(seed())
	_, err := svc.Create(context.Background(), CreateInput{
		Name: "Ana", Email: "ana@x.com", Password: "secreto123", Role: "jefe", BranchID: branchPtr("b1"),
	})
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("err = %v, quería ErrInvalidRole", err)
	}
}

// ── Update / Password / Active ────────────────────────────────────────────────

func TestChangePassword_OK(t *testing.T) {
	f := seed()
	svc := NewService(f)
	if err := svc.ChangePassword(context.Background(), "u1", "nuevopass"); err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if f.pwCalls != 1 {
		t.Errorf("pwCalls = %d, quería 1", f.pwCalls)
	}
}

func TestChangePassword_TooShort(t *testing.T) {
	svc := NewService(seed())
	if err := svc.ChangePassword(context.Background(), "u1", "123"); !errors.Is(err, ErrShortPassword) {
		t.Fatalf("err = %v, quería ErrShortPassword", err)
	}
}

func TestSetActive_NotFound(t *testing.T) {
	svc := NewService(seed())
	_, err := svc.SetActive(context.Background(), "nope", false)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("err = %v, quería ErrUserNotFound", err)
	}
}
