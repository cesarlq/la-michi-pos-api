package inventoryapi

import (
	"context"
	"errors"
	"testing"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

// fakeQuerier simula el inventario en memoria.
type fakeQuerier struct {
	rows  []db.ListInventoryByBranchRow
	stock map[string]db.Inventory // key: productID+":"+branchID
}

func newFake() *fakeQuerier {
	return &fakeQuerier{stock: make(map[string]db.Inventory)}
}

func key(p, b string) string { return p + ":" + b }

func (f *fakeQuerier) ListInventoryByBranch(_ context.Context, _ string) ([]db.ListInventoryByBranchRow, error) {
	return f.rows, nil
}

func (f *fakeQuerier) RestockInventory(_ context.Context, arg db.RestockInventoryParams) (db.Inventory, error) {
	k := key(arg.ProductID, arg.BranchID)
	inv := f.stock[k] // zero value si no existe (UPSERT crea)
	inv.ProductID = arg.ProductID
	inv.BranchID = arg.BranchID
	inv.CurrentStock += arg.Qty
	f.stock[k] = inv
	return inv, nil
}

func (f *fakeQuerier) SetMinStock(_ context.Context, arg db.SetMinStockParams) (db.Inventory, error) {
	k := key(arg.ProductID, arg.BranchID)
	inv := f.stock[k]
	inv.ProductID = arg.ProductID
	inv.BranchID = arg.BranchID
	inv.MinStock = arg.MinStock
	f.stock[k] = inv
	return inv, nil
}

const (
	prod   = "p1"
	branch = "b1"
)

// ── ListByBranch ──────────────────────────────────────────────────────────────

func TestListByBranch_OK(t *testing.T) {
	f := newFake()
	f.rows = []db.ListInventoryByBranchRow{
		{ProductID: "p1", ProductName: "Paleta", Category: db.ProductCategoryPaleta, Price: "15.00", CurrentStock: 8, MinStock: 2},
	}
	svc := NewService(f)

	got, err := svc.ListByBranch(context.Background(), branch)
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(got) != 1 || got[0].Price != 15.0 || got[0].CurrentStock != 8 {
		t.Errorf("row mal mapeado: %+v", got)
	}
}

func TestListByBranch_NoBranch(t *testing.T) {
	svc := NewService(newFake())
	_, err := svc.ListByBranch(context.Background(), "")
	if !errors.Is(err, ErrNoBranch) {
		t.Fatalf("err = %v, quería ErrNoBranch", err)
	}
}

// ── Restock ───────────────────────────────────────────────────────────────────

func TestRestock_OK(t *testing.T) {
	f := newFake()
	f.stock[key(prod, branch)] = db.Inventory{ProductID: prod, BranchID: branch, CurrentStock: 5, MinStock: 2}
	svc := NewService(f)

	item, err := svc.Restock(context.Background(), prod, branch, 10)
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if item.CurrentStock != 15 {
		t.Errorf("CurrentStock = %d, quería 15 (5+10)", item.CurrentStock)
	}
}

func TestRestock_CreatesRowWhenMissing(t *testing.T) {
	f := newFake() // sin renglón previo → UPSERT lo crea
	svc := NewService(f)

	item, err := svc.Restock(context.Background(), prod, branch, 7)
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if item.CurrentStock != 7 {
		t.Errorf("CurrentStock = %d, quería 7", item.CurrentStock)
	}
}

func TestRestock_InvalidQty(t *testing.T) {
	svc := NewService(newFake())
	_, err := svc.Restock(context.Background(), prod, branch, 0)
	if !errors.Is(err, ErrInvalidQty) {
		t.Fatalf("err = %v, quería ErrInvalidQty", err)
	}
}

func TestRestock_NoBranch(t *testing.T) {
	svc := NewService(newFake())
	_, err := svc.Restock(context.Background(), prod, "", 5)
	if !errors.Is(err, ErrNoBranch) {
		t.Fatalf("err = %v, quería ErrNoBranch", err)
	}
}

func TestRestock_NoProduct(t *testing.T) {
	svc := NewService(newFake())
	_, err := svc.Restock(context.Background(), "", branch, 5)
	if !errors.Is(err, ErrNoProduct) {
		t.Fatalf("err = %v, quería ErrNoProduct", err)
	}
}

// ── SetMinStock ───────────────────────────────────────────────────────────────

func TestSetMinStock_OK(t *testing.T) {
	f := newFake()
	f.stock[key(prod, branch)] = db.Inventory{ProductID: prod, BranchID: branch, CurrentStock: 5, MinStock: 2}
	svc := NewService(f)

	item, err := svc.SetMinStock(context.Background(), prod, branch, 10)
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if item.MinStock != 10 {
		t.Errorf("MinStock = %d, quería 10", item.MinStock)
	}
}

func TestSetMinStock_Negative(t *testing.T) {
	svc := NewService(newFake())
	_, err := svc.SetMinStock(context.Background(), prod, branch, -1)
	if !errors.Is(err, ErrInvalidMin) {
		t.Fatalf("err = %v, quería ErrInvalidMin", err)
	}
}
