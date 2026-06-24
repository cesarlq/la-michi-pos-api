package productsapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

// fakeQuerier mockea la BD para tests unitarios.
type fakeQuerier struct {
	products []db.Product
	err      error // si != nil, todas las operaciones devuelven este error
}

func (f *fakeQuerier) ListSellableProducts(_ context.Context, _ string) ([]db.ListSellableProductsRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]db.ListSellableProductsRow, 0, len(f.products))
	for _, p := range f.products {
		if p.Active {
			out = append(out, db.ListSellableProductsRow{
				ID: p.ID, Name: p.Name, Category: p.Category, Price: p.Price, Stock: 10,
			})
		}
	}
	return out, nil
}

func (f *fakeQuerier) ListProducts(_ context.Context, arg db.ListProductsParams) ([]db.Product, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]db.Product, 0)
	for _, p := range f.products {
		if arg.Active != nil && p.Active != *arg.Active {
			continue
		}
		if arg.Category != nil && p.Category != *arg.Category {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeQuerier) GetProduct(_ context.Context, id string) (db.Product, error) {
	if f.err != nil {
		return db.Product{}, f.err
	}
	for _, p := range f.products {
		if p.ID == id {
			return p, nil
		}
	}
	return db.Product{}, pgx.ErrNoRows
}

func (f *fakeQuerier) CreateProduct(_ context.Context, arg db.CreateProductParams) (db.Product, error) {
	if f.err != nil {
		return db.Product{}, f.err
	}
	p := db.Product{
		ID:        "new-id",
		Name:      arg.Name,
		Category:  arg.Category,
		Price:     arg.Price,
		Active:    true,
		CreatedAt: time.Now(),
	}
	f.products = append(f.products, p)
	return p, nil
}

func (f *fakeQuerier) UpdateProduct(_ context.Context, arg db.UpdateProductParams) (db.Product, error) {
	if f.err != nil {
		return db.Product{}, f.err
	}
	for i, p := range f.products {
		if p.ID == arg.ID {
			if arg.Name != nil {
				f.products[i].Name = *arg.Name
			}
			if arg.Category != nil {
				f.products[i].Category = *arg.Category
			}
			if arg.Price != nil {
				f.products[i].Price = *arg.Price
			}
			return f.products[i], nil
		}
	}
	return db.Product{}, pgx.ErrNoRows
}

func (f *fakeQuerier) SetProductActive(_ context.Context, arg db.SetProductActiveParams) (db.Product, error) {
	if f.err != nil {
		return db.Product{}, f.err
	}
	for i, p := range f.products {
		if p.ID == arg.ID {
			f.products[i].Active = arg.Active
			return f.products[i], nil
		}
	}
	return db.Product{}, pgx.ErrNoRows
}

// ── helpers ──────────────────────────────────────────────────────────────────

func seedProducts() []db.Product {
	return []db.Product{
		{ID: "p1", Name: "Paleta de fresa", Category: db.ProductCategoryPaleta, Price: "15.00", Active: true, CreatedAt: time.Now()},
		{ID: "p2", Name: "Nieve de vainilla", Category: db.ProductCategoryNieve, Price: "20.00", Active: true, CreatedAt: time.Now()},
		{ID: "p3", Name: "Paleta inactiva", Category: db.ProductCategoryPaleta, Price: "10.00", Active: false, CreatedAt: time.Now()},
	}
}

func newSvc(q querier) *Service { return NewService(q) }

// ── ListProducts ─────────────────────────────────────────────────────────────

func TestListProducts_All(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	got, err := svc.ListProducts(context.Background(), ListFilters{})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, quería 3", len(got))
	}
}

func TestListProducts_FilterActive(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	active := true
	got, err := svc.ListProducts(context.Background(), ListFilters{Active: &active})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, quería 2 activos", len(got))
	}
}

func TestListProducts_FilterCategory(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	cat := "paleta"
	got, err := svc.ListProducts(context.Background(), ListFilters{Category: &cat})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, quería 2 paletas", len(got))
	}
}

func TestListProducts_InvalidCategory(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	cat := "sopa"
	_, err := svc.ListProducts(context.Background(), ListFilters{Category: &cat})
	if !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("err = %v, quería ErrInvalidCategory", err)
	}
}

// ── GetProduct ───────────────────────────────────────────────────────────────

func TestGetProduct_Found(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	dto, err := svc.GetProduct(context.Background(), "p1")
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if dto.Price != 15.0 {
		t.Errorf("Price = %v, quería 15.0", dto.Price)
	}
}

func TestGetProduct_NotFound(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	_, err := svc.GetProduct(context.Background(), "inexistente")
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("err = %v, quería ErrProductNotFound", err)
	}
}

// ── CreateProduct ────────────────────────────────────────────────────────────

func TestCreateProduct_OK(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	dto, err := svc.CreateProduct(context.Background(), CreateInput{
		Name:     "Agua de jamaica",
		Category: "agua_fresca",
		Price:    12.50,
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if dto.Price != 12.50 {
		t.Errorf("Price = %v, quería 12.50", dto.Price)
	}
	if dto.Category != "agua_fresca" {
		t.Errorf("Category = %q, quería agua_fresca", dto.Category)
	}
}

func TestCreateProduct_NegativePrice(t *testing.T) {
	svc := newSvc(&fakeQuerier{})
	_, err := svc.CreateProduct(context.Background(), CreateInput{
		Name: "Paleta gratis", Category: "paleta", Price: -1,
	})
	if !errors.Is(err, ErrInvalidPrice) {
		t.Fatalf("err = %v, quería ErrInvalidPrice", err)
	}
}

func TestCreateProduct_InvalidCategory(t *testing.T) {
	svc := newSvc(&fakeQuerier{})
	_, err := svc.CreateProduct(context.Background(), CreateInput{
		Name: "X", Category: "torta", Price: 10,
	})
	if !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("err = %v, quería ErrInvalidCategory", err)
	}
}

// ── UpdateProduct ────────────────────────────────────────────────────────────

func TestUpdateProduct_OK(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	newName := "Paleta de fresa XL"
	dto, err := svc.UpdateProduct(context.Background(), "p1", UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if dto.Name != newName {
		t.Errorf("Name = %q, quería %q", dto.Name, newName)
	}
}

func TestUpdateProduct_NotFound(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	_, err := svc.UpdateProduct(context.Background(), "inexistente", UpdateInput{})
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("err = %v, quería ErrProductNotFound", err)
	}
}

func TestUpdateProduct_NegativePrice(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	p := -5.0
	_, err := svc.UpdateProduct(context.Background(), "p1", UpdateInput{Price: &p})
	if !errors.Is(err, ErrInvalidPrice) {
		t.Fatalf("err = %v, quería ErrInvalidPrice", err)
	}
}

// ── DeleteProduct ────────────────────────────────────────────────────────────

func TestDeleteProduct_OK(t *testing.T) {
	q := &fakeQuerier{products: seedProducts()}
	svc := newSvc(q)
	if err := svc.DeleteProduct(context.Background(), "p1"); err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	// verificar que active = false
	for _, p := range q.products {
		if p.ID == "p1" && p.Active {
			t.Error("el producto debería estar inactivo")
		}
	}
}

func TestDeleteProduct_NotFound(t *testing.T) {
	svc := newSvc(&fakeQuerier{products: seedProducts()})
	err := svc.DeleteProduct(context.Background(), "inexistente")
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("err = %v, quería ErrProductNotFound", err)
	}
}
