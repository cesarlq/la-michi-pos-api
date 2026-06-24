// Package inventoryapi implementa el recurso /inventory: ver y gestionar stock por sucursal.
// Operaciones: listar, reabastecer (entrada de stock) y fijar el mínimo (umbral de alerta).
package inventoryapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/cesarlq/la-michi-pos-api/internal/db"
)

var (
	ErrNoBranch   = errors.New("se requiere sucursal")
	ErrInvalidQty = errors.New("la cantidad a reabastecer debe ser mayor a cero")
	ErrInvalidMin = errors.New("el stock mínimo debe ser mayor o igual a cero")
	ErrNoProduct  = errors.New("se requiere el producto")
)

type querier interface {
	ListInventoryByBranch(ctx context.Context, branchID string) ([]db.ListInventoryByBranchRow, error)
	RestockInventory(ctx context.Context, arg db.RestockInventoryParams) (db.Inventory, error)
	SetMinStock(ctx context.Context, arg db.SetMinStockParams) (db.Inventory, error)
}

// InventoryRowDTO — un producto con su stock y mínimo en una sucursal.
type InventoryRowDTO struct {
	ProductID    string  `json:"productId"`
	ProductName  string  `json:"productName"`
	Category     string  `json:"category"`
	Price        float64 `json:"price"`
	CurrentStock int     `json:"currentStock"`
	MinStock     int     `json:"minStock"`
}

// InventoryItemDTO — el renglón de inventario tras una mutación (restock / min).
type InventoryItemDTO struct {
	ProductID    string `json:"productId"`
	BranchID     string `json:"branchId"`
	CurrentStock int    `json:"currentStock"`
	MinStock     int    `json:"minStock"`
}

type Service struct {
	q querier
}

func NewService(q querier) *Service {
	return &Service{q: q}
}

func (s *Service) ListByBranch(ctx context.Context, branchID string) ([]InventoryRowDTO, error) {
	if branchID == "" {
		return nil, ErrNoBranch
	}
	rows, err := s.q.ListInventoryByBranch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out := make([]InventoryRowDTO, 0, len(rows))
	for _, r := range rows {
		price, err := strconv.ParseFloat(r.Price, 64)
		if err != nil {
			return nil, fmt.Errorf("parseFloat price %q: %w", r.Price, err)
		}
		out = append(out, InventoryRowDTO{
			ProductID:    r.ProductID,
			ProductName:  r.ProductName,
			Category:     string(r.Category),
			Price:        price,
			CurrentStock: int(r.CurrentStock),
			MinStock:     int(r.MinStock),
		})
	}
	return out, nil
}

func itemToDTO(i db.Inventory) InventoryItemDTO {
	return InventoryItemDTO{
		ProductID:    i.ProductID,
		BranchID:     i.BranchID,
		CurrentStock: int(i.CurrentStock),
		MinStock:     int(i.MinStock),
	}
}

// Restock suma una entrada de stock (entrada de mercancía).
func (s *Service) Restock(ctx context.Context, productID, branchID string, qty int) (InventoryItemDTO, error) {
	if productID == "" {
		return InventoryItemDTO{}, ErrNoProduct
	}
	if branchID == "" {
		return InventoryItemDTO{}, ErrNoBranch
	}
	if qty <= 0 {
		return InventoryItemDTO{}, ErrInvalidQty
	}
	inv, err := s.q.RestockInventory(ctx, db.RestockInventoryParams{
		ProductID: productID,
		BranchID:  branchID,
		Qty:       int32(qty),
	})
	if err != nil {
		return InventoryItemDTO{}, err
	}
	return itemToDTO(inv), nil
}

// SetMinStock fija el umbral de alerta de stock bajo.
func (s *Service) SetMinStock(ctx context.Context, productID, branchID string, minStock int) (InventoryItemDTO, error) {
	if productID == "" {
		return InventoryItemDTO{}, ErrNoProduct
	}
	if branchID == "" {
		return InventoryItemDTO{}, ErrNoBranch
	}
	if minStock < 0 {
		return InventoryItemDTO{}, ErrInvalidMin
	}
	inv, err := s.q.SetMinStock(ctx, db.SetMinStockParams{
		ProductID: productID,
		BranchID:  branchID,
		MinStock:  int32(minStock),
	})
	if err != nil {
		return InventoryItemDTO{}, err
	}
	return itemToDTO(inv), nil
}
