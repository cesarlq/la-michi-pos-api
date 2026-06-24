// Package usersapi implementa el recurso /users: gestión de empleados (solo owner).
// El password se hashea con bcrypt en el servidor; el hash NUNCA se expone al front.
package usersapi

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

var (
	ErrUserNotFound  = errors.New("usuario no encontrado")
	ErrEmailtaken    = errors.New("ya existe un usuario con ese correo")
	ErrInvalidEmail  = errors.New("correo inválido")
	ErrShortPassword = errors.New("la contraseña debe tener al menos 6 caracteres")
	ErrInvalidRole   = errors.New("rol inválido: owner | manager | employee")
	ErrBranchNeeded  = errors.New("encargado y empleado requieren una sucursal")
	ErrEmptyName     = errors.New("el nombre es obligatorio")
)

type querier interface {
	ListUsers(ctx context.Context) ([]db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.User, error)
	UpdateUserPassword(ctx context.Context, arg db.UpdateUserPasswordParams) error
	SetUserActive(ctx context.Context, arg db.SetUserActiveParams) (db.User, error)
}

// UserDTO — sin password_hash, jamás.
type UserDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	BranchID *string `json:"branchId"`
	Active   bool    `json:"active"`
}

func toDTO(u db.User) UserDTO {
	return UserDTO{
		ID: u.ID, Name: u.Name, Email: u.Email,
		Role: string(u.Role), BranchID: u.BranchID, Active: u.Active,
	}
}

type CreateInput struct {
	Name     string
	Email    string
	Password string
	Role     string
	BranchID *string
}

type UpdateInput struct {
	Name     *string
	Role     *string
	BranchID *string // se aplica tal cual (nil = owner sin sucursal)
}

type Service struct {
	q querier
}

func NewService(q querier) *Service {
	return &Service{q: q}
}

func validRole(s string) bool {
	switch db.UserRole(s) {
	case db.UserRoleOwner, db.UserRoleManager, db.UserRoleEmployee:
		return true
	}
	return false
}

// resolveBranch aplica la regla: owner sin sucursal; manager/employee requieren una.
func resolveBranch(role string, branchID *string) (*string, error) {
	if role == string(db.UserRoleOwner) {
		return nil, nil // el owner es global, sin sucursal
	}
	if branchID == nil || *branchID == "" {
		return nil, ErrBranchNeeded
	}
	return branchID, nil
}

func (s *Service) List(ctx context.Context) ([]UserDTO, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]UserDTO, 0, len(rows))
	for _, u := range rows {
		out = append(out, toDTO(u))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (UserDTO, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(strings.ToLower(in.Email))

	if name == "" {
		return UserDTO{}, ErrEmptyName
	}
	if email == "" || !strings.Contains(email, "@") {
		return UserDTO{}, ErrInvalidEmail
	}
	if len(in.Password) < 6 {
		return UserDTO{}, ErrShortPassword
	}
	if !validRole(in.Role) {
		return UserDTO{}, ErrInvalidRole
	}
	branchID, err := resolveBranch(in.Role, in.BranchID)
	if err != nil {
		return UserDTO{}, err
	}

	// Email único: chequeo previo para dar un mensaje claro.
	if _, err := s.q.GetUserByEmail(ctx, email); err == nil {
		return UserDTO{}, ErrEmailtaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return UserDTO{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserDTO{}, err
	}

	u, err := s.q.CreateUser(ctx, db.CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         db.UserRole(in.Role),
		BranchID:     branchID,
	})
	if err != nil {
		return UserDTO{}, err
	}
	return toDTO(u), nil
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (UserDTO, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return UserDTO{}, ErrEmptyName
		}
		in.Name = &trimmed
	}

	var role *db.UserRole
	branchID := in.BranchID
	if in.Role != nil {
		if !validRole(*in.Role) {
			return UserDTO{}, ErrInvalidRole
		}
		r := db.UserRole(*in.Role)
		role = &r
		// Si cambia el rol, re-evaluar la coherencia de la sucursal.
		resolved, err := resolveBranch(*in.Role, in.BranchID)
		if err != nil {
			return UserDTO{}, err
		}
		branchID = resolved
	}

	u, err := s.q.UpdateUser(ctx, db.UpdateUserParams{
		ID:       id,
		Name:     in.Name,
		Role:     role,
		BranchID: branchID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserDTO{}, ErrUserNotFound
		}
		return UserDTO{}, err
	}
	return toDTO(u), nil
}

func (s *Service) ChangePassword(ctx context.Context, id, password string) error {
	if len(password) < 6 {
		return ErrShortPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: id, PasswordHash: string(hash)})
}

func (s *Service) SetActive(ctx context.Context, id string, active bool) (UserDTO, error) {
	u, err := s.q.SetUserActive(ctx, db.SetUserActiveParams{ID: id, Active: active})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserDTO{}, ErrUserNotFound
		}
		return UserDTO{}, err
	}
	return toDTO(u), nil
}
