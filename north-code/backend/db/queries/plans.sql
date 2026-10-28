-- name: CreatePlan :one
INSERT INTO plans (name, result)
VALUES (?, ?)
RETURNING id, name, result, created_at;

-- name: GetPlan :one
SELECT id, name, result, created_at FROM plans WHERE id = ?;

-- name: ListPlans :many
SELECT id, name, result, created_at FROM plans ORDER BY id DESC LIMIT 100;

-- name: RenamePlan :one
UPDATE plans SET name = ? WHERE id = ?
RETURNING id, name, result, created_at;

-- name: DeletePlan :execrows
DELETE FROM plans WHERE id = ?;
