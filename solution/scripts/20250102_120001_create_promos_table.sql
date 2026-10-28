-- +goose Up
-- SQL migration for creating business table
CREATE TYPE promo_mode AS ENUM ('UNIQUE', 'COMMON');


CREATE TABLE IF NOT EXISTS promos (
    id uuid UNIQUE DEFAULT uuid_generate_v4(),
    mode promo_mode NOT NULL,
    promo_common VARCHAR(30),
    promo_unique text[],
    description VARCHAR(300) NOT NULL,
    image_url VARCHAR(350),
    age_from int,
    age_until int,
    country VARCHAR(2),
    categories text[],
    max_count int NOT NULL,
    active_from text,
    active_until text,
    id_business uuid NOT NULL,
    active bool default true,
    created_at timestamp
);


-- +goose Down
DROP TABLE IF EXISTS promos;

   