-- Queries de reportes: ventas del día, top productos, stock crítico.

-- name: DailySummary :one
-- Total de ventas del día para una sucursal (o todas si branch_id es NULL).
SELECT
  COUNT(*)::int                    AS sale_count,
  COALESCE(SUM(total), 0)::text    AS total_revenue,
  COALESCE(SUM(
    (SELECT SUM(quantity) FROM sale_items si WHERE si.sale_id = s.id)
  ), 0)::int                       AS items_sold
FROM sales s
WHERE status = 'completed'
  AND created_at >= sqlc.arg('date_from')::timestamptz
  AND created_at <  sqlc.arg('date_to')::timestamptz
  AND (sqlc.narg('branch_id')::uuid IS NULL OR branch_id = sqlc.narg('branch_id'));

-- name: TopProducts :many
-- Top N productos por cantidad vendida en el rango de fechas.
SELECT
  si.product_id,
  si.product_name,
  p.category::text                        AS category,
  SUM(si.quantity)::int                   AS total_qty,
  SUM(si.subtotal)::text                  AS total_revenue
FROM sale_items si
JOIN products p  ON p.id  = si.product_id
JOIN sales    s  ON s.id  = si.sale_id
WHERE s.status = 'completed'
  AND s.created_at >= sqlc.arg('date_from')::timestamptz
  AND s.created_at <  sqlc.arg('date_to')::timestamptz
  AND (sqlc.narg('branch_id')::uuid IS NULL OR s.branch_id = sqlc.narg('branch_id'))
GROUP BY si.product_id, si.product_name, p.category
ORDER BY total_qty DESC
LIMIT sqlc.arg('lim');

-- name: CriticalStock :many
-- Productos cuyo stock actual está en o por debajo del mínimo.
SELECT
  i.product_id,
  p.name          AS product_name,
  p.category::text AS category,
  i.branch_id,
  b.name          AS branch_name,
  i.current_stock,
  i.min_stock
FROM inventory i
JOIN products  p ON p.id = i.product_id
JOIN branches  b ON b.id = i.branch_id
WHERE i.current_stock <= i.min_stock
  AND p.active = true
  AND b.active = true
  AND (sqlc.narg('branch_id')::uuid IS NULL OR i.branch_id = sqlc.narg('branch_id'))
ORDER BY i.current_stock ASC, p.name;
