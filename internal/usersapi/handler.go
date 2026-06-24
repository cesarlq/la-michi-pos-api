package usersapi

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

// Routes: todo el recurso es solo para owner.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(web.Authenticator(h.token))
	r.Use(web.RequireRole("owner"))

	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.deactivate)

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener usuarios")
		return
	}
	web.JSON(w, http.StatusOK, users)
}

type createRequest struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Role     string  `json:"role"`
	BranchID *string `json:"branchId"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}
	user, err := h.svc.Create(r.Context(), CreateInput{
		Name: req.Name, Email: req.Email, Password: req.Password,
		Role: req.Role, BranchID: req.BranchID,
	})
	if err != nil {
		writeUserError(w, err, "Error al crear usuario")
		return
	}
	web.JSON(w, http.StatusCreated, user)
}

type updateRequest struct {
	Name     *string `json:"name"`
	Role     *string `json:"role"`
	BranchID *string `json:"branchId"`
	Password *string `json:"password"` // opcional: si viene, cambia la contraseña
	Active   *bool   `json:"active"`   // si viene, solo cambia el estado
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req updateRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	// Cambio de estado (reactivar / desactivar).
	if req.Active != nil {
		// Regla: el owner no puede desactivarse a sí mismo (no quedarse sin acceso).
		if !*req.Active && id == claims.Subject {
			web.Error(w, http.StatusBadRequest, "No puedes desactivar tu propia cuenta")
			return
		}
		user, err := h.svc.SetActive(r.Context(), id, *req.Active)
		if err != nil {
			writeUserError(w, err, "Error al actualizar usuario")
			return
		}
		web.JSON(w, http.StatusOK, user)
		return
	}

	// Cambio de contraseña.
	if req.Password != nil {
		if err := h.svc.ChangePassword(r.Context(), id, *req.Password); err != nil {
			writeUserError(w, err, "Error al cambiar la contraseña")
			return
		}
	}

	user, err := h.svc.Update(r.Context(), id, UpdateInput{
		Name: req.Name, Role: req.Role, BranchID: req.BranchID,
	})
	if err != nil {
		writeUserError(w, err, "Error al actualizar usuario")
		return
	}
	web.JSON(w, http.StatusOK, user)
}

// deactivate — DELETE /users/{id}
func (h *Handler) deactivate(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if id == claims.Subject {
		web.Error(w, http.StatusBadRequest, "No puedes desactivar tu propia cuenta")
		return
	}

	if _, err := h.svc.SetActive(r.Context(), id, false); err != nil {
		writeUserError(w, err, "Error al desactivar usuario")
		return
	}
	web.JSON(w, http.StatusOK, map[string]string{"message": "Usuario desactivado"})
}

func writeUserError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		web.Error(w, http.StatusNotFound, "Usuario no encontrado")
	case errors.Is(err, ErrEmailtaken):
		web.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidEmail), errors.Is(err, ErrShortPassword),
		errors.Is(err, ErrInvalidRole), errors.Is(err, ErrBranchNeeded),
		errors.Is(err, ErrEmptyName):
		web.Error(w, http.StatusBadRequest, err.Error())
	default:
		web.Error(w, http.StatusInternalServerError, fallback)
	}
}
