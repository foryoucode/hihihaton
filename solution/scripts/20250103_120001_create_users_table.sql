-- +goose Up
-- SQL migration for creating business table
CREATE TABLE IF NOT EXISTS users (
    id uuid UNIQUE DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    email VARCHAR(120) UNIQUE NOT NULL,
    password VARCHAR(60) NOT NULL,
    surname VARCHAR(120) NOT NULL,
    avatar_url VARCHAR(350),
    age int NOT NULL,
    country VARCHAR(2)
);

-- +goose Down
DROP TABLE IF EXISTS users;
   