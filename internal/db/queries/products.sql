-- Queries para el recurso products.

-- name: ListProducts :many
SELECT id, name, category, price, active, created_at
FROM products
WHERE
  (sqlc.narg('active')::boolean IS NULL OR active = sqlc.narg('active'))
  AND (sqlc.narg('category')::product_category IS NULL OR category = sqlc.narg('category'))
ORDER BY name;

-- name: GetProduct :one
SELECT id, name, category, price, active, created_at
FROM products
WHERE id = $1;

-- name: CreateProduct :one
INSERT INTO products (name, category, price)
VALUES (sqlc.arg('name'), sqlc.arg('category'), sqlc.arg('price')::numeric)
RETURNING *;

-- name: UpdateProduct :one
UPDATE products
SET
  name     = COALESCE(sqlc.narg('name'), name),
  category = COALESCE(sqlc.narg('category')::product_category, category),
  price    = COALESCE(sqlc.narg('price')::text::numeric, price)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetProductActive :one
UPDATE products SET active = $2 WHERE id = $1 RETURNING *;
