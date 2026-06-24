package inventoryapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/token"
	"github.com/cesarlq/la-michi-pos-api/internal/web"
)

type Handler struct {
	svc   *Service
	token *token.Manager
}

func NewHandler(svc *Service, tm *token.Manager) *Handler {
	return &Handler{svc: svc, token: tm}
}

// Routes registra las rutas bajo /inventory. Solo owner y manager (no employee).
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(web.Authenticator(h.token))
	r.Use(web.RequireRole("owner", "manager"))

	r.Get("/", h.list)
	r.Post("/restock", h.restock)
	r.Patch("/min-stock", h.setMinStock)

	return r
}

// resolveBranch: managers/employees están atados a su sucursal del JWT.
// Owners eligen sucursal (query param en GET, campo branchId en el body de mutaciones).
func resolveBranch(claims *token.Claims, fromRequest string) string {
	if claims.Role != "owner" && claims.BranchID != nil {
		return *claims.BranchID
	}
	return fromRequest
}

// list — GET /inventory?branch_id=xxx
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())
	branchID := resolveBranch(claims, r.URL.Query().Get("branch_id"))

	rows, err := h.svc.ListByBranch(r.Context(), branchID)
	if err != nil {
		if errors.Is(err, ErrNoBranch) {
			web.Error(w, http.StatusBadRequest, "Se requiere sucursal")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al obtener inventario")
		return
	}
	web.JSON(w, http.StatusOK, rows)
}

type restockRequest struct {
	ProductID string  `json:"productId"`
	BranchID  *string `json:"branchId"`
	Quantity  int     `json:"quantity"`
}

// restock — POST /inventory/restock
func (h *Handler) restock(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	var req restockRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	branchFromReq := ""
	if req.BranchID != nil {
		branchFromReq = *req.BranchID
	}
	branchID := resolveBranch(claims, branchFromReq)

	item, err := h.svc.Restock(r.Context(), req.ProductID, branchID, req.Quantity)
	if err != nil {
		writeMutationError(w, err, "Error al reabastecer")
		return
	}
	web.JSON(w, http.StatusOK, item)
}

type minStockRequest struct {
	ProductID string  `json:"productId"`
	BranchID  *string `json:"branchId"`
	MinStock  int     `json:"minStock"`
}

// setMinStock — PATCH /inventory/min-stock
func (h *Handler) setMinStock(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	var req minStockRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	branchFromReq := ""
	if req.BranchID != nil {
		branchFromReq = *req.BranchID
	}
	branchID := resolveBranch(claims, branchFromReq)

	item, err := h.svc.SetMinStock(r.Context(), req.ProductID, branchID, req.MinStock)
	if err != nil {
		writeMutationError(w, err, "Error al actualizar el mínimo")
		return
	}
	web.JSON(w, http.StatusOK, item)
}

// writeMutationError mapea los errores de negocio a códigos HTTP.
func writeMutationError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrNoProduct), errors.Is(err, ErrNoBranch),
		errors.Is(err, ErrInvalidQty), errors.Is(err, ErrInvalidMin):
		web.Error(w, http.StatusBadRequest, err.Error())
	default:
		web.Error(w, http.StatusInternalServerError, fallback)
	}
}
