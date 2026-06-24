package salesapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeQuerier struct {
	products  map[string]db.Product
	inventory map[string]db.Inventory // key: productID+":"+branchID
	sales     map[string]db.Sale
	saleItems map[string][]db.SaleItem // key: saleID
}

func newFakeQuerier() *fakeQuerier {
	return &fakeQuerier{
		products:  make(map[string]db.Product),
		inventory: make(map[string]db.Inventory),
		sales:     make(map[string]db.Sale),
		saleItems: make(map[string][]db.SaleItem),
	}
}

func invKey(productID, branchID string) string { return productID + ":" + branchID }

func (f *fakeQuerier) GetProduct(_ context.Context, id string) (db.Product, error) {
	p, ok := f.products[id]
	if !ok {
		return db.Product{}, pgx.ErrNoRows
	}
	return p, nil
}

func (f *fakeQuerier) GetInventoryByProductBranch(_ context.Context, arg db.GetInventoryByProductBranchParams) (db.Inventory, error) {
	inv, ok := f.inventory[invKey(arg.ProductID, arg.BranchID)]
	if !ok {
		return db.Inventory{}, pgx.ErrNoRows
	}
	return inv, nil
}

func (f *fakeQuerier) DecrementInventory(_ context.Context, arg db.DecrementInventoryParams) (db.Inventory, error) {
	key := invKey(arg.ProductID, arg.BranchID)
	inv, ok := f.inventory[key]
	if !ok || inv.CurrentStock < arg.Qty {
		return db.Inventory{}, pgx.ErrNoRows
	}
	inv.CurrentStock -= arg.Qty
	f.inventory[key] = inv
	return inv, nil
}

func (f *fakeQuerier) CreateSale(_ context.Context, arg db.CreateSaleParams) (db.Sale, error) {
	s := db.Sale{
		ID:            "sale-1",
		BranchID:      arg.BranchID,
		UserID:        arg.UserID,
		Total:         arg.Total,
		PaymentMethod: arg.PaymentMethod,
		Status:        db.SaleStatusCompleted,
		CreatedAt:     time.Now(),
	}
	f.sales[s.ID] = s
	return s, nil
}

func (f *fakeQuerier) CreateSaleItem(_ context.Context, arg db.CreateSaleItemParams) (db.SaleItem, error) {
	item := db.SaleItem{
		ID:          "item-" + arg.ProductID,
		SaleID:      arg.SaleID,
		ProductID:   arg.ProductID,
		ProductName: arg.ProductName,
		UnitPrice:   arg.UnitPrice,
		Quantity:    arg.Quantity,
		Subtotal:    arg.Subtotal,
	}
	f.saleItems[arg.SaleID] = append(f.saleItems[arg.SaleID], item)
	return item, nil
}

func (f *fakeQuerier) GetSale(_ context.Context, id string) (db.Sale, error) {
	s, ok := f.sales[id]
	if !ok {
		return db.Sale{}, pgx.ErrNoRows
	}
	return s, nil
}

func (f *fakeQuerier) GetSaleItems(_ context.Context, saleID string) ([]db.SaleItem, error) {
	return f.saleItems[saleID], nil
}

func (f *fakeQuerier) ListSales(_ context.Context, arg db.ListSalesParams) ([]db.Sale, error) {
	out := make([]db.Sale, 0)
	for _, s := range f.sales {
		if arg.BranchID != nil && s.BranchID != *arg.BranchID {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// fakeTxRunner ejecuta fn directamente sin BD (útil para unit tests).
func fakeTxRunner(q querier) txRunner {
	return func(ctx context.Context, fn func(querier) error) error {
		return fn(q)
	}
}

// ── seed helpers ──────────────────────────────────────────────────────────────

const (
	branchCentro = "branch-centro"
	productFresa = "prod-fresa"
	userEmployee = "user-emp"
)

func seedForSale(t *testing.T) *fakeQuerier {
	t.Helper()
	q := newFakeQuerier()
	q.products[productFresa] = db.Product{
		ID:       productFresa,
		Name:     "Paleta de fresa",
		Category: db.ProductCategoryPaleta,
		Price:    "15.00",
		Active:   true,
	}
	q.inventory[invKey(productFresa, branchCentro)] = db.Inventory{
		ID:           "inv-1",
		ProductID:    productFresa,
		BranchID:     branchCentro,
		CurrentStock: 10,
		MinStock:     2,
	}
	return q
}

func newSvc(q *fakeQuerier) *Service {
	return NewService(q, fakeTxRunner(q))
}

// ── CreateSale ────────────────────────────────────────────────────────────────

func TestCreateSale_OK(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	dto, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	// total = 15.00 * 2 = 30.00
	if dto.Total != 30.0 {
		t.Errorf("Total = %v, quería 30.0", dto.Total)
	}
	if len(dto.Items) != 1 {
		t.Errorf("Items len = %d, quería 1", len(dto.Items))
	}
	// verificar que el stock bajó
	inv := q.inventory[invKey(productFresa, branchCentro)]
	if inv.CurrentStock != 8 {
		t.Errorf("CurrentStock = %d, quería 8 (10-2)", inv.CurrentStock)
	}
}

func TestCreateSale_InsufficientStock(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 20}}, // más que el stock
	})
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("err = %v, quería ErrInsufficientStock", err)
	}
}

func TestCreateSale_ProductUnavailable(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: "no-existe", Quantity: 1}},
	})
	if !errors.Is(err, ErrProductUnavailable) {
		t.Fatalf("err = %v, quería ErrProductUnavailable", err)
	}
}

func TestCreateSale_InactiveProduct(t *testing.T) {
	q := seedForSale(t)
	q.products["prod-inactivo"] = db.Product{ID: "prod-inactivo", Name: "X", Price: "10.00", Active: false}
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: "prod-inactivo", Quantity: 1}},
	})
	if !errors.Is(err, ErrProductUnavailable) {
		t.Fatalf("err = %v, quería ErrProductUnavailable", err)
	}
}

func TestCreateSale_InvalidPayment(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "bitcoin",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 1}},
	})
	if !errors.Is(err, ErrInvalidPayment) {
		t.Fatalf("err = %v, quería ErrInvalidPayment", err)
	}
}

func TestCreateSale_EmptyItems(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{},
	})
	if !errors.Is(err, ErrEmptyItems) {
		t.Fatalf("err = %v, quería ErrEmptyItems", err)
	}
}

func TestCreateSale_NoBranch(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      "",
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 1}},
	})
	if !errors.Is(err, ErrNoBranchID) {
		t.Fatalf("err = %v, quería ErrNoBranchID", err)
	}
}

func TestCreateSale_ZeroQty(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	_, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 0}},
	})
	if !errors.Is(err, ErrInvalidQty) {
		t.Fatalf("err = %v, quería ErrInvalidQty", err)
	}
}

// ── GetSale ───────────────────────────────────────────────────────────────────

func TestGetSale_NotFound(t *testing.T) {
	q := newFakeQuerier()
	svc := newSvc(q)

	_, err := svc.GetSale(context.Background(), "inexistente")
	if !errors.Is(err, ErrSaleNotFound) {
		t.Fatalf("err = %v, quería ErrSaleNotFound", err)
	}
}

func TestGetSale_FoundWithItems(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	// Crear venta primero
	dto, err := svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "card",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 3}},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	got, err := svc.GetSale(context.Background(), dto.ID)
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if got.Total != 45.0 { // 15 * 3
		t.Errorf("Total = %v, quería 45.0", got.Total)
	}
	if len(got.Items) != 1 {
		t.Errorf("Items len = %d, quería 1", len(got.Items))
	}
}

// ── ListSales ─────────────────────────────────────────────────────────────────

func TestListSales_FilterBranch(t *testing.T) {
	q := seedForSale(t)
	svc := newSvc(q)

	// Crear una venta en Centro
	_, _ = svc.CreateSale(context.Background(), CreateInput{
		BranchID:      branchCentro,
		UserID:        userEmployee,
		PaymentMethod: "cash",
		Items:         []ItemInput{{ProductID: productFresa, Quantity: 1}},
	})

	branch := branchCentro
	list, err := svc.ListSales(context.Background(), ListFilters{BranchID: &branch})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len = %d, quería 1", len(list))
	}
}
