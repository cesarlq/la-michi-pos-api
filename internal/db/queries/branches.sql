-- Queries para el recurso branches (sucursales).

-- name: ListBranches :many
-- Solo activas. La usan el POS, el dashboard y el inventario.
SELECT id, name, address, phone, active, created_at
FROM branches
WHERE active = true
ORDER BY name;

-- name: ListAllBranches :many
-- Todas (activas e inactivas). La usa el owner en la pantalla de gestión.
SELECT id, name, address, phone, active, created_at
FROM branches
ORDER BY name;

-- name: GetBranch :one
SELECT id, name, address, phone, active, created_at
FROM branches
WHERE id = $1;

-- name: CreateBranch :one
INSERT INTO branches (name, address, phone)
VALUES (sqlc.arg('name'), sqlc.narg('address'), sqlc.narg('phone'))
RETURNING *;

-- name: UpdateBranch :one
UPDATE branches
SET
  name    = COALESCE(sqlc.narg('name'), name),
  address = COALESCE(sqlc.narg('address'), address),
  phone   = COALESCE(sqlc.narg('phone'), phone)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: SetBranchActive :one
UPDATE branches SET active = $2 WHERE id = $1 RETURNING *;
