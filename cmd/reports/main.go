// Lambda "reports": expone /reports (resúmenes de negocio).
// Mismo patrón que los demás comandos: Lambda en AWS o servidor HTTP en local.
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

	"github.com/cesarlq/la-michi-pos-api/internal/config"
	"github.com/cesarlq/la-michi-pos-api/internal/database"
	"github.com/cesarlq/la-michi-pos-api/internal/db"
	"github.com/cesarlq/la-michi-pos-api/internal/reportsapi"
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

	tm := token.NewManager(cfg.JWTSecret)
	svc := reportsapi.NewService(db.New(pool))
	h := reportsapi.NewHandler(svc, tm)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Mount("/reports", h.Routes())
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

	addr := ":" + getenv("PORT", "4003")
	log.Printf("reports API (local) escuchando en %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
