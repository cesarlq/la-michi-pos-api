-- name: GetUserByEmail :one
SELECT id, name, email, password_hash, role, branch_id, active, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, name, email, password_hash, role, branch_id, active, created_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, name, email, password_hash, role, branch_id, active, created_at
FROM users
ORDER BY name;

-- name: CreateUser :one
INSERT INTO users (name, email, password_hash, role, branch_id)
VALUES (
  sqlc.arg('name'),
  sqlc.arg('email'),
  sqlc.arg('password_hash'),
  sqlc.arg('role'),
  sqlc.narg('branch_id')
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET
  name      = COALESCE(sqlc.narg('name'), name),
  role      = COALESCE(sqlc.narg('role')::user_role, role),
  branch_id = sqlc.arg('branch_id')  -- nullable: puede limpiarse (owner sin sucursal)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: SetUserActive :one
UPDATE users SET active = $2 WHERE id = $1 RETURNING *;
