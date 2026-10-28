-- name: CreatePlan :one
INSERT INTO plans (dataset_id, name, result) VALUES ($1, $2, $3) RETURNING *;

-- name: GetPlan :one
SELECT * FROM plans WHERE id = $1;

-- name: ListPlans :many
SELECT * FROM plans ORDER BY id DESC LIMIT 100;

-- name: RenamePlan :one
UPDATE plans SET name = $1 WHERE id = $2 RETURNING *;

-- name: DeletePlan :execrows
DELETE FROM plans WHERE id = $1;
