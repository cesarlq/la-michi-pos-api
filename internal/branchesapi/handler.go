package branchesapi

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

// Routes: GET para cualquier autenticado (lo usan POS, dashboard, inventario);
// crear/editar/activar solo owner.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(web.Authenticator(h.token))

	r.Get("/", h.list)
	r.Get("/{id}", h.get)

	r.Group(func(r chi.Router) {
		r.Use(web.RequireRole("owner"))
		r.Post("/", h.create)
		r.Patch("/{id}", h.update)
		r.Delete("/{id}", h.deactivate)
	})

	return r
}

// list — GET /branches?all=true (all solo lo respeta para owner)
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())
	all := r.URL.Query().Get("all") == "true" && claims.Role == "owner"

	branches, err := h.svc.List(r.Context(), all)
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener sucursales")
		return
	}
	web.JSON(w, http.StatusOK, branches)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	branch, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			web.Error(w, http.StatusNotFound, "Sucursal no encontrada")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al obtener sucursal")
		return
	}
	web.JSON(w, http.StatusOK, branch)
}

type branchRequest struct {
	Name    string  `json:"name"`
	Address *string `json:"address"`
	Phone   *string `json:"phone"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req branchRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}
	branch, err := h.svc.Create(r.Context(), CreateInput{Name: req.Name, Address: req.Address, Phone: req.Phone})
	if err != nil {
		if errors.Is(err, ErrEmptyName) {
			web.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al crear sucursal")
		return
	}
	web.JSON(w, http.StatusCreated, branch)
}

type branchUpdateRequest struct {
	Name    *string `json:"name"`
	Address *string `json:"address"`
	Phone   *string `json:"phone"`
	Active  *bool   `json:"active"` // si viene, solo cambia el estado (reactivar/desactivar)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req branchUpdateRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}
	id := chi.URLParam(r, "id")

	// Si el body trae `active`, es un cambio de estado (no de campos).
	if req.Active != nil {
		branch, err := h.svc.SetActive(r.Context(), id, *req.Active)
		if err != nil {
			if errors.Is(err, ErrBranchNotFound) {
				web.Error(w, http.StatusNotFound, "Sucursal no encontrada")
				return
			}
			web.Error(w, http.StatusInternalServerError, "Error al actualizar sucursal")
			return
		}
		web.JSON(w, http.StatusOK, branch)
		return
	}

	branch, err := h.svc.Update(r.Context(), id, UpdateInput{
		Name: req.Name, Address: req.Address, Phone: req.Phone,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrBranchNotFound):
			web.Error(w, http.StatusNotFound, "Sucursal no encontrada")
		case errors.Is(err, ErrEmptyName):
			web.Error(w, http.StatusBadRequest, err.Error())
		default:
			web.Error(w, http.StatusInternalServerError, "Error al actualizar sucursal")
		}
		return
	}
	web.JSON(w, http.StatusOK, branch)
}

// deactivate — DELETE /branches/{id} (baja lógica: active = false)
func (h *Handler) deactivate(w http.ResponseWriter, r *http.Request) {
	_, err := h.svc.SetActive(r.Context(), chi.URLParam(r, "id"), false)
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			web.Error(w, http.StatusNotFound, "Sucursal no encontrada")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al desactivar sucursal")
		return
	}
	web.JSON(w, http.StatusOK, map[string]string{"message": "Sucursal desactivada"})
}
