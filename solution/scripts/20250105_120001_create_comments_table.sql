-- +goose Up
-- SQL migration for creating business table
CREATE TABLE IF NOT EXISTS comments(
    id_promo uuid NOT NULL,
    id_user uuid NOT NULL,
    text VARCHAR(1000) NOT NULL
);

-- +goose Down
DROP TABLE comments;