// Package token firma y verifica los JWT que emite el API.
// Es el equivalente Go del JwtModule/JwtStrategy que teníamos en Nest:
// un solo lugar que conoce el secreto y la forma del payload (Claims).
package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims = lo que viaja DENTRO del token. Va firmado (HS256), no encriptado.
type Claims struct {
	Name                 string  `json:"name"`
	Email                string  `json:"email"`
	Role                 string  `json:"role"`
	BranchID             *string `json:"branchId"` // NULL para el owner (acceso global)
	jwt.RegisteredClaims         // incluye Subject (user id), ExpiresAt, IssuedAt
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret), ttl: 8 * time.Hour} // un turno laboral
}

// Sign genera un JWT firmado para un usuario.
func (m *Manager) Sign(userID, name, email, role string, branchID *string, now time.Time) (string, error) {
	claims := Claims{
		Name:     name,
		Email:    email,
		Role:     role,
		BranchID: branchID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse verifica la firma y la expiración, y devuelve los claims.
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Rechaza tokens firmados con un algoritmo distinto (defensa básica).
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo de firma inesperado: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, fmt.Errorf("token inválido")
	}
	return claims, nil
}
