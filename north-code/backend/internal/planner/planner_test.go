package planner

import (
	"context"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"
)

func sourceData(t *testing.T) *Data {
	t.Helper()
	data, err := Load(filepath.Join("..", "..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExactReference(t *testing.T) {
	data := sourceData(t)
	prepared, err := Prepare(data, 14)
	if err != nil {
		t.Fatal(err)
	}
	variants, err := Exact(context.Background(), data, prepared)
	if err != nil {
		t.Fatal(err)
	}

	costs := []int{}
	for _, variant := range variants {
		if err := Validate(prepared, variant); err != nil {
			t.Fatal(err)
		}
		costs = append(costs, variant.Cost)
	}
	if !reflect.DeepEqual(costs, []int{1824000, 2027000, 2231000, 2482000, 2782000, 3633000}) {
		t.Fatal("неверная граница стоимости", costs)
	}
	result, err := Result(data, prepared, variants, 2600000, "exact")
	if err != nil {
		t.Fatal(err)
	}
	if result["cost"] != 2482000 || result["optional_count"] != 3 || result["delivery_cost"] != 270000 || result["ready_days"] != 14 {
		t.Fatal("неверный эталонный результат", result)
	}
	if !reflect.DeepEqual(result["selected"], []string{"M01", "M02", "M05"}) {
		t.Fatal("выбраны неверные пункты", result["selected"])
	}

	for _, row := range []struct{ budget, cost, count int }{{1000000, 1824000, 0}, {2782000, 2782000, 4}} {
		result, err := Result(data, prepared, variants, row.budget, "exact")
		if err != nil || result["cost"] != row.cost || result["optional_count"] != row.count {
			t.Fatal("нарушены приоритеты бюджета", result, err)
		}
	}
}

func TestTemperatureDiagnosisAndDeadline(t *testing.T) {
	data := sourceData(t)
	_, needs := Diagnose(data)
	if len(needs) != 14 {
		t.Fatal("неверное число замен", len(needs))
	}
	for _, req := range needs {
		if req.Key == "B02:Топливный отопитель" && !reflect.DeepEqual(req.Reasons, []string{"Не подходит температура эксплуатации"}) {
			t.Fatal("температурная ловушка не обнаружена", req)
		}
	}
	prepared, _ := Prepare(data, 1)
	variants, err := Exact(context.Background(), data, prepared)
	if err != nil || len(variants) != 0 {
		t.Fatal("невозможный срок принят", err)
	}
}

func TestRejectsInvalidNeuralAssignments(t *testing.T) {
	data := sourceData(t)
	p, _ := Prepare(data, 14)
	variants, _ := Exact(context.Background(), data, p)
	v := variants[0]
	v.Plan[0].ObjectID = "OTHER"
	if Validate(p, v) == nil {
		t.Fatal("подмена объекта не обнаружена")
	}
	v.Plan[0].ObjectID = v.Plan[0].Key[:3]
	v.Plan = append(v.Plan, v.Plan[0])
	if Validate(p, v) == nil {
		t.Fatal("повтор закупки не обнаружен")
	}
}

func TestSharedStockCannotBeUsedTwice(t *testing.T) {
	req1 := Need{Key: "B1:gen", ObjectID: "B1", Optional: "no"}
	req2 := Need{Key: "B2:gen", ObjectID: "B2", Optional: "no"}
	offer := Candidate{OfferID: "O1", SupplierID: "S1", Stock: 1, Price: 10}
	p := &Prepared{Needs: []Need{req1, req2}, Candidates: map[string][]Candidate{req1.Key: {offer}, req2.Key: {offer}}}
	data := &Data{Suppliers: []Row{{"supplier_id": "S1", "delivery_cost_rub": "5"}}}
	variants, err := Exact(context.Background(), data, p)
	if err != nil || len(variants) != 0 {
		t.Fatal("общий остаток потрачен дважды", variants, err)
	}
}

func TestExactMatchesExhaustiveSearch(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		rng := rand.New(rand.NewSource(seed))
		data := &Data{Suppliers: []Row{{"supplier_id": "S1", "delivery_cost_rub": "15"}, {"supplier_id": "S2", "delivery_cost_rub": "30"}}}
		p := &Prepared{Optional: []string{"M1", "M2"}, Candidates: map[string][]Candidate{}}
		offers := []Candidate{}
		for i, id := range []string{"O1", "O2", "O3", "O4"} {
			supplier := data.Suppliers[i%2]
			offers = append(offers, Candidate{OfferID: id, SupplierID: supplier["supplier_id"], Stock: 1 + rng.Intn(2), Price: 1 + rng.Intn(50), Supplier: supplier})
		}
		for i, id := range []string{"B1", "M1", "M2"} {
			req := Need{Key: id + ":gen", ObjectID: id, Optional: "yes"}
			if i == 0 {
				req.Optional = "no"
			}
			p.Needs = append(p.Needs, req)
			for _, offer := range offers {
				if rng.Intn(4) != 0 {
					p.Candidates[req.Key] = append(p.Candidates[req.Key], offer)
				}
			}
		}
		best := map[int]int{}
		for mask := 0; mask < 4; mask++ {
			selected := []string{}
			for i, id := range p.Optional {
				if mask&(1<<i) != 0 {
					selected = append(selected, id)
				}
			}
			required := p.Required(selected)
			counts := map[string]int{}
			var visit func(int, []Assignment)
			visit = func(i int, plan []Assignment) {
				if i == len(required) {
					cost := Total(data, Variant{Plan: plan}, 0).Cost
					prev, ok := best[len(selected)]
					if !ok || cost < prev {
						best[len(selected)] = cost
					}
					return
				}
				req := required[i]
				for _, offer := range p.Candidates[req.Key] {
					if counts[offer.OfferID] >= offer.Stock {
						continue
					}
					counts[offer.OfferID]++
					visit(i+1, append(plan, Purchase(req, offer)))
					counts[offer.OfferID]--
				}
			}
			visit(0, nil)
		}
		actual, err := Exact(context.Background(), data, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(actual) != len(best) {
			t.Fatalf("seed %d: неверное число допустимых вариантов", seed)
		}
		for _, v := range actual {
			if v.Cost != best[len(v.Selected)] {
				t.Fatalf("seed %d: %d вместо %d", seed, v.Cost, best[len(v.Selected)])
			}
		}
	}
}

func TestCancellationAndNumbers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := sourceData(t)
	p, _ := Prepare(data, 14)
	if _, err := Exact(ctx, data, p); err == nil {
		t.Fatal("отмена расчёта проигнорирована")
	}
	data.Equipment[0]["capacity"] = "NaN"
	if data.Validate() == nil {
		t.Fatal("NaN принят")
	}
}
