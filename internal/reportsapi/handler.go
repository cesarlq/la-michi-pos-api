package reportsapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cesarlq/la-michi-pos-api/internal/token"
	"github.com/cesarlq/la-michi-pos-api/internal/web"
)

type Handler struct {
	svc   *Service
	token *token.Manager
}

func NewHandler(svc *Service, tm *token.Manager) *Handler {
	return &Handler{svc: svc, token: tm}
}

// Routes registra las rutas del recurso bajo /reports.
// Todos los endpoints requieren autenticación.
// Managers ven solo su sucursal; owners pueden ver cualquier sucursal.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(web.Authenticator(h.token))

	r.Get("/daily", h.daily)
	r.Get("/sales-trend", h.salesTrend)
	r.Get("/top-products", h.topProducts)
	r.Get("/critical-stock", h.criticalStock)

	return r
}

// resolveBranch devuelve el branch_id efectivo según el rol:
// - managers/employees: siempre su sucursal del JWT (ignora ?branch_id del query)
// - owners: opcional ?branch_id=xxx, o nil para ver todas
func resolveBranch(claims *token.Claims, r *http.Request) *string {
	if claims.Role != "owner" {
		return claims.BranchID
	}
	if v := r.URL.Query().Get("branch_id"); v != "" {
		return &v
	}
	return nil
}

// daily — GET /reports/daily?date=2026-06-21&branch_id=xxx
func (h *Handler) daily(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	// Parsear fecha (default: hoy UTC)
	date := time.Now().UTC()
	if v := r.URL.Query().Get("date"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			web.Error(w, http.StatusBadRequest, "Formato de fecha inválido. Use YYYY-MM-DD")
			return
		}
		date = parsed
	}

	dto, err := h.svc.DailySummary(r.Context(), DailyFilters{
		Date:     date,
		BranchID: resolveBranch(claims, r),
	})
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener resumen del día")
		return
	}
	web.JSON(w, http.StatusOK, dto)
}

// salesTrend — GET /reports/sales-trend?days=7&branch_id=xxx
// Devuelve una serie de ingresos/ventas por día (incluye días en cero).
func (h *Handler) salesTrend(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 90 {
			web.Error(w, http.StatusBadRequest, "El parámetro days debe ser un número entre 1 y 90")
			return
		}
		days = n
	}

	// Construimos `days` cubetas: desde (hoy - days + 1) hasta hoy, todo a inicio de día UTC.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dto, err := h.svc.SalesTrend(r.Context(), SalesTrendFilters{
		DateFrom: today.AddDate(0, 0, -(days - 1)),
		DateTo:   today,
		BranchID: resolveBranch(claims, r),
	})
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener la tendencia de ventas")
		return
	}
	web.JSON(w, http.StatusOK, dto)
}

// topProducts — GET /reports/top-products?days=7&limit=10&branch_id=xxx
func (h *Handler) topProducts(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 365 {
			web.Error(w, http.StatusBadRequest, "El parámetro days debe ser un número entre 1 y 365")
			return
		}
		days = n
	}

	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			web.Error(w, http.StatusBadRequest, "El parámetro limit debe ser un número positivo")
			return
		}
		limit = n
	}

	now := time.Now().UTC()
	dto, err := h.svc.TopProducts(r.Context(), TopProductsFilters{
		DateFrom: now.AddDate(0, 0, -days).Truncate(24 * time.Hour),
		DateTo:   now,
		BranchID: resolveBranch(claims, r),
		Limit:    limit,
	})
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener top productos")
		return
	}
	web.JSON(w, http.StatusOK, dto)
}

// criticalStock — GET /reports/critical-stock?branch_id=xxx
func (h *Handler) criticalStock(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	dto, err := h.svc.CriticalStock(r.Context(), CriticalStockFilters{
		BranchID: resolveBranch(claims, r),
	})
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener stock crítico")
		return
	}
	web.JSON(w, http.StatusOK, dto)
}
