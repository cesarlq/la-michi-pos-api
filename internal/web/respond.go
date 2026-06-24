// Package web reúne helpers HTTP transversales: respuestas JSON, manejo de
// errores y el middleware de autenticación/roles. Lo usan los 4 recursos.
package web

import (
	"encoding/json"
	"net/http"
)

// JSON escribe una respuesta JSON con el status dado.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// Error escribe un error en formato uniforme: { "error": "mensaje" }.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON parsea el body en dst y rechaza campos desconocidos
// (equivalente a forbidNonWhitelisted del ValidationPipe de Nest).
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
