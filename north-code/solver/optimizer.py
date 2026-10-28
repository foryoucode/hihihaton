import time

from ortools.sat.python import cp_model


def solve(body):
    needs = body['needs']
    candidates = body['candidates']
    optional = body['optional']
    if not isinstance(needs, list) or not isinstance(candidates, dict):
        raise ValueError('Некорректные потребности')
    if not isinstance(optional, list) or len(optional) > 10 or len(set(optional)) != len(optional):
        raise ValueError('Некорректные объекты')

    variants = []
    infeasible = []
    end = time.monotonic() + 90

    for count in range(len(optional) + 1):
        model = cp_model.CpModel()
        selected = {key: model.new_bool_var('object_' + key) for key in optional}
        model.add(sum(selected.values()) == count)
        edges = []
        by_offer = {}
        by_supplier = {}
        stocks = {}
        shipping = {}

        for req in needs:
            choices = []
            for offer in candidates[req['key']]:
                price, stock = offer['unit_price_rub'], offer['stock_quantity']
                if type(price) is not int or price < 0 or type(stock) is not int or stock <= 0:
                    raise ValueError('Некорректная цена или остаток')
                var = model.new_bool_var('x' + str(len(edges)))
                edges.append((req, offer, var))
                choices.append(var)
                by_offer.setdefault(offer['offer_id'], []).append(var)
                by_supplier.setdefault(offer['supplier_id'], []).append(var)
                stocks[offer['offer_id']] = stock
                shipping[offer['supplier_id']] = int(offer['supplier']['delivery_cost_rub'])

            target = selected[req['object_id']] if req['optional'] == 'yes' else 1
            model.add(sum(choices) == target)

        for key, variables in by_offer.items():
            model.add(sum(variables) <= stocks[key])

        used = {}
        for key, variables in by_supplier.items():
            used[key] = model.new_bool_var('supplier_' + key)
            for var in variables:
                model.add(var <= used[key])
            model.add(used[key] <= sum(variables))

        model.minimize(sum(offer['unit_price_rub'] * var for _, offer, var in edges)
                       + sum(shipping[key] * var for key, var in used.items()))
        solver = cp_model.CpSolver()
        remaining = end - time.monotonic()
        if remaining <= 0:
            raise TimeoutError('OR-Tools не успел доказать оптимальность')
        solver.parameters.max_time_in_seconds = remaining
        solver.parameters.num_search_workers = 2
        solver.parameters.random_seed = 42
        status = solver.solve(model)

        if status == cp_model.INFEASIBLE:
            infeasible.append(count)
            continue
        if status != cp_model.OPTIMAL:
            raise TimeoutError('OR-Tools не подтвердил оптимум: ' + solver.status_name(status))

        plan = []
        for req, offer, var in edges:
            if not solver.value(var):
                continue
            plan.append({
                'key': req['key'], 'object_id': req['object_id'], 'equipment_type': req['equipment_type'],
                'offer_id': offer['offer_id'], 'equipment_id': offer['equipment_id'],
                'model_name': offer['model']['model_name'], 'supplier_id': offer['supplier_id'],
                'supplier_name': offer['supplier']['supplier_name'],
                'unit_price_rub': offer['unit_price_rub'], 'ready_days': offer['ready_days']
            })

        variants.append({'selected': [key for key in optional if solver.value(selected[key])], 'plan': plan})

    return {'variants': variants, 'infeasible_counts': infeasible, 'complete': True, 'solver': 'OR-Tools CP-SAT'}
