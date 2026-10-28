-- name: CreateDataset :one
INSERT INTO datasets (name, station_id, station_name, budget, deadline)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ActiveDataset :one
SELECT * FROM datasets ORDER BY id DESC LIMIT 1;

-- name: GetDataset :one
SELECT * FROM datasets WHERE id = $1;

-- name: ImportEquipment :exec
INSERT INTO equipment
SELECT sqlc.arg(dataset_id)::bigint, r->>'equipment_id', r->>'model_name', r->>'equipment_type',
       (r->>'capacity')::double precision, r->>'capacity_unit',
       (r->>'min_operating_temperature_c')::double precision, (r->>'installation_days')::integer
FROM jsonb_array_elements(sqlc.arg(rows)::jsonb) AS r;

-- name: ImportSuppliers :exec
INSERT INTO suppliers
SELECT sqlc.arg(dataset_id)::bigint, r->>'supplier_id', r->>'supplier_name',
       (r->>'delivery_cost_rub')::bigint, (r->>'delivery_days')::integer
FROM jsonb_array_elements(sqlc.arg(rows)::jsonb) AS r;

-- name: ImportOffers :exec
INSERT INTO offers
SELECT sqlc.arg(dataset_id)::bigint, r->>'offer_id', r->>'supplier_id', r->>'equipment_id',
       r->>'equipment_type', (r->>'unit_price_rub')::bigint, (r->>'stock_quantity')::integer
FROM jsonb_array_elements(sqlc.arg(rows)::jsonb) AS r;

-- name: ImportObjects :exec
INSERT INTO objects
SELECT DISTINCT sqlc.arg(dataset_id)::bigint, r->>'object_id', r->>'object_name', (r->>'optional' = 'yes')
FROM jsonb_array_elements(sqlc.arg(rows)::jsonb) AS r;

-- name: ImportRequirements :exec
INSERT INTO requirements
SELECT sqlc.arg(dataset_id)::bigint, r->>'object_id', r->>'equipment_type',
       (r->>'required_capacity')::double precision, r->>'capacity_unit',
       (r->>'operating_temperature_c')::double precision, nullif(r->>'installed_equipment_id','NONE'),
       (r->>'available_capacity')::double precision, ordinal::integer
FROM jsonb_array_elements(sqlc.arg(rows)::jsonb) WITH ORDINALITY AS item(r, ordinal);

-- name: ReadDataset :one
SELECT jsonb_build_object(
    'equipment', coalesce((
        SELECT jsonb_agg((SELECT jsonb_object_agg(key,value) FROM jsonb_each_text(to_jsonb(e)-'dataset_id')) ORDER BY e.equipment_id)
        FROM equipment e WHERE e.dataset_id=d.id
    ), '[]'::jsonb),
    'suppliers', coalesce((
        SELECT jsonb_agg((SELECT jsonb_object_agg(key,value) FROM jsonb_each_text(to_jsonb(s)-'dataset_id')) ORDER BY s.supplier_id)
        FROM suppliers s WHERE s.dataset_id=d.id
    ), '[]'::jsonb),
    'offers', coalesce((
        SELECT jsonb_agg((SELECT jsonb_object_agg(key,value) FROM jsonb_each_text(to_jsonb(o)-'dataset_id')) ORDER BY o.offer_id)
        FROM offers o WHERE o.dataset_id=d.id
    ), '[]'::jsonb),
    'station', coalesce((
        SELECT jsonb_agg(jsonb_build_object(
            'station_id',d.station_id, 'station_name',d.station_name,
            'recommended_budget_rub',d.budget::text, 'deadline_days',d.deadline::text,
            'object_id',r.object_id, 'object_name',b.object_name,
            'optional',CASE WHEN b.optional THEN 'yes' ELSE 'no' END,
            'equipment_type',r.equipment_type, 'required_capacity',r.required_capacity::text,
            'capacity_unit',r.capacity_unit, 'operating_temperature_c',r.operating_temperature_c::text,
            'installed_equipment_id',coalesce(r.installed_equipment_id,'NONE'),
            'available_capacity',r.available_capacity::text
        ) ORDER BY r.position)
        FROM requirements r JOIN objects b ON b.dataset_id=r.dataset_id AND b.object_id=r.object_id
        WHERE r.dataset_id=d.id
    ), '[]'::jsonb)
)::jsonb AS data
FROM datasets d WHERE d.id=$1;
