package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/cesarlq/la-michi-pos-api/internal/token"
)

// ctxKey es un tipo privado para no colisionar con otras claves del contexto.
type ctxKey string

const userKey ctxKey = "user"

// Authenticator es el "portero" (equivalente al JwtAuthGuard de Nest):
// exige "Authorization: Bearer <token>", lo verifica y pega los claims en el
// contexto. Sin token o token inválido → 401.
func Authenticator(tm *token.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				Error(w, http.StatusUnauthorized, "Falta el token de autenticación")
				return
			}
			raw := strings.TrimPrefix(header, "Bearer ")
			claims, err := tm.Parse(raw)
			if err != nil {
				Error(w, http.StatusUnauthorized, "Token inválido o expirado")
				return
			}
			ctx := context.WithValue(r.Context(), userKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole exige que el usuario autenticado tenga uno de los roles dados
// (equivalente al RolesGuard + @Roles de Nest). Debe ir DESPUÉS de Authenticator.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := UserFromContext(r.Context())
			if claims == nil {
				Error(w, http.StatusUnauthorized, "No autenticado")
				return
			}
			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			Error(w, http.StatusForbidden, "No tienes permiso para esta acción")
		})
	}
}

// UserFromContext recupera los claims del usuario autenticado.
func UserFromContext(ctx context.Context) *token.Claims {
	claims, _ := ctx.Value(userKey).(*token.Claims)
	return claims
}
