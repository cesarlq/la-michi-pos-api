// Lambda "sales": expone /sales (registro y consulta de ventas).
// Mismo patrón que cmd/auth y cmd/products.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/cesarlq/la-michi-pos-api/internal/config"
	"github.com/cesarlq/la-michi-pos-api/internal/database"
	"github.com/cesarlq/la-michi-pos-api/internal/db"
	"github.com/cesarlq/la-michi-pos-api/internal/salesapi"
	"github.com/cesarlq/la-michi-pos-api/internal/token"
)

// newPgTxRunner devuelve un TxRunner real que usa el pool para transacciones Postgres.
func newPgTxRunner(pool *pgxpool.Pool) salesapi.TxRunner {
	return func(ctx context.Context, fn func(q salesapi.Querier) error) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		if err := fn(db.New(tx)); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

func buildRouter() *chi.Mux {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := database.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	tm := token.NewManager(cfg.JWTSecret)
	svc := salesapi.NewService(db.New(pool), newPgTxRunner(pool))
	h := salesapi.NewHandler(svc, tm)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Mount("/sales", h.Routes())
	return r
}

func main() {
	r := buildRouter()

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		adapter := chiadapter.NewV2(r)
		lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return adapter.ProxyWithContextV2(ctx, req)
		})
		return
	}

	addr := ":" + getenv("PORT", "4002")
	log.Printf("sales API (local) escuchando en %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
