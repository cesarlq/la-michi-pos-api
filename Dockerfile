# syntax=docker/dockerfile:1

# ─── Stage 1: Build ─────────────────────────────────────────────
FROM golang:1.26-alpine AS build

# CMD_PATH = build argument: cada Lambda elige qué comando compilar
# (./cmd/auth, ./cmd/products, ...) desde template.yaml.
ARG CMD_PATH=./cmd/auth

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Binario estático arm64, sin símbolos de debug (-s -w) → imagen mínima.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w" -o /out/bootstrap ${CMD_PATH}

# ─── Stage 2: Runtime ───────────────────────────────────────────
# Imagen base oficial de AWS para Lambdas con runtime "provided" (custom).
FROM public.ecr.aws/lambda/provided:al2023-arm64

COPY --from=build /out/bootstrap /var/runtime/bootstrap

ENTRYPOINT ["/var/runtime/bootstrap"]
