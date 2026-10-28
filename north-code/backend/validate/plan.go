package validate

type PlanRequest struct {
	Method            string   `json:"method"`
	Budget            *int     `json:"budget"`
	Deadline          *int     `json:"deadline"`
	DisabledSuppliers []string `json:"disabled_suppliers"`
	Name              string   `json:"name"`
}

type SearchRequest struct {
	EquipmentType string   `json:"equipment_type"`
	Capacity      float64  `json:"capacity"`
	Temperature   *float64 `json:"temperature"`
}

type RenameRequest struct {
	Name string `json:"name"`
}
