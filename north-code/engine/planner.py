import copy
import csv
import itertools
import math
from collections import Counter
from pathlib import Path

import numpy as np
from scipy.optimize import Bounds, LinearConstraint, milp
from scipy.sparse import lil_matrix

from common import assignment, selected_needs


ROOT = Path(__file__).resolve().parents[1]


def load_data(catalog_only=False):
    data = {}
    names = ['equipment'] if catalog_only else ['equipment', 'offers', 'suppliers', 'station_1']

    for name in names:
        with (ROOT / 'data' / (name + '.csv')).open(encoding='utf-8-sig') as file:
            rows = list(csv.DictReader(file, delimiter=';'))

        data['station' if name == 'station_1' else name] = rows

    if not catalog_only:
        validate_data(data)

    return data


def validate_data(data):
    for name, key in [('equipment', 'equipment_id'), ('offers', 'offer_id'), ('suppliers', 'supplier_id')]:
        rows = data[name]

        if not rows or len({r[key] for r in rows}) != len(rows):
            raise ValueError('Пустые данные или повторяющиеся ID: ' + name)

    equipment = {r['equipment_id']: r for r in data['equipment']}
    suppliers = {r['supplier_id']: r for r in data['suppliers']}

    for r in data['equipment']:
        for field in ['capacity', 'min_operating_temperature_c', 'installation_days']:
            if not math.isfinite(float(r[field])):
                raise ValueError('Некорректная характеристика модели')

        if float(r['capacity']) <= 0 or int(r['installation_days']) < 0:
            raise ValueError('Некорректная мощность или установка')

    for r in data['suppliers']:
        if int(r['delivery_cost_rub']) < 0 or int(r['delivery_days']) < 0:
            raise ValueError('Некорректные условия доставки')

    for r in data['offers']:
        if r['equipment_id'] not in equipment or r['supplier_id'] not in suppliers:
            raise ValueError('Неизвестная модель или поставщик')

        if r['equipment_type'] != equipment[r['equipment_id']]['equipment_type']:
            raise ValueError('Тип предложения не совпадает с моделью')

        if int(r['unit_price_rub']) < 0 or int(r['stock_quantity']) < 0:
            raise ValueError('Отрицательная цена или остаток')

    station = data['station']

    if not station:
        raise ValueError('Нет отчёта станции')

    for field in ['station_id', 'recommended_budget_rub', 'deadline_days']:
        if len({r[field] for r in station}) != 1:
            raise ValueError('Несогласованные параметры станции')

    if int(station[0]['recommended_budget_rub']) < 0 or int(station[0]['deadline_days']) < 1:
        raise ValueError('Некорректные бюджет или срок')

    keys = set()
    objects = {}

    for r in station:
        key = (r['object_id'], r['equipment_type'])

        if key in keys or r['optional'] not in ['yes', 'no']:
            raise ValueError('Повтор потребности или неверное optional')

        keys.add(key)
        obj = (r['object_name'], r['optional'])

        if r['object_id'] in objects and objects[r['object_id']] != obj:
            raise ValueError('Несогласованные свойства объекта')

        objects[r['object_id']] = obj

        for field in ['required_capacity', 'available_capacity', 'operating_temperature_c']:
            if not math.isfinite(float(r[field])):
                raise ValueError('Некорректная потребность')

        if float(r['required_capacity']) <= 0 or float(r['available_capacity']) < 0:
            raise ValueError('Некорректная мощность потребности')

        old = r['installed_equipment_id']

        if old != 'NONE':
            if old not in equipment:
                raise ValueError('Неизвестное установленное оборудование')

            model = equipment[old]

            if model['equipment_type'] != r['equipment_type'] or model['capacity_unit'] != r['capacity_unit']:
                raise ValueError('Несовпадение типа или единиц установленного оборудования')


def diagnose(data):
    models = {r['equipment_id']: r for r in data['equipment']}
    rows = []

    for r in data['station']:
        row = dict(r)
        reasons = []
        old = models.get(r['installed_equipment_id'])

        if old is None:
            reasons.append('Оборудование отсутствует')
        else:
            if float(r['available_capacity']) < float(r['required_capacity']):
                reasons.append('Недостаточная доступная мощность')

            if float(old['min_operating_temperature_c']) > float(r['operating_temperature_c']):
                reasons.append('Не подходит температура эксплуатации')

        row['reasons'] = reasons
        row['needs_replacement'] = bool(reasons)
        row['key'] = r['object_id'] + ':' + r['equipment_type']
        rows.append(row)

    return rows


def find_equipment(data, equipment_type, capacity, temperature):
    return [r for r in data['equipment'] if r['equipment_type'] == equipment_type
            and float(r['capacity']) >= capacity
            and float(r['min_operating_temperature_c']) <= temperature]


def prepare(data, deadline):
    models = {r['equipment_id']: r for r in data['equipment']}
    suppliers = {r['supplier_id']: r for r in data['suppliers']}
    needs = [r for r in diagnose(data) if r['needs_replacement']]
    candidates = {}

    for r in needs:
        items = []

        for offer in data['offers']:
            model = models[offer['equipment_id']]
            supplier = suppliers[offer['supplier_id']]
            ready = int(supplier['delivery_days']) + int(model['installation_days'])

            if model['equipment_type'] != r['equipment_type'] or model['capacity_unit'] != r['capacity_unit']:
                continue

            if float(model['capacity']) < float(r['required_capacity']):
                continue

            if float(model['min_operating_temperature_c']) > float(r['operating_temperature_c']):
                continue

            if ready > deadline or int(offer['stock_quantity']) == 0:
                continue

            items.append({**offer, 'unit_price_rub': int(offer['unit_price_rub']),
                          'stock_quantity': int(offer['stock_quantity']), 'ready_days': ready,
                          'model': model, 'supplier': supplier})

        candidates[r['key']] = items

    return needs, candidates




def exact_subset(needs, candidates, selected, time_limit=15):
    required = selected_needs(needs, selected)

    if any(not candidates[r['key']] for r in required):
        return None

    if not required:
        return []

    edges = [(r, p) for r in required for p in candidates[r['key']]]
    suppliers = {p['supplier_id']: p['supplier'] for _, p in edges}
    offers = {p['offer_id']: p for _, p in edges}
    supplier_ids = sorted(suppliers)
    offer_ids = sorted(offers)
    req_index = {r['key']: i for i, r in enumerate(required)}
    offer_index = {key: i + len(required) for i, key in enumerate(offer_ids)}
    supplier_index = {key: i + len(required) + len(offer_ids) for i, key in enumerate(supplier_ids)}
    n = len(edges)
    total = n + len(suppliers)
    a = lil_matrix((len(required) + len(offers) + len(suppliers), total))
    lower = np.full(a.shape[0], -np.inf)
    upper = np.zeros(a.shape[0])
    cost = np.zeros(total)

    for i in range(len(required)):
        lower[i] = upper[i] = 1

    for key in offer_ids:
        upper[offer_index[key]] = offers[key]['stock_quantity']

    for i, key in enumerate(supplier_ids):
        cost[n + i] = int(suppliers[key]['delivery_cost_rub'])
        a[supplier_index[key], n + i] = -len(required)

    for i, (r, p) in enumerate(edges):
        cost[i] = p['unit_price_rub']
        a[req_index[r['key']], i] = 1
        a[offer_index[p['offer_id']], i] = 1
        a[supplier_index[p['supplier_id']], i] = 1

    result = milp(cost, integrality=np.ones(total), bounds=Bounds(0, 1),
                  constraints=LinearConstraint(a.tocsr(), lower, upper),
                  options={'mip_rel_gap': 0.0, 'time_limit': time_limit})

    if result.status == 2:
        return None

    if result.status != 0:
        raise RuntimeError('Решатель не подтвердил оптимум: ' + result.message)

    return [assignment(r, p) for i, (r, p) in enumerate(edges) if result.x[i] > 0.5]




def totals(data, plan):
    suppliers = {r['supplier_id']: r for r in data['suppliers']}
    used = sorted({r['supplier_id'] for r in plan})
    equipment = sum(r['unit_price_rub'] for r in plan)
    shipping = sum(int(suppliers[key]['delivery_cost_rub']) for key in used)

    return equipment, shipping, equipment + shipping


def validate_plan(data, plan, selected, deadline):
    needs, candidates = prepare(data, deadline)
    required = selected_needs(needs, selected)
    errors = []
    keys = [r['key'] for r in plan]

    if set(keys) != {r['key'] for r in required} or len(keys) != len(set(keys)):
        errors.append('Не все потребности выбранных объектов закрыты ровно один раз')

    offers = {p['offer_id']: p for p in data['offers']}

    for r in plan:
        correct = next((p for p in candidates.get(r['key'], []) if p['offer_id'] == r['offer_id']), None)

        if correct is None:
            errors.append('Недопустимое предложение: ' + r['offer_id'])
        elif any(r.get(field) != correct[field] for field in ['equipment_id', 'supplier_id', 'unit_price_rub', 'ready_days']):
            errors.append('Неверные характеристики закупки: ' + r['offer_id'])

    for key, count in Counter(r['offer_id'] for r in plan).items():
        if key not in offers or count > int(offers[key]['stock_quantity']):
            errors.append('Превышен остаток: ' + key)

    return errors


def solve(data, budget, deadline, method='exact', model=None):
    if method not in ['exact', 'neural']:
        raise ValueError('Неизвестный метод')

    if method == 'neural' and model is None:
        raise ValueError('Нейросеть не обучена. Запустите engine/train.py')

    needs, candidates = prepare(data, deadline)
    optional = sorted({r['object_id'] for r in data['station'] if r['optional'] == 'yes'})

    if len(optional) > 10:
        raise ValueError('В демонстрационном режиме поддерживается до 10 необязательных объектов')

    variants = []

    for mask in itertools.product([False, True], repeat=len(optional)):
        selected = [key for key, flag in zip(optional, mask) if flag]

        if method == 'exact':
            plan = exact_subset(needs, candidates, selected)
        else:
            from neural import neural_subset
            plan = neural_subset(needs, candidates, selected, deadline, model)

        if plan is None:
            continue

        errors = validate_plan(data, plan, selected, deadline)

        if errors:
            raise RuntimeError('; '.join(errors))

        equipment, shipping, cost = totals(data, plan)
        variants.append({'selected': selected, 'plan': plan, 'cost': cost,
                         'equipment_cost': equipment, 'delivery_cost': shipping,
                         'overrun': max(0, cost - budget)})

    if not variants:
        missing = [{'object_id': r['object_id'], 'equipment_type': r['equipment_type'],
                    'reason': 'Нет подходящих предложений к сроку' if not candidates[r['key']]
                    else 'Требуется совместное распределение доступных остатков'}
                   for r in needs if r['optional'] == 'no']

        return {'status': 'infeasible' if method == 'exact' else 'not_found', 'method': method,
                'message': 'Обязательное восстановление невозможно' if method == 'exact'
                else 'Нейросеть не нашла допустимый план. Это не доказывает невозможность восстановления.',
                'uncovered': missing, 'budget': budget, 'deadline': deadline, 'diagnosis': diagnose(data)}

    best = min(variants, key=lambda r: (r['overrun'], -len(r['selected']), r['cost']))
    objects = []

    for key in dict.fromkeys(r['object_id'] for r in data['station']):
        row = next(r for r in data['station'] if r['object_id'] == key)
        restored = row['optional'] == 'no' or key in best['selected']
        ready = max([p['ready_days'] for p in best['plan'] if p['object_id'] == key], default=0)
        objects.append({'object_id': key, 'name': row['object_name'], 'optional': row['optional'] == 'yes',
                        'restored': restored, 'ready_days': ready if restored else None})

    frontier = []

    for count in range(len(optional) + 1):
        rows = [r for r in variants if len(r['selected']) == count]

        if rows:
            row = min(rows, key=lambda r: r['cost'])
            frontier.append({'optional_count': count, 'cost': row['cost'], 'selected': row['selected']})

    return {**best, 'status': 'ok', 'method': method, 'optimal': method == 'exact',
            'validated': True, 'budget': budget, 'deadline': deadline,
            'balance': max(0, budget - best['cost']), 'optional_count': len(best['selected']),
            'ready_days': max([r['ready_days'] for r in best['plan']], default=0),
            'objects': objects, 'frontier': frontier, 'diagnosis': diagnose(data),
            'warning': '' if method == 'exact' else 'Экспериментальная нейросеть обучена на синтетических сценариях. Оптимальность не гарантируется. MILP при выборе этого метода не вызывается.'}


def make_scenario(base, seed):
    rng = np.random.default_rng(seed)
    data = copy.deepcopy(base)

    for r in data['offers']:
        r['unit_price_rub'] = str(max(1000, round(int(r['unit_price_rub']) * rng.uniform(0.7, 1.4) / 1000) * 1000))
        r['stock_quantity'] = str(int(rng.integers(0, 7)))

    for r in data['suppliers']:
        r['delivery_cost_rub'] = str(round(int(r['delivery_cost_rub']) * rng.uniform(0.6, 1.5) / 1000) * 1000)
        r['delivery_days'] = str(max(1, int(r['delivery_days']) + int(rng.integers(-2, 3))))

    deadline = int(rng.integers(12, 19))
    budget = int(rng.integers(2000, 4201)) * 1000
    models = {r['equipment_id']: r for r in data['equipment']}

    for r in data['station']:
        r['deadline_days'] = str(deadline)
        r['recommended_budget_rub'] = str(budget)

        if r['installed_equipment_id'] != 'NONE' and rng.random() < 0.2:
            capacity = float(models[r['installed_equipment_id']]['capacity'])
            r['available_capacity'] = str(capacity if rng.random() > 0.5 else 0)

    return data
