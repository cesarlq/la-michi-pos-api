// Package database abre el pool de conexiones a Postgres con pgx.
//
// Patrón Lambda: el pool se crea UNA vez en el cold start (en el main de cada
// comando) y se reutiliza entre invocaciones mientras la Lambda siga "caliente".
// Por eso el pool se mantiene chico (en RDS las conexiones son un recurso caro;
// en producción se pone RDS Proxy en medio).
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database: url inválida: %w", err)
	}

	// Pool pequeño: cada Lambda concurrente abre el suyo; no queremos saturar RDS.
	cfg.MaxConns = 4
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: no se pudo crear el pool: %w", err)
	}

	// Verifica que la BD responde antes de seguir.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("database: ping falló: %w", err)
	}
	return pool, nil
}
