-- +goose Up
CREATE TABLE datasets (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    station_id TEXT NOT NULL,
    station_name TEXT NOT NULL,
    budget BIGINT NOT NULL CHECK (budget >= 0),
    deadline INTEGER NOT NULL CHECK (deadline > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE equipment (
    dataset_id BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    equipment_id TEXT NOT NULL,
    model_name TEXT NOT NULL,
    equipment_type TEXT NOT NULL,
    capacity DOUBLE PRECISION NOT NULL CHECK (capacity > 0),
    capacity_unit TEXT NOT NULL,
    min_operating_temperature_c DOUBLE PRECISION NOT NULL,
    installation_days INTEGER NOT NULL CHECK (installation_days >= 0),
    PRIMARY KEY (dataset_id, equipment_id)
);

CREATE TABLE suppliers (
    dataset_id BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    supplier_id TEXT NOT NULL,
    supplier_name TEXT NOT NULL,
    delivery_cost_rub BIGINT NOT NULL CHECK (delivery_cost_rub >= 0),
    delivery_days INTEGER NOT NULL CHECK (delivery_days >= 0),
    PRIMARY KEY (dataset_id, supplier_id)
);

CREATE TABLE offers (
    dataset_id BIGINT NOT NULL,
    offer_id TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    equipment_id TEXT NOT NULL,
    equipment_type TEXT NOT NULL,
    unit_price_rub BIGINT NOT NULL CHECK (unit_price_rub >= 0),
    stock_quantity INTEGER NOT NULL CHECK (stock_quantity >= 0),
    PRIMARY KEY (dataset_id, offer_id),
    FOREIGN KEY (dataset_id, supplier_id) REFERENCES suppliers(dataset_id, supplier_id),
    FOREIGN KEY (dataset_id, equipment_id) REFERENCES equipment(dataset_id, equipment_id)
);

CREATE TABLE objects (
    dataset_id BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    object_id TEXT NOT NULL,
    object_name TEXT NOT NULL,
    optional BOOLEAN NOT NULL,
    PRIMARY KEY (dataset_id, object_id)
);

CREATE TABLE requirements (
    dataset_id BIGINT NOT NULL,
    object_id TEXT NOT NULL,
    equipment_type TEXT NOT NULL,
    required_capacity DOUBLE PRECISION NOT NULL CHECK (required_capacity > 0),
    capacity_unit TEXT NOT NULL,
    operating_temperature_c DOUBLE PRECISION NOT NULL,
    installed_equipment_id TEXT,
    available_capacity DOUBLE PRECISION NOT NULL CHECK (available_capacity >= 0),
    position INTEGER NOT NULL,
    PRIMARY KEY (dataset_id, object_id, equipment_type),
    FOREIGN KEY (dataset_id, object_id) REFERENCES objects(dataset_id, object_id),
    FOREIGN KEY (dataset_id, installed_equipment_id) REFERENCES equipment(dataset_id, equipment_id)
);

CREATE TABLE plans (
    id BIGSERIAL PRIMARY KEY,
    dataset_id BIGINT NOT NULL REFERENCES datasets(id),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    result JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE plans;
DROP TABLE requirements;
DROP TABLE objects;
DROP TABLE offers;
DROP TABLE suppliers;
DROP TABLE equipment;
DROP TABLE datasets;
