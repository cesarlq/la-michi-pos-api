// Package config carga la configuración desde variables de entorno.
// En Lambda las vars vienen del template.yaml; en local, del .env (cargado
// por godotenv en el main de cada comando).
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
}

// Load lee y valida las variables requeridas. Falla rápido si falta alguna.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: falta DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("config: falta JWT_SECRET")
	}
	return cfg, nil
}
