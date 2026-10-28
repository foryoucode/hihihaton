-- +goose Up
-- SQL migration for creating business table
CREATE TABLE IF NOT EXISTS business (
    id uuid UNIQUE DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL,
    email VARCHAR(120) UNIQUE NOT NULL,
    password VARCHAR(60) NOT NULL
);


-- +goose Down
DROP TABLE IF EXISTS business;