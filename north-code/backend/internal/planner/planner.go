package planner

import (
	"fmt"
	"sort"
)

type Need struct {
	Key              string   `json:"key"`
	ObjectID         string   `json:"object_id"`
	EquipmentType    string   `json:"equipment_type"`
	RequiredCapacity float64  `json:"required_capacity"`
	Temperature      float64  `json:"operating_temperature_c"`
	Unit             string   `json:"capacity_unit"`
	Optional         string   `json:"optional"`
	Row              Row      `json:"-"`
	Reasons          []string `json:"-"`
}

type Candidate struct {
	OfferID       string `json:"offer_id"`
	SupplierID    string `json:"supplier_id"`
	EquipmentID   string `json:"equipment_id"`
	EquipmentType string `json:"equipment_type"`
	Price         int    `json:"unit_price_rub"`
	Stock         int    `json:"stock_quantity"`
	Ready         int    `json:"ready_days"`
	Model         Row    `json:"model"`
	Supplier      Row    `json:"supplier"`
}

type Assignment struct {
	Key           string `json:"key"`
	ObjectID      string `json:"object_id"`
	EquipmentType string `json:"equipment_type"`
	OfferID       string `json:"offer_id"`
	EquipmentID   string `json:"equipment_id"`
	ModelName     string `json:"model_name"`
	SupplierID    string `json:"supplier_id"`
	SupplierName  string `json:"supplier_name"`
	Price         int    `json:"unit_price_rub"`
	Ready         int    `json:"ready_days"`
}

type Variant struct {
	Selected      []string     `json:"selected"`
	Plan          []Assignment `json:"plan"`
	Cost          int          `json:"cost"`
	EquipmentCost int          `json:"equipment_cost"`
	DeliveryCost  int          `json:"delivery_cost"`
	Overrun       int          `json:"overrun"`
}

type Prepared struct {
	Needs      []Need                 `json:"needs"`
	Candidates map[string][]Candidate `json:"candidates"`
	Optional   []string               `json:"optional"`
	Deadline   int                    `json:"deadline"`
}

func Diagnose(data *Data) ([]map[string]any, []Need) {
	models := index(data.Equipment, "equipment_id")
	diagnosis := []map[string]any{}
	needs := []Need{}

	for _, row := range data.Station {
		reasons := []string{}
		model := models[row["installed_equipment_id"]]
		if model == nil {
			reasons = append(reasons, "Оборудование отсутствует")
		} else {
			if Number(row, "available_capacity") < Number(row, "required_capacity") {
				reasons = append(reasons, "Недостаточная доступная мощность")
			}
			if Number(model, "min_operating_temperature_c") > Number(row, "operating_temperature_c") {
				reasons = append(reasons, "Не подходит температура эксплуатации")
			}
		}

		item := map[string]any{}
		for key, value := range row {
			item[key] = value
		}
		key := row["object_id"] + ":" + row["equipment_type"]
		item["key"], item["reasons"], item["needs_replacement"] = key, reasons, len(reasons) > 0
		diagnosis = append(diagnosis, item)

		if len(reasons) > 0 {
			needs = append(needs, Need{Key: key, ObjectID: row["object_id"], EquipmentType: row["equipment_type"],
				RequiredCapacity: Number(row, "required_capacity"), Temperature: Number(row, "operating_temperature_c"),
				Unit: row["capacity_unit"], Optional: row["optional"], Row: row, Reasons: reasons})
		}
	}
	return diagnosis, needs
}

func Prepare(data *Data, deadline int) (*Prepared, error) {
	_, needs := Diagnose(data)
	result := &Prepared{Needs: needs, Candidates: map[string][]Candidate{}, Optional: []string{}, Deadline: deadline}
	models, suppliers := index(data.Equipment, "equipment_id"), index(data.Suppliers, "supplier_id")
	optional := map[string]bool{}
	for _, row := range data.Station {
		if row["optional"] == "yes" {
			optional[row["object_id"]] = true
		}
	}
	for key := range optional {
		result.Optional = append(result.Optional, key)
	}
	sort.Strings(result.Optional)
	if len(result.Optional) > 10 || len(data.Suppliers) > 12 {
		return nil, fmt.Errorf("поддерживается до 10 необязательных объектов и 12 поставщиков")
	}

	for _, req := range needs {
		items := []Candidate{}
		for _, offer := range data.Offers {
			model, supplier := models[offer["equipment_id"]], suppliers[offer["supplier_id"]]
			ready := Integer(supplier, "delivery_days") + Integer(model, "installation_days")
			if model["equipment_type"] != req.EquipmentType || model["capacity_unit"] != req.Unit {
				continue
			}
			if Number(model, "capacity") < req.RequiredCapacity || Number(model, "min_operating_temperature_c") > req.Temperature {
				continue
			}
			if ready > deadline || Integer(offer, "stock_quantity") == 0 {
				continue
			}
			items = append(items, Candidate{OfferID: offer["offer_id"], SupplierID: offer["supplier_id"],
				EquipmentID: offer["equipment_id"], EquipmentType: offer["equipment_type"],
				Price: Integer(offer, "unit_price_rub"), Stock: Integer(offer, "stock_quantity"), Ready: ready,
				Model: model, Supplier: supplier})
		}
		result.Candidates[req.Key] = items
	}
	return result, nil
}

func (p *Prepared) Required(selected []string) []Need {
	chosen := map[string]bool{}
	for _, key := range selected {
		chosen[key] = true
	}
	items := []Need{}
	for _, req := range p.Needs {
		if req.Optional == "no" || chosen[req.ObjectID] {
			items = append(items, req)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Optional != b.Optional {
			return a.Optional == "no"
		}
		if a.ObjectID != b.ObjectID {
			return a.ObjectID < b.ObjectID
		}
		return a.EquipmentType < b.EquipmentType
	})
	return items
}

func Purchase(req Need, offer Candidate) Assignment {
	return Assignment{Key: req.Key, ObjectID: req.ObjectID, EquipmentType: req.EquipmentType,
		OfferID: offer.OfferID, EquipmentID: offer.EquipmentID, ModelName: offer.Model["model_name"],
		SupplierID: offer.SupplierID, SupplierName: offer.Supplier["supplier_name"], Price: offer.Price, Ready: offer.Ready}
}

func Validate(p *Prepared, variant Variant) error {
	selected := map[string]bool{}
	for _, key := range variant.Selected {
		if selected[key] || !contains(p.Optional, key) {
			return fmt.Errorf("неизвестный или повторяющийся объект: %s", key)
		}
		selected[key] = true
	}
	needs := p.Required(variant.Selected)
	if len(needs) != len(variant.Plan) {
		return fmt.Errorf("не все потребности закрыты ровно один раз")
	}
	required := map[string]Need{}
	for _, req := range needs {
		required[req.Key] = req
	}
	counts := map[string]int{}
	for _, item := range variant.Plan {
		req, ok := required[item.Key]
		if !ok {
			return fmt.Errorf("неизвестная или повторяющаяся потребность")
		}
		found := false
		for _, offer := range p.Candidates[item.Key] {
			if offer.OfferID != item.OfferID {
				continue
			}
			if item != Purchase(req, offer) {
				return fmt.Errorf("неверные характеристики закупки")
			}
			counts[offer.OfferID]++
			if counts[offer.OfferID] > offer.Stock {
				return fmt.Errorf("превышен общий остаток предложения")
			}
			found = true
			break
		}
		if !found {
			return fmt.Errorf("недопустимое предложение %s", item.OfferID)
		}
		delete(required, item.Key)
	}
	return nil
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func Total(data *Data, v Variant, budget int) Variant {
	used := map[string]bool{}
	v.Cost, v.EquipmentCost, v.DeliveryCost = 0, 0, 0
	for _, item := range v.Plan {
		v.EquipmentCost += item.Price
		used[item.SupplierID] = true
	}
	for _, supplier := range data.Suppliers {
		if used[supplier["supplier_id"]] {
			v.DeliveryCost += Integer(supplier, "delivery_cost_rub")
		}
	}
	v.Cost = v.EquipmentCost + v.DeliveryCost
	v.Overrun = max(0, v.Cost-budget)
	return v
}

func better(a, b Variant) bool {
	if a.Overrun != b.Overrun {
		return a.Overrun < b.Overrun
	}
	if len(a.Selected) != len(b.Selected) {
		return len(a.Selected) > len(b.Selected)
	}
	return a.Cost < b.Cost
}

func Result(data *Data, p *Prepared, variants []Variant, budget int, method string) (map[string]any, error) {
	diagnosis, _ := Diagnose(data)
	if len(variants) == 0 {
		missing := []map[string]string{}
		for _, req := range p.Needs {
			if req.Optional == "no" {
				reason := "Требуется совместное распределение доступных остатков"
				if len(p.Candidates[req.Key]) == 0 {
					reason = "Нет подходящих предложений к сроку"
				}
				missing = append(missing, map[string]string{"object_id": req.ObjectID, "equipment_type": req.EquipmentType, "reason": reason})
			}
		}
		status, message := "infeasible", "Обязательное восстановление невозможно"
		if method == "neural" {
			status, message = "not_found", "Нейросеть не нашла допустимый план. Это не доказывает невозможность восстановления."
		}
		return map[string]any{"status": status, "method": method, "message": message, "uncovered": missing,
			"budget": budget, "deadline": p.Deadline, "diagnosis": diagnosis}, nil
	}

	best := Total(data, variants[0], budget)
	frontier := map[int]Variant{}
	for _, variant := range variants {
		if err := Validate(p, variant); err != nil {
			return nil, err
		}
		variant = Total(data, variant, budget)
		if better(variant, best) {
			best = variant
		}
		count := len(variant.Selected)
		if row, ok := frontier[count]; !ok || variant.Cost < row.Cost {
			frontier[count] = variant
		}
	}

	objects := []map[string]any{}
	seen := map[string]bool{}
	ready := 0
	for _, row := range data.Station {
		key := row["object_id"]
		if seen[key] {
			continue
		}
		seen[key] = true
		restored := row["optional"] == "no" || contains(best.Selected, key)
		day := 0
		for _, item := range best.Plan {
			if item.ObjectID == key {
				day = max(day, item.Ready)
			}
		}
		var objectDay any
		if restored {
			objectDay = day
			ready = max(ready, day)
		}
		objects = append(objects, map[string]any{"object_id": key, "name": row["object_name"], "optional": row["optional"] == "yes",
			"restored": restored, "ready_days": objectDay})
	}
	rows := []map[string]any{}
	for count := 0; count <= len(p.Optional); count++ {
		if row, ok := frontier[count]; ok {
			rows = append(rows, map[string]any{"optional_count": count, "cost": row.Cost, "selected": row.Selected})
		}
	}
	warning := ""
	if method == "neural" {
		warning = "Экспериментальная нейросеть обучена на синтетических сценариях. Оптимальность не гарантируется. Точный решатель не вызывается."
	}
	return map[string]any{"status": "ok", "method": method, "optimal": method == "exact", "validated": true,
		"budget": budget, "deadline": p.Deadline, "balance": max(0, budget-best.Cost), "optional_count": len(best.Selected),
		"cost": best.Cost, "overrun": best.Overrun, "equipment_cost": best.EquipmentCost, "delivery_cost": best.DeliveryCost,
		"selected": best.Selected, "plan": best.Plan, "objects": objects, "ready_days": ready,
		"frontier": rows, "diagnosis": diagnosis, "warning": warning}, nil
}
