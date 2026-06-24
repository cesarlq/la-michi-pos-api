// Package salesapi implementa el recurso /sales: registro y consulta de ventas.
// El flujo de CreateSale es una transacción atómica: sale + sale_items + decremento de inventory.
// El precio siempre viene del servidor (nunca del cliente) para evitar manipulaciones.
package salesapi

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
	ErrSaleNotFound       = errors.New("venta no encontrada")
	ErrInsufficientStock  = errors.New("stock insuficiente para uno o más productos")
	ErrProductUnavailable = errors.New("uno o más productos no existen o están inactivos")
	ErrInvalidPayment     = errors.New("método de pago inválido: cash | card | transfer")
	ErrNoBranchID         = errors.New("se requiere sucursal para registrar una venta")
	ErrEmptyItems         = errors.New("la venta debe tener al menos un producto")
	ErrInvalidQty         = errors.New("la cantidad de cada producto debe ser mayor a cero")
)

// Querier = interfaz mínima que salesapi necesita (exportada para que cmd/sales pueda usar db.Queries).
type Querier interface {
	GetProduct(ctx context.Context, id string) (db.Product, error)
	GetInventoryByProductBranch(ctx context.Context, arg db.GetInventoryByProductBranchParams) (db.Inventory, error)
	DecrementInventory(ctx context.Context, arg db.DecrementInventoryParams) (db.Inventory, error)
	CreateSale(ctx context.Context, arg db.CreateSaleParams) (db.Sale, error)
	CreateSaleItem(ctx context.Context, arg db.CreateSaleItemParams) (db.SaleItem, error)
	GetSale(ctx context.Context, id string) (db.Sale, error)
	GetSaleItems(ctx context.Context, saleID string) ([]db.SaleItem, error)
	ListSales(ctx context.Context, arg db.ListSalesParams) ([]db.Sale, error)
}

// TxRunner ejecuta fn dentro de una transacción Postgres.
// Abstracción que permite mockear la TX en tests unitarios sin necesitar una BD real.
type TxRunner func(ctx context.Context, fn func(q Querier) error) error

// alias interno para no tener que cambiar todos los usos en el paquete
type querier = Querier
type txRunner = TxRunner

// DTOs ─────────────────────────────────────────────────────────────────────────

type SaleItemDTO struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	UnitPrice   float64 `json:"unitPrice"`
	Quantity    int     `json:"quantity"`
	Subtotal    float64 `json:"subtotal"`
}

type SaleDTO struct {
	ID            string        `json:"id"`
	BranchID      string        `json:"branchId"`
	UserID        string        `json:"userId"`
	Total         float64       `json:"total"`
	PaymentMethod string        `json:"paymentMethod"`
	Status        string        `json:"status"`
	CreatedAt     string        `json:"createdAt"`
	Items         []SaleItemDTO `json:"items"`
}

// Inputs ───────────────────────────────────────────────────────────────────────

type ItemInput struct {
	ProductID string
	Quantity  int
}

type CreateInput struct {
	BranchID      string
	UserID        string
	PaymentMethod string
	Items         []ItemInput
}

type ListFilters struct {
	BranchID *string // nil = todas las sucursales (solo owner)
	Limit    int
}

// Service ─────────────────────────────────────────────────────────────────────

type Service struct {
	q   querier
	tx  txRunner
	now func() time.Time
}

func NewService(q querier, tx txRunner) *Service {
	return &Service{q: q, tx: tx, now: time.Now}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseAmount(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func fmtAmount(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func validPayment(s string) bool {
	switch db.PaymentMethod(s) {
	case db.PaymentMethodCash, db.PaymentMethodCard, db.PaymentMethodTransfer:
		return true
	}
	return false
}

func saleToDTO(s db.Sale, items []SaleItemDTO) (SaleDTO, error) {
	total, err := parseAmount(s.Total)
	if err != nil {
		return SaleDTO{}, fmt.Errorf("parseFloat total: %w", err)
	}
	return SaleDTO{
		ID:            s.ID,
		BranchID:      s.BranchID,
		UserID:        s.UserID,
		Total:         total,
		PaymentMethod: string(s.PaymentMethod),
		Status:        string(s.Status),
		CreatedAt:     s.CreatedAt.UTC().Format(time.RFC3339),
		Items:         items,
	}, nil
}

func itemToDTO(i db.SaleItem) (SaleItemDTO, error) {
	up, err := parseAmount(i.UnitPrice)
	if err != nil {
		return SaleItemDTO{}, err
	}
	sub, err := parseAmount(i.Subtotal)
	if err != nil {
		return SaleItemDTO{}, err
	}
	return SaleItemDTO{
		ID:          i.ID,
		ProductID:   i.ProductID,
		ProductName: i.ProductName,
		UnitPrice:   up,
		Quantity:    int(i.Quantity),
		Subtotal:    sub,
	}, nil
}

// ── CreateSale ────────────────────────────────────────────────────────────────

// CreateSale registra una venta de forma atómica:
//  1. Verifica que todos los productos existan y estén activos (precio del servidor)
//  2. Verifica stock suficiente en la sucursal
//  3. En una sola TX: crea sale, crea sale_items, descuenta inventory
func (s *Service) CreateSale(ctx context.Context, in CreateInput) (SaleDTO, error) {
	if in.BranchID == "" {
		return SaleDTO{}, ErrNoBranchID
	}
	if len(in.Items) == 0 {
		return SaleDTO{}, ErrEmptyItems
	}
	if !validPayment(in.PaymentMethod) {
		return SaleDTO{}, ErrInvalidPayment
	}
	for _, it := range in.Items {
		if it.Quantity <= 0 {
			return SaleDTO{}, ErrInvalidQty
		}
	}

	// ── Paso 1: obtener precios del servidor y verificar stock (fuera de TX) ──
	type resolvedItem struct {
		input   ItemInput
		product db.Product
		price   float64
	}

	resolved := make([]resolvedItem, 0, len(in.Items))
	for _, it := range in.Items {
		p, err := s.q.GetProduct(ctx, it.ProductID)
		if err != nil || !p.Active {
			return SaleDTO{}, ErrProductUnavailable
		}
		price, err := parseAmount(p.Price)
		if err != nil {
			return SaleDTO{}, fmt.Errorf("precio inválido en producto %s: %w", p.ID, err)
		}

		inv, err := s.q.GetInventoryByProductBranch(ctx, db.GetInventoryByProductBranchParams{
			ProductID: it.ProductID,
			BranchID:  in.BranchID,
		})
		if err != nil || inv.CurrentStock < int32(it.Quantity) {
			return SaleDTO{}, ErrInsufficientStock
		}

		resolved = append(resolved, resolvedItem{input: it, product: p, price: price})
	}

	// ── Paso 2: calcular total del servidor ───────────────────────────────────
	var total float64
	for _, ri := range resolved {
		total += ri.price * float64(ri.input.Quantity)
	}

	// ── Paso 3: transacción atómica ───────────────────────────────────────────
	var createdSale db.Sale
	var createdItems []db.SaleItem

	txErr := s.tx(ctx, func(q querier) error {
		var err error
		createdSale, err = q.CreateSale(ctx, db.CreateSaleParams{
			BranchID:      in.BranchID,
			UserID:        in.UserID,
			Total:         fmtAmount(total),
			PaymentMethod: db.PaymentMethod(in.PaymentMethod),
		})
		if err != nil {
			return fmt.Errorf("CreateSale: %w", err)
		}

		for _, ri := range resolved {
			subtotal := ri.price * float64(ri.input.Quantity)
			item, err := q.CreateSaleItem(ctx, db.CreateSaleItemParams{
				SaleID:      createdSale.ID,
				ProductID:   ri.product.ID,
				ProductName: ri.product.Name,
				UnitPrice:   fmtAmount(ri.price),
				Quantity:    int32(ri.input.Quantity),
				Subtotal:    fmtAmount(subtotal),
			})
			if err != nil {
				return fmt.Errorf("CreateSaleItem %s: %w", ri.product.ID, err)
			}
			createdItems = append(createdItems, item)

			_, err = q.DecrementInventory(ctx, db.DecrementInventoryParams{
				Qty:       int32(ri.input.Quantity),
				ProductID: ri.product.ID,
				BranchID:  in.BranchID,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrInsufficientStock
				}
				return fmt.Errorf("DecrementInventory %s: %w", ri.product.ID, err)
			}
		}
		return nil
	})
	if txErr != nil {
		return SaleDTO{}, txErr
	}

	itemDTOs := make([]SaleItemDTO, 0, len(createdItems))
	for _, i := range createdItems {
		dto, err := itemToDTO(i)
		if err != nil {
			return SaleDTO{}, err
		}
		itemDTOs = append(itemDTOs, dto)
	}
	return saleToDTO(createdSale, itemDTOs)
}

// ── GetSale ───────────────────────────────────────────────────────────────────

func (s *Service) GetSale(ctx context.Context, id string) (SaleDTO, error) {
	sale, err := s.q.GetSale(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SaleDTO{}, ErrSaleNotFound
		}
		return SaleDTO{}, err
	}
	rows, err := s.q.GetSaleItems(ctx, id)
	if err != nil {
		return SaleDTO{}, err
	}
	itemDTOs := make([]SaleItemDTO, 0, len(rows))
	for _, i := range rows {
		dto, err := itemToDTO(i)
		if err != nil {
			return SaleDTO{}, err
		}
		itemDTOs = append(itemDTOs, dto)
	}
	return saleToDTO(sale, itemDTOs)
}

// ── ListSales ─────────────────────────────────────────────────────────────────

func (s *Service) ListSales(ctx context.Context, f ListFilters) ([]SaleDTO, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.q.ListSales(ctx, db.ListSalesParams{
		Limit:    int32(limit),
		BranchID: f.BranchID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]SaleDTO, 0, len(rows))
	for _, sale := range rows {
		dto, err := saleToDTO(sale, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}
