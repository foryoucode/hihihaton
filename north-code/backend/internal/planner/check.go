package planner

import "fmt"

func CheckPlan(data *Data, variant Variant, deadline int) error {
	models := index(data.Equipment, "equipment_id")
	offers := index(data.Offers, "offer_id")
	suppliers := index(data.Suppliers, "supplier_id")
	optional := map[string]bool{}
	for _, r := range data.Station {
		if r["optional"] == "yes" {
			optional[r["object_id"]] = true
		}
	}
	selected := map[string]bool{}
	for _, id := range variant.Selected {
		if !optional[id] || selected[id] {
			return fmt.Errorf("неверный список восстановленных пунктов")
		}
		selected[id] = true
	}
	assigned := map[string]Assignment{}
	counts := map[string]int{}
	for _, item := range variant.Plan {
		if _, exists := assigned[item.Key]; exists {
			return fmt.Errorf("повтор потребности: %s", item.Key)
		}
		assigned[item.Key] = item
		counts[item.OfferID]++
	}
	for _, r := range data.Station {
		key := r["object_id"] + ":" + r["equipment_type"]
		item, bought := assigned[key]
		restored := r["optional"] == "no" || selected[r["object_id"]]
		if !restored {
			if bought {
				return fmt.Errorf("закупка для закрытого объекта")
			}
			continue
		}
		if !bought {
			old := models[r["installed_equipment_id"]]
			if old == nil || Number(r, "available_capacity") < Number(r, "required_capacity") || Number(old, "min_operating_temperature_c") > Number(r, "operating_temperature_c") {
				return fmt.Errorf("незакрытая потребность: %s", key)
			}
			continue
		}
		offer := offers[item.OfferID]
		if offer == nil {
			return fmt.Errorf("неизвестное предложение: %s", item.OfferID)
		}
		model, supplier := models[offer["equipment_id"]], suppliers[offer["supplier_id"]]
		if model == nil || supplier == nil {
			return fmt.Errorf("нарушены связи оборудования")
		}
		if model["equipment_type"] != r["equipment_type"] || model["capacity_unit"] != r["capacity_unit"] || Number(model, "capacity") < Number(r, "required_capacity") || Number(model, "min_operating_temperature_c") > Number(r, "operating_temperature_c") {
			return fmt.Errorf("не подходят характеристики: %s", key)
		}
		ready := Integer(supplier, "delivery_days") + Integer(model, "installation_days")
		if ready > deadline {
			return fmt.Errorf("закупка опаздывает: %s", key)
		}
		if counts[item.OfferID] > Integer(offer, "stock_quantity") {
			return fmt.Errorf("превышен общий остаток: %s", item.OfferID)
		}
		expected := Assignment{Key: key, ObjectID: r["object_id"], EquipmentType: r["equipment_type"], OfferID: offer["offer_id"], EquipmentID: model["equipment_id"], ModelName: model["model_name"], SupplierID: supplier["supplier_id"], SupplierName: supplier["supplier_name"], Price: Integer(offer, "unit_price_rub"), Ready: ready}
		if item != expected {
			return fmt.Errorf("неверные параметры закупки: %s", key)
		}
		delete(assigned, key)
	}
	if len(assigned) != 0 {
		return fmt.Errorf("закупка для неизвестной потребности")
	}
	return nil
}
