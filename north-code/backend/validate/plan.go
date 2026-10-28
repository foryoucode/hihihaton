package validate

type PlanRequest struct {
	Method            string   `json:"method"`
	Budget            int      `json:"budget"`
	Deadline          int      `json:"deadline"`
	DisabledSuppliers []string `json:"disabled_suppliers"`
	Name              string   `json:"name"`
}
