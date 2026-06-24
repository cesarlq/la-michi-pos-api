-- Queries de inventario (usadas por el POS para mostrar productos con stock).

-- name: ListSellableProducts :many
-- Productos activos con su stock en la sucursal dada.
-- El front los usa en el POS para mostrar precio y disponibilidad.
SELECT
  p.id,
  p.name,
  p.category,
  p.price,
  i.current_stock AS stock
FROM products p
JOIN inventory i ON i.product_id = p.id AND i.branch_id = $1
WHERE p.active = true
ORDER BY p.name;

-- name: ListInventoryByBranch :many
-- Todos los productos activos con su stock y mínimo en la sucursal dada.
-- LEFT JOIN: incluye productos SIN renglón de inventario aún (stock/min = 0).
SELECT
  p.id            AS product_id,
  p.name          AS product_name,
  p.category,
  p.price,
  COALESCE(i.current_stock, 0)::int AS current_stock,
  COALESCE(i.min_stock, 0)::int     AS min_stock
FROM products p
LEFT JOIN inventory i ON i.product_id = p.id AND i.branch_id = $1
WHERE p.active = true
ORDER BY p.name;

-- name: RestockInventory :one
-- Suma una entrada de stock. UPSERT: crea el renglón si el producto aún no tiene
-- inventario en esa sucursal (ej. producto recién creado por el CRUD).
INSERT INTO inventory (product_id, branch_id, current_stock, min_stock)
VALUES (sqlc.arg('product_id'), sqlc.arg('branch_id'), sqlc.arg('qty'), 0)
ON CONFLICT (product_id, branch_id)
DO UPDATE SET current_stock = inventory.current_stock + EXCLUDED.current_stock,
              updated_at    = now()
RETURNING *;

-- name: SetMinStock :one
-- Fija el stock mínimo (umbral de alerta). UPSERT por la misma razón que restock.
INSERT INTO inventory (product_id, branch_id, current_stock, min_stock)
VALUES (sqlc.arg('product_id'), sqlc.arg('branch_id'), 0, sqlc.arg('min_stock'))
ON CONFLICT (product_id, branch_id)
DO UPDATE SET min_stock  = EXCLUDED.min_stock,
              updated_at = now()
RETURNING *;
