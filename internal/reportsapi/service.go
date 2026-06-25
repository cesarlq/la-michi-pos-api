// Package reportsapi implementa el recurso /reports: resúmenes de negocio.
// Tres endpoints: ventas del día, top productos, stock crítico.
package reportsapi

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

type querier interface {
	DailySummary(ctx context.Context, arg db.DailySummaryParams) (db.DailySummaryRow, error)
	TopProducts(ctx context.Context, arg db.TopProductsParams) ([]db.TopProductsRow, error)
	CriticalStock(ctx context.Context, branchID *string) ([]db.CriticalStockRow, error)
	SalesTrend(ctx context.Context, arg db.SalesTrendParams) ([]db.SalesTrendRow, error)
}

// DTOs ─────────────────────────────────────────────────────────────────────────

type DailySummaryDTO struct {
	Date         string  `json:"date"`
	SaleCount    int     `json:"saleCount"`
	TotalRevenue float64 `json:"totalRevenue"`
	ItemsSold    int     `json:"itemsSold"`
}

type TopProductDTO struct {
	ProductID    string  `json:"productId"`
	ProductName  string  `json:"productName"`
	Category     string  `json:"category"`
	TotalQty     int     `json:"totalQty"`
	TotalRevenue float64 `json:"totalRevenue"`
}

type SalesTrendPointDTO struct {
	Date         string  `json:"date"` // YYYY-MM-DD
	SaleCount    int     `json:"saleCount"`
	TotalRevenue float64 `json:"totalRevenue"`
}

type CriticalStockDTO struct {
	ProductID    string `json:"productId"`
	ProductName  string `json:"productName"`
	Category     string `json:"category"`
	BranchID     string `json:"branchId"`
	BranchName   string `json:"branchName"`
	CurrentStock int    `json:"currentStock"`
	MinStock     int    `json:"minStock"`
}

// Inputs ───────────────────────────────────────────────────────────────────────

type DailyFilters struct {
	Date     time.Time // día a consultar (cualquier hora; el service calcula inicio/fin del día)
	BranchID *string
}

type TopProductsFilters struct {
	DateFrom time.Time
	DateTo   time.Time
	BranchID *string
	Limit    int
}

type SalesTrendFilters struct {
	DateFrom time.Time
	DateTo   time.Time
	BranchID *string
}

type CriticalStockFilters struct {
	BranchID *string
}

// Service ─────────────────────────────────────────────────────────────────────

type Service struct {
	q   querier
	now func() time.Time
}

func NewService(q querier) *Service {
	return &Service{q: q, now: time.Now}
}

// DailySummary devuelve el resumen de ventas del día solicitado (UTC).
func (s *Service) DailySummary(ctx context.Context, f DailyFilters) (DailySummaryDTO, error) {
	// Truncar al inicio/fin del día en UTC.
	day := f.Date.UTC().Truncate(24 * time.Hour)
	nextDay := day.Add(24 * time.Hour)

	row, err := s.q.DailySummary(ctx, db.DailySummaryParams{
		DateFrom: day,
		DateTo:   nextDay,
		BranchID: f.BranchID,
	})
	if err != nil {
		return DailySummaryDTO{}, fmt.Errorf("DailySummary query: %w", err)
	}

	revenue, err := strconv.ParseFloat(row.TotalRevenue, 64)
	if err != nil {
		return DailySummaryDTO{}, fmt.Errorf("parseFloat revenue: %w", err)
	}

	return DailySummaryDTO{
		Date:         day.Format("2006-01-02"),
		SaleCount:    int(row.SaleCount),
		TotalRevenue: revenue,
		ItemsSold:    int(row.ItemsSold),
	}, nil
}

// Summary devuelve el resumen agregado de ventas en un rango [DateFrom, DateTo).
// DateTo es exclusivo (lo prepara el handler). Sirve para semana/mes/año/personalizado.
func (s *Service) Summary(ctx context.Context, f SalesTrendFilters) (DailySummaryDTO, error) {
	row, err := s.q.DailySummary(ctx, db.DailySummaryParams{
		DateFrom: f.DateFrom,
		DateTo:   f.DateTo,
		BranchID: f.BranchID,
	})
	if err != nil {
		return DailySummaryDTO{}, fmt.Errorf("Summary query: %w", err)
	}

	revenue, err := strconv.ParseFloat(row.TotalRevenue, 64)
	if err != nil {
		return DailySummaryDTO{}, fmt.Errorf("parseFloat revenue: %w", err)
	}

	return DailySummaryDTO{
		Date:         f.DateFrom.Format("2006-01-02"),
		SaleCount:    int(row.SaleCount),
		TotalRevenue: revenue,
		ItemsSold:    int(row.ItemsSold),
	}, nil
}

// TopProducts devuelve los N productos más vendidos en el rango de fechas.
func (s *Service) TopProducts(ctx context.Context, f TopProductsFilters) ([]TopProductDTO, error) {
	limit := f.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := s.q.TopProducts(ctx, db.TopProductsParams{
		DateFrom: f.DateFrom,
		DateTo:   f.DateTo,
		BranchID: f.BranchID,
		Lim:      int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("TopProducts query: %w", err)
	}

	out := make([]TopProductDTO, 0, len(rows))
	for _, r := range rows {
		rev, err := strconv.ParseFloat(r.TotalRevenue, 64)
		if err != nil {
			return nil, fmt.Errorf("parseFloat revenue for %s: %w", r.ProductID, err)
		}
		out = append(out, TopProductDTO{
			ProductID:    r.ProductID,
			ProductName:  r.ProductName,
			Category:     r.Category,
			TotalQty:     int(r.TotalQty),
			TotalRevenue: rev,
		})
	}
	return out, nil
}

// SalesTrend devuelve los ingresos y número de ventas por día en el rango dado.
// Incluye días sin ventas (en cero) para una gráfica de línea sin huecos.
func (s *Service) SalesTrend(ctx context.Context, f SalesTrendFilters) ([]SalesTrendPointDTO, error) {
	rows, err := s.q.SalesTrend(ctx, db.SalesTrendParams{
		DateFrom: f.DateFrom,
		DateTo:   f.DateTo,
		BranchID: f.BranchID,
	})
	if err != nil {
		return nil, fmt.Errorf("SalesTrend query: %w", err)
	}

	out := make([]SalesTrendPointDTO, 0, len(rows))
	for _, r := range rows {
		rev, err := strconv.ParseFloat(r.TotalRevenue, 64)
		if err != nil {
			return nil, fmt.Errorf("parseFloat revenue for %s: %w", r.Day.Format("2006-01-02"), err)
		}
		out = append(out, SalesTrendPointDTO{
			Date:         r.Day.UTC().Format("2006-01-02"),
			SaleCount:    int(r.SaleCount),
			TotalRevenue: rev,
		})
	}
	return out, nil
}

// CriticalStock devuelve los productos con stock ≤ mínimo (por sucursal o globales).
func (s *Service) CriticalStock(ctx context.Context, f CriticalStockFilters) ([]CriticalStockDTO, error) {
	rows, err := s.q.CriticalStock(ctx, f.BranchID)
	if err != nil {
		return nil, fmt.Errorf("CriticalStock query: %w", err)
	}

	out := make([]CriticalStockDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, CriticalStockDTO{
			ProductID:    r.ProductID,
			ProductName:  r.ProductName,
			Category:     r.Category,
			BranchID:     r.BranchID,
			BranchName:   r.BranchName,
			CurrentStock: int(r.CurrentStock),
			MinStock:     int(r.MinStock),
		})
	}
	return out, nil
}
