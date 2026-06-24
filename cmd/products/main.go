// Lambda "products": expone /products (CRUD de productos).
// Mismo patrón que cmd/auth: corre como Lambda en AWS o servidor HTTP en local.
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
	"github.com/joho/godotenv"

	"github.com/cesarlq/la-michi-pos-api/internal/branchesapi"
	"github.com/cesarlq/la-michi-pos-api/internal/config"
	"github.com/cesarlq/la-michi-pos-api/internal/database"
	"github.com/cesarlq/la-michi-pos-api/internal/db"
	"github.com/cesarlq/la-michi-pos-api/internal/inventoryapi"
	"github.com/cesarlq/la-michi-pos-api/internal/productsapi"
	"github.com/cesarlq/la-michi-pos-api/internal/token"
)

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

	queries := db.New(pool)
	tm := token.NewManager(cfg.JWTSecret)

	svc := productsapi.NewService(queries)
	ph := productsapi.NewHandler(svc, tm)
	bh := branchesapi.NewHandler(branchesapi.NewService(queries), tm)
	ih := inventoryapi.NewHandler(inventoryapi.NewService(queries), tm)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Mount("/products", ph.Routes())
	r.Mount("/branches", bh.Routes())
	r.Mount("/inventory", ih.Routes())
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

	addr := ":" + getenv("PORT", "4001")
	log.Printf("products API (local) escuchando en %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
