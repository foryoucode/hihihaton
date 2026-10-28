package planner

import (
	"context"
	"math"
	"sort"
)

type edge struct {
	to       int
	reverse  int
	capacity int
	cost     int
}

type graph [][]edge

func (g graph) add(from, to, capacity, cost int) {
	forward := edge{to: to, reverse: len(g[to]), capacity: capacity, cost: cost}
	backward := edge{to: from, reverse: len(g[from]), cost: -cost}
	g[from] = append(g[from], forward)
	g[to] = append(g[to], backward)
}

func (g graph) flow(ctx context.Context, source, sink, target int) (bool, error) {
	for sent := 0; sent < target; sent++ {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		distance := make([]int, len(g))
		previous, paths := make([]int, len(g)), make([]int, len(g))
		queued := make([]bool, len(g))
		for i := range distance {
			distance[i] = math.MaxInt / 4
		}
		distance[source] = 0
		queue := []int{source}
		queued[source] = true

		for head := 0; head < len(queue); head++ {
			from := queue[head]
			queued[from] = false
			for i, e := range g[from] {
				if e.capacity == 0 || distance[e.to] <= distance[from]+e.cost {
					continue
				}
				distance[e.to] = distance[from] + e.cost
				previous[e.to], paths[e.to] = from, i
				if !queued[e.to] {
					queue = append(queue, e.to)
					queued[e.to] = true
				}
			}
		}
		if distance[sink] == math.MaxInt/4 {
			return false, nil
		}

		for to := sink; to != source; {
			from, idx := previous[to], paths[to]
			e := g[from][idx]
			g[from][idx].capacity--
			g[to][e.reverse].capacity++
			to = from
		}
	}
	return true, nil
}

func assign(ctx context.Context, required []Need, candidates map[string][]Candidate, allowed map[string]bool) ([]Assignment, bool, error) {
	offers := map[string]Candidate{}
	for _, req := range required {
		for _, offer := range candidates[req.Key] {
			if allowed[offer.SupplierID] {
				offers[offer.OfferID] = offer
			}
		}
	}
	ids := []string{}
	for id := range offers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	offerNodes := map[string]int{}
	sink := 1 + len(required) + len(ids)
	g := make(graph, sink+1)
	for i, id := range ids {
		node := 1 + len(required) + i
		offerNodes[id] = node
		g.add(node, sink, offers[id].Stock, 0)
	}

	type reference struct {
		from, index int
		req         Need
		offer       Candidate
	}
	refs := []reference{}
	for i, req := range required {
		node := i + 1
		g.add(0, node, 1, 0)
		for _, offer := range candidates[req.Key] {
			if !allowed[offer.SupplierID] {
				continue
			}
			refs = append(refs, reference{node, len(g[node]), req, offer})
			g.add(node, offerNodes[offer.OfferID], 1, offer.Price)
		}
	}
	complete, err := g.flow(ctx, 0, sink, len(required))
	if err != nil || !complete {
		return nil, false, err
	}

	plan := []Assignment{}
	for _, ref := range refs {
		if g[ref.from][ref.index].capacity == 0 {
			plan = append(plan, Purchase(ref.req, ref.offer))
		}
	}
	return plan, true, nil
}

func Exact(ctx context.Context, data *Data, p *Prepared) ([]Variant, error) {
	active := map[string]bool{}
	for _, offers := range p.Candidates {
		for _, offer := range offers {
			active[offer.SupplierID] = true
		}
	}
	suppliers := []Row{}
	for _, row := range data.Suppliers {
		if active[row["supplier_id"]] {
			suppliers = append(suppliers, row)
		}
	}
	sort.Slice(suppliers, func(i, j int) bool { return suppliers[i]["supplier_id"] < suppliers[j]["supplier_id"] })
	bits := map[string]int{}
	for i, row := range suppliers {
		bits[row["supplier_id"]] = 1 << i
	}
	type supplierSet struct{ mask, shipping int }
	sets := []supplierSet{}
	for mask := 0; mask < 1<<len(suppliers); mask++ {
		cost := 0
		for i, row := range suppliers {
			if mask&(1<<i) != 0 {
				cost += Integer(row, "delivery_cost_rub")
			}
		}
		sets = append(sets, supplierSet{mask, cost})
	}
	sort.SliceStable(sets, func(i, j int) bool { return sets[i].shipping < sets[j].shipping })
	best := map[int]Variant{}

	for mask := 0; mask < 1<<len(p.Optional); mask++ {
		selected := []string{}
		for i, key := range p.Optional {
			if mask&(1<<i) != 0 {
				selected = append(selected, key)
			}
		}
		required := p.Required(selected)
		count := len(selected)
		if len(required) == 0 {
			best[count] = Total(data, Variant{Selected: selected, Plan: []Assignment{}}, 0)
			continue
		}

		for _, set := range sets {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			current, found := best[count]
			if found && set.shipping >= current.Cost {
				continue
			}
			lower := set.shipping
			possible := true
			for _, req := range required {
				minimum := math.MaxInt / 4
				for _, offer := range p.Candidates[req.Key] {
					if bits[offer.SupplierID]&set.mask != 0 {
						minimum = min(minimum, offer.Price)
					}
				}
				if minimum == math.MaxInt/4 {
					possible = false
					break
				}
				lower += minimum
			}
			if !possible || (found && lower >= current.Cost) {
				continue
			}

			allowed := map[string]bool{}
			for key, bit := range bits {
				allowed[key] = bit&set.mask != 0
			}
			plan, complete, err := assign(ctx, required, p.Candidates, allowed)
			if err != nil {
				return nil, err
			}
			if !complete {
				continue
			}
			variant := Total(data, Variant{Selected: selected, Plan: plan}, 0)
			if !found || variant.Cost < current.Cost {
				best[count] = variant
			}
		}
	}
	variants := []Variant{}
	for count := 0; count <= len(p.Optional); count++ {
		if variant, ok := best[count]; ok {
			variants = append(variants, variant)
		}
	}
	return variants, nil
}
