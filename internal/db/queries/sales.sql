-- Queries para el recurso sales (ventas) e inventory (stock).

-- name: GetInventoryByProductBranch :one
SELECT id, product_id, branch_id, current_stock, min_stock, updated_at
FROM inventory
WHERE product_id = $1 AND branch_id = $2;

-- name: DecrementInventory :one
-- La condición current_stock >= qty garantiza que nunca bajamos de cero.
-- Si no hay stock suficiente, devuelve pgx.ErrNoRows (detectado como ErrInsufficientStock).
UPDATE inventory
SET current_stock = current_stock - sqlc.arg('qty'),
    updated_at    = now()
WHERE product_id    = sqlc.arg('product_id')
  AND branch_id     = sqlc.arg('branch_id')
  AND current_stock >= sqlc.arg('qty')
RETURNING *;

-- name: CreateSale :one
INSERT INTO sales (branch_id, user_id, total, payment_method)
VALUES (
  sqlc.arg('branch_id'),
  sqlc.arg('user_id'),
  sqlc.arg('total')::numeric,
  sqlc.arg('payment_method')
)
RETURNING *;

-- name: CreateSaleItem :one
INSERT INTO sale_items (sale_id, product_id, product_name, unit_price, quantity, subtotal)
VALUES (
  sqlc.arg('sale_id'),
  sqlc.arg('product_id'),
  sqlc.arg('product_name'),
  sqlc.arg('unit_price')::numeric,
  sqlc.arg('quantity'),
  sqlc.arg('subtotal')::numeric
)
RETURNING *;

-- name: GetSale :one
SELECT id, branch_id, user_id, total, payment_method, status, created_at
FROM sales
WHERE id = $1;

-- name: GetSaleItems :many
SELECT id, sale_id, product_id, product_name, unit_price, quantity, subtotal
FROM sale_items
WHERE sale_id = $1
ORDER BY id;

-- name: ListSales :many
SELECT id, branch_id, user_id, total, payment_method, status, created_at
FROM sales
WHERE (sqlc.narg('branch_id')::uuid IS NULL OR branch_id = sqlc.narg('branch_id'))
ORDER BY created_at DESC
LIMIT $1;
