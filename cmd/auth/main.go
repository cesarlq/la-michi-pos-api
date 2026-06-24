// Lambda "auth": expone /auth/login y /auth/me.
// El mismo binario corre como Lambda (en AWS) o como servidor HTTP (en local),
// según exista la variable AWS_LAMBDA_RUNTIME_API.
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

	"github.com/cesarlq/la-michi-pos-api/internal/authapi"
	"github.com/cesarlq/la-michi-pos-api/internal/config"
	"github.com/cesarlq/la-michi-pos-api/internal/database"
	"github.com/cesarlq/la-michi-pos-api/internal/db"
	"github.com/cesarlq/la-michi-pos-api/internal/token"
	"github.com/cesarlq/la-michi-pos-api/internal/usersapi"
)

// buildRouter arma todo el cableado UNA vez (cold start) y se reutiliza.
func buildRouter() *chi.Mux {
	_ = godotenv.Load() // local: carga .env. En Lambda no existe → se ignora.

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

	authH := authapi.NewHandler(authapi.NewService(queries, tm), tm)
	usersH := usersapi.NewHandler(usersapi.NewService(queries), tm)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Mount("/auth", authH.Routes())
	r.Mount("/users", usersH.Routes())
	return r
}

func main() {
	r := buildRouter()

	// En AWS: envuelve el router chi como handler de API Gateway (HTTP API v2).
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		adapter := chiadapter.NewV2(r)
		lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return adapter.ProxyWithContextV2(ctx, req)
		})
		return
	}

	// En local: servidor HTTP normal para probar con curl sin SAM.
	addr := ":" + getenv("PORT", "4000")
	log.Printf("auth API (local) escuchando en %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
