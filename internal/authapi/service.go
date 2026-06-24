// Package authapi implementa el recurso /auth: login y perfil del usuario.
// El Service tiene la lógica de negocio (validar credenciales, emitir token);
// el handler solo traduce HTTP <-> Service.
package authapi

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
	"github.com/cesarlq/la-michi-pos-api/internal/token"
)

// ErrInvalidCredentials: mismo error para "no existe", "inactivo" y "password
// mal" → no revelamos al atacante qué parte falló.
var ErrInvalidCredentials = errors.New("credenciales inválidas")

// querier = solo lo que authapi necesita de la BD (facilita el mock en tests).
type querier interface {
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
}

type Service struct {
	q     querier
	token *token.Manager
	now   func() time.Time // inyectable para tests
}

func NewService(q querier, tm *token.Manager) *Service {
	return &Service{q: q, token: tm, now: time.Now}
}

// UserDTO = la forma del usuario que devolvemos al front (sin password_hash).
type UserDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	BranchID *string `json:"branchId"`
}

type LoginResult struct {
	AccessToken string  `json:"accessToken"`
	User        UserDTO `json:"user"`
}

// Login valida credenciales contra la BD y, si son correctas, emite un JWT.
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}
	if !user.Active {
		return LoginResult{}, ErrInvalidCredentials
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	role := string(user.Role)
	accessToken, err := s.token.Sign(user.ID, user.Name, user.Email, role, user.BranchID, s.now())
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken: accessToken,
		User: UserDTO{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Role:     role,
			BranchID: user.BranchID,
		},
	}, nil
}
