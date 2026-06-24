package authapi

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

// Routes registra las rutas del recurso bajo /auth.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/login", h.login)                           // público
	r.With(web.Authenticator(h.token)).Get("/me", h.me) // protegido
	return r
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Error(w, http.StatusBadRequest, "Body inválido")
		return
	}

	// Validación mínima (equivalente al LoginDto de Nest).
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		web.Error(w, http.StatusBadRequest, "Email inválido")
		return
	}
	if len(req.Password) < 6 {
		web.Error(w, http.StatusBadRequest, "La contraseña debe tener al menos 6 caracteres")
		return
	}

	result, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			web.Error(w, http.StatusUnauthorized, "Credenciales inválidas")
			return
		}
		web.Error(w, http.StatusInternalServerError, "Error al iniciar sesión")
		return
	}
	web.JSON(w, http.StatusOK, result)
}

// me devuelve el usuario del token. Sirve para que el front valide la sesión.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())
	web.JSON(w, http.StatusOK, map[string]any{
		"id":       claims.Subject,
		"name":     claims.Name,
		"email":    claims.Email,
		"role":     claims.Role,
		"branchId": claims.BranchID,
	})
}
