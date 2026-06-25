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
	r.Get("/summary", h.summary)
	r.Get("/sales-trend", h.salesTrend)
	r.Get("/top-products", h.topProducts)
	r.Get("/critical-stock", h.criticalStock)

	return r
}

// parseRange lee from/to (YYYY-MM-DD) del query y devuelve [from, to] como inicio
// de día UTC, con `to` inclusivo. Si faltan, usa los últimos defaultDays días.
func parseRange(r *http.Request, defaultDays int) (from, to time.Time, err error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	qf, qt := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if qf == "" || qt == "" {
		return today.AddDate(0, 0, -(defaultDays - 1)), today, nil
	}
	if from, err = time.Parse("2006-01-02", qf); err != nil {
		return
	}
	if to, err = time.Parse("2006-01-02", qt); err != nil {
		return
	}
	from = from.UTC().Truncate(24 * time.Hour)
	to = to.UTC().Truncate(24 * time.Hour)
	if to.Before(from) {
		from, to = to, from
	}
	return from, to, nil
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

// summary — GET /reports/summary?from=2026-06-01&to=2026-06-30&branch_id=xxx
// Resumen agregado del periodo (ventas, ingresos, unidades). Default: hoy.
func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	from, to, err := parseRange(r, 1)
	if err != nil {
		web.Error(w, http.StatusBadRequest, "Formato de fecha inválido. Use YYYY-MM-DD")
		return
	}

	dto, err := h.svc.Summary(r.Context(), SalesTrendFilters{
		DateFrom: from,
		DateTo:   to.Add(24 * time.Hour), // exclusivo: incluye el día `to`
		BranchID: resolveBranch(claims, r),
	})
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener el resumen")
		return
	}
	web.JSON(w, http.StatusOK, dto)
}

// salesTrend — GET /reports/sales-trend?from=&to=&branch_id=xxx
// Serie de ingresos/ventas por día (incluye días en cero). Default: últimos 7 días.
func (h *Handler) salesTrend(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	from, to, err := parseRange(r, 7)
	if err != nil {
		web.Error(w, http.StatusBadRequest, "Formato de fecha inválido. Use YYYY-MM-DD")
		return
	}
	if to.Sub(from) > 366*24*time.Hour {
		web.Error(w, http.StatusBadRequest, "El rango no puede exceder 366 días")
		return
	}

	dto, err := h.svc.SalesTrend(r.Context(), SalesTrendFilters{
		DateFrom: from,
		DateTo:   to, // generate_series usa endpoints inclusivos
		BranchID: resolveBranch(claims, r),
	})
	if err != nil {
		web.Error(w, http.StatusInternalServerError, "Error al obtener la tendencia de ventas")
		return
	}
	web.JSON(w, http.StatusOK, dto)
}

// topProducts — GET /reports/top-products?from=&to=&limit=10&branch_id=xxx
func (h *Handler) topProducts(w http.ResponseWriter, r *http.Request) {
	claims := web.UserFromContext(r.Context())

	from, to, err := parseRange(r, 7)
	if err != nil {
		web.Error(w, http.StatusBadRequest, "Formato de fecha inválido. Use YYYY-MM-DD")
		return
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

	dto, err := h.svc.TopProducts(r.Context(), TopProductsFilters{
		DateFrom: from,
		DateTo:   to.Add(24 * time.Hour), // exclusivo: incluye el día `to`
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
