-- +goose Up
-- SQL migration for creating business table
CREATE TABLE IF NOT EXISTS likes (
    promo_id uuid NOT NULL,
    user_id uuid NOT NULL
);

-- +goose Down
DROP TABLE likes;