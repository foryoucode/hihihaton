package validate

type PlanRequest struct {
	DatasetID         *int64   `json:"dataset_id"`
	Method            string   `json:"method"`
	Budget            *int     `json:"budget"`
	Deadline          *int     `json:"deadline"`
	DisabledSuppliers []string `json:"disabled_suppliers"`
	Name              string   `json:"name"`
}

type SearchRequest struct {
	DatasetID     *int64   `json:"dataset_id"`
	EquipmentType string   `json:"equipment_type"`
	Capacity      float64  `json:"capacity"`
	Temperature   *float64 `json:"temperature"`
}

type RenameRequest struct {
	Name string `json:"name"`
}
