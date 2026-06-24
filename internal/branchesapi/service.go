// Package branchesapi implementa el recurso /branches.
// GET (lista/detalle) para cualquier autenticado; mutaciones solo owner.
package branchesapi

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

var (
	ErrBranchNotFound = errors.New("sucursal no encontrada")
	ErrEmptyName      = errors.New("el nombre de la sucursal es obligatorio")
)

type querier interface {
	ListBranches(ctx context.Context) ([]db.Branch, error)
	ListAllBranches(ctx context.Context) ([]db.Branch, error)
	GetBranch(ctx context.Context, id string) (db.Branch, error)
	CreateBranch(ctx context.Context, arg db.CreateBranchParams) (db.Branch, error)
	UpdateBranch(ctx context.Context, arg db.UpdateBranchParams) (db.Branch, error)
	SetBranchActive(ctx context.Context, arg db.SetBranchActiveParams) (db.Branch, error)
}

// BranchDTO — forma que devolvemos al front.
type BranchDTO struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Address *string `json:"address"`
	Phone   *string `json:"phone"`
	Active  bool    `json:"active"`
}

func toDTO(b db.Branch) BranchDTO {
	return BranchDTO{ID: b.ID, Name: b.Name, Address: b.Address, Phone: b.Phone, Active: b.Active}
}

type CreateInput struct {
	Name    string
	Address *string
	Phone   *string
}

type UpdateInput struct {
	Name    *string
	Address *string
	Phone   *string
}

type Service struct {
	q querier
}

func NewService(q querier) *Service {
	return &Service{q: q}
}

// List devuelve sucursales. all=true incluye inactivas (gestión del owner).
func (s *Service) List(ctx context.Context, all bool) ([]BranchDTO, error) {
	var rows []db.Branch
	var err error
	if all {
		rows, err = s.q.ListAllBranches(ctx)
	} else {
		rows, err = s.q.ListBranches(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]BranchDTO, 0, len(rows))
	for _, b := range rows {
		out = append(out, toDTO(b))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (BranchDTO, error) {
	b, err := s.q.GetBranch(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BranchDTO{}, ErrBranchNotFound
		}
		return BranchDTO{}, err
	}
	return toDTO(b), nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (BranchDTO, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return BranchDTO{}, ErrEmptyName
	}
	b, err := s.q.CreateBranch(ctx, db.CreateBranchParams{
		Name:    name,
		Address: in.Address,
		Phone:   in.Phone,
	})
	if err != nil {
		return BranchDTO{}, err
	}
	return toDTO(b), nil
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (BranchDTO, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return BranchDTO{}, ErrEmptyName
		}
		in.Name = &trimmed
	}
	b, err := s.q.UpdateBranch(ctx, db.UpdateBranchParams{
		ID:      id,
		Name:    in.Name,
		Address: in.Address,
		Phone:   in.Phone,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BranchDTO{}, ErrBranchNotFound
		}
		return BranchDTO{}, err
	}
	return toDTO(b), nil
}

func (s *Service) SetActive(ctx context.Context, id string, active bool) (BranchDTO, error) {
	b, err := s.q.SetBranchActive(ctx, db.SetBranchActiveParams{ID: id, Active: active})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BranchDTO{}, ErrBranchNotFound
		}
		return BranchDTO{}, err
	}
	return toDTO(b), nil
}
