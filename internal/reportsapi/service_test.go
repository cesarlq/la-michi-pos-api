package reportsapi

import (
	"context"
	"testing"
	"time"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeQuerier struct {
	summaryRow   db.DailySummaryRow
	topRows      []db.TopProductsRow
	criticalRows []db.CriticalStockRow
	trendRows    []db.SalesTrendRow
}

func (f *fakeQuerier) SalesTrend(_ context.Context, _ db.SalesTrendParams) ([]db.SalesTrendRow, error) {
	return f.trendRows, nil
}

func (f *fakeQuerier) DailySummary(_ context.Context, _ db.DailySummaryParams) (db.DailySummaryRow, error) {
	return f.summaryRow, nil
}

func (f *fakeQuerier) TopProducts(_ context.Context, arg db.TopProductsParams) ([]db.TopProductsRow, error) {
	limit := int(arg.Lim)
	if limit > len(f.topRows) {
		limit = len(f.topRows)
	}
	return f.topRows[:limit], nil
}

func (f *fakeQuerier) CriticalStock(_ context.Context, branchID *string) ([]db.CriticalStockRow, error) {
	if branchID == nil {
		return f.criticalRows, nil
	}
	out := make([]db.CriticalStockRow, 0)
	for _, r := range f.criticalRows {
		if r.BranchID == *branchID {
			out = append(out, r)
		}
	}
	return out, nil
}

func newSvc(q querier) *Service { return NewService(q) }

// ── DailySummary ──────────────────────────────────────────────────────────────

func TestDailySummary_OK(t *testing.T) {
	q := &fakeQuerier{
		summaryRow: db.DailySummaryRow{
			SaleCount:    5,
			TotalRevenue: "250.00",
			ItemsSold:    18,
		},
	}
	svc := newSvc(q)

	dto, err := svc.DailySummary(context.Background(), DailyFilters{Date: time.Now()})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if dto.SaleCount != 5 {
		t.Errorf("SaleCount = %d, quería 5", dto.SaleCount)
	}
	if dto.TotalRevenue != 250.0 {
		t.Errorf("TotalRevenue = %v, quería 250.0", dto.TotalRevenue)
	}
	if dto.ItemsSold != 18 {
		t.Errorf("ItemsSold = %d, quería 18", dto.ItemsSold)
	}
	if dto.Date == "" {
		t.Error("Date no debe estar vacío")
	}
}

func TestDailySummary_Zero(t *testing.T) {
	q := &fakeQuerier{
		summaryRow: db.DailySummaryRow{SaleCount: 0, TotalRevenue: "0", ItemsSold: 0},
	}
	svc := newSvc(q)

	dto, err := svc.DailySummary(context.Background(), DailyFilters{Date: time.Now()})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if dto.TotalRevenue != 0 {
		t.Errorf("TotalRevenue = %v, quería 0", dto.TotalRevenue)
	}
}

// ── TopProducts ───────────────────────────────────────────────────────────────

func TestTopProducts_OK(t *testing.T) {
	q := &fakeQuerier{
		topRows: []db.TopProductsRow{
			{ProductID: "p1", ProductName: "Paleta fresa", Category: "paleta", TotalQty: 30, TotalRevenue: "450.00"},
			{ProductID: "p2", ProductName: "Nieve vainilla", Category: "nieve", TotalQty: 20, TotalRevenue: "400.00"},
			{ProductID: "p3", ProductName: "Agua jamaica", Category: "agua_fresca", TotalQty: 15, TotalRevenue: "180.00"},
		},
	}
	svc := newSvc(q)

	now := time.Now()
	dto, err := svc.TopProducts(context.Background(), TopProductsFilters{
		DateFrom: now.AddDate(0, 0, -7),
		DateTo:   now,
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 2 {
		t.Errorf("len = %d, quería 2 (limitado)", len(dto))
	}
	if dto[0].TotalRevenue != 450.0 {
		t.Errorf("TotalRevenue[0] = %v, quería 450.0", dto[0].TotalRevenue)
	}
}

func TestTopProducts_DefaultLimit(t *testing.T) {
	rows := make([]db.TopProductsRow, 15)
	for i := range rows {
		rows[i] = db.TopProductsRow{TotalRevenue: "10.00"}
	}
	q := &fakeQuerier{topRows: rows}
	svc := newSvc(q)

	now := time.Now()
	// limit = 0 → default 10
	dto, err := svc.TopProducts(context.Background(), TopProductsFilters{
		DateFrom: now.AddDate(0, 0, -7),
		DateTo:   now,
		Limit:    0,
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 10 {
		t.Errorf("len = %d, quería 10 (default limit)", len(dto))
	}
}

// ── CriticalStock ─────────────────────────────────────────────────────────────

func TestCriticalStock_All(t *testing.T) {
	q := &fakeQuerier{
		criticalRows: []db.CriticalStockRow{
			{ProductID: "p1", ProductName: "Paleta", Category: "paleta", BranchID: "b1", BranchName: "Centro", CurrentStock: 1, MinStock: 5},
			{ProductID: "p2", ProductName: "Nieve", Category: "nieve", BranchID: "b2", BranchName: "Norte", CurrentStock: 0, MinStock: 3},
		},
	}
	svc := newSvc(q)

	dto, err := svc.CriticalStock(context.Background(), CriticalStockFilters{})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 2 {
		t.Errorf("len = %d, quería 2", len(dto))
	}
}

func TestCriticalStock_FilterBranch(t *testing.T) {
	branch := "b1"
	q := &fakeQuerier{
		criticalRows: []db.CriticalStockRow{
			{ProductID: "p1", BranchID: "b1", BranchName: "Centro", CurrentStock: 1, MinStock: 5},
			{ProductID: "p2", BranchID: "b2", BranchName: "Norte", CurrentStock: 0, MinStock: 3},
		},
	}
	svc := newSvc(q)

	dto, err := svc.CriticalStock(context.Background(), CriticalStockFilters{BranchID: &branch})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 1 {
		t.Errorf("len = %d, quería 1 (solo Centro)", len(dto))
	}
	if dto[0].BranchName != "Centro" {
		t.Errorf("BranchName = %q, quería Centro", dto[0].BranchName)
	}
}

func TestCriticalStock_Empty(t *testing.T) {
	q := &fakeQuerier{criticalRows: nil}
	svc := newSvc(q)

	dto, err := svc.CriticalStock(context.Background(), CriticalStockFilters{})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 0 {
		t.Errorf("len = %d, quería 0 (sin stock crítico)", len(dto))
	}
}

// ── Summary (rango) ─────────────────────────────────────────────────────────

func TestSummary_Range(t *testing.T) {
	q := &fakeQuerier{
		summaryRow: db.DailySummaryRow{SaleCount: 12, TotalRevenue: "1500.50", ItemsSold: 40},
	}
	svc := newSvc(q)

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	dto, err := svc.Summary(context.Background(), SalesTrendFilters{DateFrom: from, DateTo: to})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if dto.SaleCount != 12 {
		t.Errorf("SaleCount = %d, quería 12", dto.SaleCount)
	}
	if dto.TotalRevenue != 1500.5 {
		t.Errorf("TotalRevenue = %v, quería 1500.5", dto.TotalRevenue)
	}
	if dto.ItemsSold != 40 {
		t.Errorf("ItemsSold = %d, quería 40", dto.ItemsSold)
	}
}

// ── SalesTrend ────────────────────────────────────────────────────────────────

func TestSalesTrend_OK(t *testing.T) {
	day1 := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	q := &fakeQuerier{
		trendRows: []db.SalesTrendRow{
			{Day: day1, SaleCount: 3, TotalRevenue: "150.50"},
			{Day: day2, SaleCount: 0, TotalRevenue: "0"}, // día sin ventas → cero
		},
	}
	svc := newSvc(q)

	dto, err := svc.SalesTrend(context.Background(), SalesTrendFilters{
		DateFrom: day1, DateTo: day2,
	})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 2 {
		t.Fatalf("len = %d, quería 2", len(dto))
	}
	if dto[0].Date != "2026-06-20" {
		t.Errorf("Date[0] = %q, quería 2026-06-20", dto[0].Date)
	}
	if dto[0].TotalRevenue != 150.5 {
		t.Errorf("TotalRevenue[0] = %v, quería 150.5", dto[0].TotalRevenue)
	}
	if dto[1].SaleCount != 0 || dto[1].TotalRevenue != 0 {
		t.Errorf("día sin ventas debe ser cero, fue count=%d rev=%v", dto[1].SaleCount, dto[1].TotalRevenue)
	}
}

func TestSalesTrend_Empty(t *testing.T) {
	q := &fakeQuerier{trendRows: nil}
	svc := newSvc(q)

	dto, err := svc.SalesTrend(context.Background(), SalesTrendFilters{})
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if len(dto) != 0 {
		t.Errorf("len = %d, quería 0", len(dto))
	}
}
