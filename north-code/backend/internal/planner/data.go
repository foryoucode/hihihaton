package planner

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Row map[string]string

type Data struct {
	Equipment []Row `json:"equipment"`
	Offers    []Row `json:"offers"`
	Suppliers []Row `json:"suppliers"`
	Station   []Row `json:"station"`
}

func Number(row Row, field string) float64 {
	value, _ := strconv.ParseFloat(row[field], 64)
	return value
}

func Integer(row Row, field string) int {
	value, _ := strconv.Atoi(row[field])
	return value
}

func ReadCSV(path string) ([]Row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	seen := map[string]bool{}
	for _, field := range header {
		if field == "" || seen[field] {
			return nil, fmt.Errorf("повтор или пустое имя столбца: %s", path)
		}
		seen[field] = true
	}

	rows := []Row{}
	for {
		values, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		row := Row{}
		for i, field := range header {
			row[field] = values[i]
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("пустой CSV: %s", path)
	}

	return rows, nil
}

func Load(dir string) (*Data, error) {
	data := &Data{}
	files := []struct {
		name string
		rows *[]Row
	}{
		{"equipment", &data.Equipment}, {"offers", &data.Offers},
		{"suppliers", &data.Suppliers}, {"station_1", &data.Station},
	}
	for _, file := range files {
		rows, err := ReadCSV(filepath.Join(dir, file.name+".csv"))
		if err != nil {
			return nil, err
		}
		*file.rows = rows
	}
	if err := data.Validate(); err != nil {
		return nil, err
	}

	return data, nil
}

func index(rows []Row, key string) map[string]Row {
	result := map[string]Row{}
	for _, row := range rows {
		result[row[key]] = row
	}
	return result
}

func checkRows(rows []Row, key string, integers, decimals, fields []string) error {
	seen := map[string]bool{}
	for _, row := range rows {
		id := row[key]
		if id == "" || seen[id] {
			return fmt.Errorf("пустой или повторяющийся %s", key)
		}
		seen[id] = true

		for _, field := range fields {
			if row[field] == "" {
				return fmt.Errorf("отсутствует %s у %s", field, id)
			}
		}
		for _, field := range integers {
			value, err := strconv.Atoi(row[field])
			if err != nil || value < 0 || value > 1000000000 {
				return fmt.Errorf("некорректное %s у %s", field, id)
			}
		}
		for _, field := range decimals {
			value, err := strconv.ParseFloat(row[field], 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("некорректное %s у %s", field, id)
			}
		}
	}
	return nil
}

func ValidateEquipment(rows []Row) error {
	if err := checkRows(rows, "equipment_id", []string{"installation_days"},
		[]string{"capacity", "min_operating_temperature_c"}, []string{"model_name", "equipment_type", "capacity_unit"}); err != nil {
		return err
	}
	for _, row := range rows {
		if Number(row, "capacity") <= 0 {
			return fmt.Errorf("некорректная производительность модели")
		}
	}
	return nil
}

func (data *Data) Validate() error {
	if len(data.Station) == 0 || len(data.Equipment) == 0 || len(data.Suppliers) == 0 {
		return fmt.Errorf("нет данных станции, оборудования или поставщиков")
	}
	if err := ValidateEquipment(data.Equipment); err != nil {
		return err
	}
	if err := checkRows(data.Suppliers, "supplier_id", []string{"delivery_cost_rub", "delivery_days"}, nil, []string{"supplier_name"}); err != nil {
		return err
	}
	if err := checkRows(data.Offers, "offer_id", []string{"unit_price_rub", "stock_quantity"}, nil, []string{"supplier_id", "equipment_id", "equipment_type"}); err != nil {
		return err
	}

	models, suppliers := index(data.Equipment, "equipment_id"), index(data.Suppliers, "supplier_id")
	for _, row := range data.Offers {
		model := models[row["equipment_id"]]
		if model == nil || suppliers[row["supplier_id"]] == nil || row["equipment_type"] != model["equipment_type"] {
			return fmt.Errorf("неверные связи предложения %s", row["offer_id"])
		}
	}

	seen := map[string]bool{}
	objects := map[string]string{}
	for _, row := range data.Station {
		key := row["object_id"] + ":" + row["equipment_type"]
		if err := checkRows([]Row{row}, "object_id", []string{"recommended_budget_rub", "deadline_days"},
			[]string{"required_capacity", "available_capacity", "operating_temperature_c"},
			[]string{"station_id", "station_name", "object_name", "equipment_type", "capacity_unit", "installed_equipment_id"}); err != nil {
			return err
		}
		if seen[key] || (row["optional"] != "yes" && row["optional"] != "no") {
			return fmt.Errorf("неверная потребность %s", key)
		}
		seen[key] = true
		if Number(row, "required_capacity") <= 0 || Number(row, "available_capacity") < 0 || Integer(row, "deadline_days") < 1 {
			return fmt.Errorf("некорректная мощность или срок")
		}
		for _, field := range []string{"station_id", "recommended_budget_rub", "deadline_days"} {
			if row[field] != data.Station[0][field] {
				return fmt.Errorf("несогласованный параметр станции: %s", field)
			}
		}
		object := row["object_name"] + ":" + row["optional"]
		if previous, ok := objects[row["object_id"]]; ok && previous != object {
			return fmt.Errorf("несогласованные свойства объекта")
		}
		objects[row["object_id"]] = object
		if row["installed_equipment_id"] != "NONE" {
			model := models[row["installed_equipment_id"]]
			if model == nil || model["equipment_type"] != row["equipment_type"] || model["capacity_unit"] != row["capacity_unit"] {
				return fmt.Errorf("неверная установленная модель у %s", key)
			}
		}
	}
	return nil
}
