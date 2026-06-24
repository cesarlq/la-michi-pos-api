package productsapi

import (
	"errors"
	"net/http"
	"strings"

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

// Routes registra las rutas del recurso bajo /products.
// Todos los endpoints requieren autenticación; POST/PATCH/DELETE requieren rol owner o manager.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(web.Authenticator(h.token))

	r.Get("/", h.list)
	r.Get("/sellable", h.sellable)
	r.Get("/{id}", h.get)

	// Solo owner y manager pueden escribir
	r.With(web.RequireRole("owner", "manager")).Post("/", h.create)
	r.With(web.RequireRole("owner", "manager")).Patch("/{id}", h.update)
	// Solo owner puede eliminar (soft-delete)
	r.With(web.RequireRole("owner")).Delete("/{id}", h.delete)

	return r
}

// list — GET /products?active=true&category=paleta
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filters := ListFilters{}

	if v := r.URL.Query().Get("active"); v != "" {
		b := v == "true"
		filters.Active = &b
	}
	if v := r.URL.Query().Get("category"); v != "" {
		filters.Category = &v
	}

	products, err := h.svc.ListProducts(r.Context(), filters)
	if err != nil {
		if errors.Is(err, ErrInvalidCategory) {
			web.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al obtener productos")
		return
	}
	web.JSON(w, http.StatusOK, products)
}

// get — GET /products/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	product, err := h.svc.GetProduct(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			web.Error(w, http.StatusNotFound, "Producto no encontrado")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al obtener producto")
		return
	}
	web.JSON(w, http.StatusOK, product)
}

type createRequest struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
}

// create — POST /products
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		web.Error(w, http.StatusBadRequest, "El nombre es requerido")
		return
	}

	product, err := h.svc.CreateProduct(r.Context(), CreateInput{
		Name:     req.Name,
		Category: req.Category,
		Price:    req.Price,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPrice):
			web.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrInvalidCategory):
			web.Error(w, http.StatusBadRequest, err.Error())
		default:
			web.Error(w, http.StatusInternalServerError, "Error al crear producto")
		}
		return
	}
	web.JSON(w, http.StatusCreated, product)
}

type updateRequest struct {
	Name     *string  `json:"name"`
	Category *string  `json:"category"`
	Price    *float64 `json:"price"`
}

// update — PATCH /products/{id}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
		if *req.Name == "" {
			web.Error(w, http.StatusBadRequest, "El nombre no puede estar vacío")
			return
		}
	}

	product, err := h.svc.UpdateProduct(r.Context(), id, UpdateInput{
		Name:     req.Name,
		Category: req.Category,
		Price:    req.Price,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrProductNotFound):
			web.Error(w, http.StatusNotFound, "Producto no encontrado")
		case errors.Is(err, ErrInvalidPrice):
			web.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrInvalidCategory):
			web.Error(w, http.StatusBadRequest, err.Error())
		default:
			web.Error(w, http.StatusInternalServerError, "Error al actualizar producto")
		}
		return
	}
	web.JSON(w, http.StatusOK, product)
}

// sellable — GET /products/sellable?branch_id=xxx
// Devuelve productos activos con su stock en la sucursal. Lo consume el POS del front.
func (h *Handler) sellable(w http.ResponseWriter, r *http.Request) {
	branchID := r.URL.Query().Get("branch_id")
	if branchID == "" {
		// Si el usuario tiene sucursal en el JWT, usarla como default.
		claims := web.UserFromContext(r.Context())
		if claims != nil && claims.BranchID != nil {
			branchID = *claims.BranchID
		}
	}
	if branchID == "" {
		web.Error(w, http.StatusBadRequest, "Se requiere branch_id")
		return
	}
	products, err := h.svc.ListSellableProducts(r.Context(), branchID)
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener productos vendibles")
		return
	}
	web.JSON(w, http.StatusOK, products)
}

// delete — DELETE /products/{id}  (soft-delete: active = false)
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteProduct(r.Context(), id); err != nil {
		if errors.Is(err, ErrProductNotFound) {
			web.Error(w, http.StatusNotFound, "Producto no encontrado")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al eliminar producto")
		return
	}
	web.JSON(w, http.StatusOK, map[string]string{"message": "Producto desactivado"})
}
