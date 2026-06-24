package salesapi

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

// Routes registra las rutas del recurso bajo /sales.
// Todos los endpoints requieren autenticación.
// - POST: cualquier rol (el cajero puede ser employee)
// - GET list: managers ven solo su sucursal; owners ven todas
// - GET detail: cualquier rol
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(web.Authenticator(h.token))

	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)

	return r
}

type itemInput struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type createRequest struct {
	BranchID      *string     `json:"branchId"` // opcional: toma del JWT si nil
	PaymentMethod string      `json:"paymentMethod"`
	Items         []itemInput `json:"items"`
}

// create — POST /sales
// El branch_id viene del JWT; si el usuario es owner puede enviarlo en el body.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	var req createRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	// Determinar sucursal: JWT tiene precedencia para no-owners.
	branchID := ""
	if claims.BranchID != nil {
		branchID = *claims.BranchID
	} else if req.BranchID != nil {
		branchID = *req.BranchID
	}

	items := make([]ItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, ItemInput{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
		})
	}

	sale, err := h.svc.CreateSale(r.Context(), CreateInput{
		BranchID:      branchID,
		UserID:        claims.Subject,
		PaymentMethod: req.PaymentMethod,
		Items:         items,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrNoBranchID):
			web.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrEmptyItems):
			web.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrInvalidQty):
			web.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrInvalidPayment):
			web.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrProductUnavailable):
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, ErrInsufficientStock):
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
		default:
			web.Error(w, http.StatusInternalServerError, "Error al registrar la venta")
		}
		return
	}
	web.JSON(w, http.StatusCreated, sale)
}

// list — GET /sales?limit=50
// Managers ven solo su sucursal (del JWT). Owners ven todo.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	filters := ListFilters{}

	// Si el usuario es manager/employee, restringir a su sucursal.
	if claims.Role != "owner" && claims.BranchID != nil {
		filters.BranchID = claims.BranchID
	}

	sales, err := h.svc.ListSales(r.Context(), filters)
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener ventas")
		return
	}
	web.JSON(w, http.StatusOK, sales)
}

// get — GET /sales/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sale, err := h.svc.GetSale(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSaleNotFound) {
			web.Error(w, http.StatusNotFound, "Venta no encontrada")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al obtener la venta")
		return
	}
	web.JSON(w, http.StatusOK, sale)
}
