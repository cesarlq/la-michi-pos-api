// Package productsapi implementa el recurso /products: CRUD de productos.
// El Service tiene la lógica de negocio; el Handler solo traduce HTTP ↔ Service.
package productsapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

var (
	ErrProductNotFound = errors.New("producto no encontrado")
	ErrInvalidPrice    = errors.New("precio inválido: debe ser un número mayor o igual a cero")
	ErrInvalidCategory = errors.New("categoría inválida: paleta | nieve | agua_fresca | otro")
)

// querier = solo lo que productsapi necesita de la BD (facilita el mock en tests).
type querier interface {
	ListProducts(ctx context.Context, arg db.ListProductsParams) ([]db.Product, error)
	GetProduct(ctx context.Context, id string) (db.Product, error)
	CreateProduct(ctx context.Context, arg db.CreateProductParams) (db.Product, error)
	UpdateProduct(ctx context.Context, arg db.UpdateProductParams) (db.Product, error)
	SetProductActive(ctx context.Context, arg db.SetProductActiveParams) (db.Product, error)
	ListSellableProducts(ctx context.Context, branchID string) ([]db.ListSellableProductsRow, error)
}

// ProductDTO = la forma del producto que devolvemos al front.
// Price como float64 para que el front lo use directo en operaciones aritméticas.
type ProductDTO struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	Active    bool    `json:"active"`
	CreatedAt string  `json:"createdAt"`
}

type ListFilters struct {
	Active   *bool
	Category *string
}

type CreateInput struct {
	Name     string
	Category string
	Price    float64
}

type UpdateInput struct {
	Name     *string
	Category *string
	Price    *float64
}

type Service struct {
	q   querier
	now func() time.Time
}

func NewService(q querier) *Service {
	return &Service{q: q, now: time.Now}
}

// toDTO convierte un db.Product al DTO expuesto al front.
// Price viene de la BD como string (numeric→string en sqlc); se parsea a float64.
func toDTO(p db.Product) (ProductDTO, error) {
	price, err := strconv.ParseFloat(p.Price, 64)
	if err != nil {
		return ProductDTO{}, fmt.Errorf("parseFloat price %q: %w", p.Price, err)
	}
	return ProductDTO{
		ID:        p.ID,
		Name:      p.Name,
		Category:  string(p.Category),
		Price:     price,
		Active:    p.Active,
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func validCategory(s string) bool {
	switch db.ProductCategory(s) {
	case db.ProductCategoryPaleta, db.ProductCategoryNieve,
		db.ProductCategoryAguaFresca, db.ProductCategoryOtro:
		return true
	}
	return false
}

func (s *Service) ListProducts(ctx context.Context, f ListFilters) ([]ProductDTO, error) {
	var cat *db.ProductCategory
	if f.Category != nil {
		if !validCategory(*f.Category) {
			return nil, ErrInvalidCategory
		}
		c := db.ProductCategory(*f.Category)
		cat = &c
	}

	rows, err := s.q.ListProducts(ctx, db.ListProductsParams{
		Active:   f.Active,
		Category: cat,
	})
	if err != nil {
		return nil, err
	}

	out := make([]ProductDTO, 0, len(rows))
	for _, p := range rows {
		dto, err := toDTO(p)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *Service) GetProduct(ctx context.Context, id string) (ProductDTO, error) {
	p, err := s.q.GetProduct(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProductDTO{}, ErrProductNotFound
		}
		return ProductDTO{}, err
	}
	return toDTO(p)
}

func (s *Service) CreateProduct(ctx context.Context, in CreateInput) (ProductDTO, error) {
	if in.Price < 0 {
		return ProductDTO{}, ErrInvalidPrice
	}
	if !validCategory(in.Category) {
		return ProductDTO{}, ErrInvalidCategory
	}

	p, err := s.q.CreateProduct(ctx, db.CreateProductParams{
		Name:     in.Name,
		Category: db.ProductCategory(in.Category),
		Price:    strconv.FormatFloat(in.Price, 'f', 2, 64),
	})
	if err != nil {
		return ProductDTO{}, err
	}
	return toDTO(p)
}

func (s *Service) UpdateProduct(ctx context.Context, id string, in UpdateInput) (ProductDTO, error) {
	if in.Category != nil && !validCategory(*in.Category) {
		return ProductDTO{}, ErrInvalidCategory
	}
	if in.Price != nil && *in.Price < 0 {
		return ProductDTO{}, ErrInvalidPrice
	}

	var dbCat *db.ProductCategory
	if in.Category != nil {
		c := db.ProductCategory(*in.Category)
		dbCat = &c
	}

	var priceStr *string
	if in.Price != nil {
		s := strconv.FormatFloat(*in.Price, 'f', 2, 64)
		priceStr = &s
	}

	p, err := s.q.UpdateProduct(ctx, db.UpdateProductParams{
		ID:       id,
		Name:     in.Name,
		Category: dbCat,
		Price:    priceStr,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProductDTO{}, ErrProductNotFound
		}
		return ProductDTO{}, err
	}
	return toDTO(p)
}

// SellableProductDTO — producto activo con su stock en una sucursal.
// Lo consume el POS del front para mostrar precio y stock disponible.
type SellableProductDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Stock    int     `json:"stock"`
}

func (s *Service) ListSellableProducts(ctx context.Context, branchID string) ([]SellableProductDTO, error) {
	if branchID == "" {
		return nil, errors.New("branch_id es requerido")
	}
	rows, err := s.q.ListSellableProducts(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out := make([]SellableProductDTO, 0, len(rows))
	for _, r := range rows {
		price, err := strconv.ParseFloat(r.Price, 64)
		if err != nil {
			return nil, fmt.Errorf("parseFloat price %q: %w", r.Price, err)
		}
		out = append(out, SellableProductDTO{
			ID:       r.ID,
			Name:     r.Name,
			Category: string(r.Category),
			Price:    price,
			Stock:    int(r.Stock),
		})
	}
	return out, nil
}

// DeleteProduct realiza un soft-delete (active = false).
func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	_, err := s.q.SetProductActive(ctx, db.SetProductActiveParams{ID: id, Active: false})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProductNotFound
		}
		return err
	}
	return nil
}
