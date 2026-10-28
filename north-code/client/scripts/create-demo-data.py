"""Generate fictional UI fixtures. Reads no uploads, CSV files, database or API."""
import itertools
import json
from pathlib import Path

import numpy as np
from scipy.optimize import Bounds, LinearConstraint, milp
from scipy.sparse import lil_matrix


catalog = []
types = {
    'G': ('Генератор', 'кВт', [4, 6, 9, 12, 16], -35, 95000, 19500),
    'H': ('Топливный отопитель', 'кВт', [8, 12, 16], -35, 40000, 11000),
    'P': ('Водяной насос', 'л/мин', [25, 35, 50], 0, 28000, 800),
    'W': ('Очистка воды', 'л/ч', [80, 110, 160], 0, 42000, 900),
    'V': ('Вентиляционная установка', 'м3/ч', [220, 280, 400], -25, 36000, 250),
}
prices = {}
for prefix, (kind, unit, capacities, temp, fixed, rate) in types.items():
    for capacity in capacities:
        key = f'DEMO-{prefix}{capacity}'
        catalog.append({'equipment_id': key, 'model_name': f'Учебная модель {prefix}-{capacity}',
                        'equipment_type': kind, 'capacity': str(capacity), 'capacity_unit': unit,
                        'min_operating_temperature_c': str(temp), 'installation_days': str(1 + (capacity % 3 == 0))})
        prices[key] = fixed + rate * capacity
catalog.append({'equipment_id': 'DEMO-H16-WARM', 'model_name': 'Учебный отопитель для тёплого помещения',
                'equipment_type': 'Топливный отопитель', 'capacity': '16', 'capacity_unit': 'кВт',
                'min_operating_temperature_c': '-15', 'installation_days': '1'})
prices['DEMO-H16-WARM'] = 150000
suppliers = [
    {'supplier_id': 'D01', 'supplier_name': 'Демо: Северный маршрут', 'delivery_cost_rub': '95000', 'delivery_days': '5'},
    {'supplier_id': 'D02', 'supplier_name': 'Демо: Полярный склад', 'delivery_cost_rub': '55000', 'delivery_days': '8'},
    {'supplier_id': 'D03', 'supplier_name': 'Демо: Экспресс', 'delivery_cost_rub': '135000', 'delivery_days': '3'},
    {'supplier_id': 'D04', 'supplier_name': 'Демо: Сборная поставка', 'delivery_cost_rub': '30000', 'delivery_days': '12'},
]
offers = []
for i, model in enumerate(catalog):
    for j, supplier in enumerate(suppliers):
        price = round((prices[model['equipment_id']] * [1.08, .96, 1.18, .9][j] + ((i + j) % 3) * 2000) / 1000) * 1000
        offers.append({'offer_id': f'DEMO-O{len(offers) + 1:03}', 'supplier_id': supplier['supplier_id'],
                       'equipment_id': model['equipment_id'], 'equipment_type': model['equipment_type'],
                       'unit_price_rub': str(price), 'stock_quantity': str([3, 2, 5, 1][j])})
station = []
objects = [
    ('BASE01', 'Жилой модуль', False, [('G', 9, -30, 'DEMO-G12', 4), ('H', 14, -25, 'DEMO-H16', 16)]),
    ('BASE02', 'Связь и навигация', False, [('G', 4, -30, 'DEMO-G6', 0), ('V', 220, -15, 'DEMO-V280', 280)]),
    ('BASE03', 'Водяной узел', False, [('P', 28, 8, 'DEMO-P35', 10), ('W', 90, 8, 'DEMO-W110', 110)]),
    ('MOBILE01', 'Пункт наблюдений', True, [('G', 4, -30, 'NONE', 0), ('H', 7, -25, 'DEMO-H8', 8)]),
    ('MOBILE02', 'Пробоотбор', True, [('G', 6, -30, 'DEMO-G6', 6), ('P', 22, 8, 'DEMO-P25', 0)]),
    ('MOBILE03', 'Полевой пост', True, [('G', 6, -30, 'DEMO-G6', 3), ('V', 180, -15, 'DEMO-V220', 0)]),
    ('MOBILE04', 'Исследовательский модуль', True, [('G', 11, -30, 'DEMO-G12', 0), ('H', 10, -30, 'DEMO-H16-WARM', 16)]),
    ('MOBILE05', 'Пункт мониторинга', True, [('W', 65, 8, 'DEMO-W80', 0), ('H', 7, -25, 'DEMO-H8', 8)]),
]
for key, name, optional, requirements in objects:
    for prefix, capacity, temp, installed, available in requirements:
        station.append({'station_id': 'DEMO-STATION', 'station_name': 'Учебная станция Полюс',
                        'recommended_budget_rub': '1900000', 'deadline_days': '14', 'object_id': key,
                        'object_name': name, 'optional': 'yes' if optional else 'no',
                        'equipment_type': types[prefix][0], 'required_capacity': str(capacity),
                        'capacity_unit': types[prefix][1], 'operating_temperature_c': str(temp),
                        'installed_equipment_id': installed, 'available_capacity': str(available)})
models = {r['equipment_id']: r for r in catalog}
supply = {r['supplier_id']: r for r in suppliers}
diagnosis = []
for row in station:
    old = models.get(row['installed_equipment_id'])
    reasons = []
    if old is None:
        reasons.append('Оборудование отсутствует')
    else:
        if float(row['available_capacity']) < float(row['required_capacity']):
            reasons.append('Недостаточная доступная мощность')
        if float(old['min_operating_temperature_c']) > float(row['operating_temperature_c']):
            reasons.append('Не подходит температура эксплуатации')
    diagnosis.append({**row, 'key': row['object_id'] + ':' + row['equipment_type'], 'reasons': reasons, 'needs_replacement': bool(reasons)})


def plans_for(deadline, disabled):
    variants = []
    optional = [key for key, _, flag, _ in objects if flag]
    for mask in itertools.product([False, True], repeat=len(optional)):
        selected = [key for key, flag in zip(optional, mask) if flag]
        required = [r for r in diagnosis if r['needs_replacement'] and (r['optional'] == 'no' or r['object_id'] in selected)]
        edges = []
        for i, req in enumerate(required):
            for j, offer in enumerate(offers):
                model = models[offer['equipment_id']]
                supplier = supply[offer['supplier_id']]
                if offer['supplier_id'] in disabled or model['equipment_type'] != req['equipment_type']:
                    continue
                ready = int(supplier['delivery_days']) + int(model['installation_days'])
                if float(model['capacity']) >= float(req['required_capacity']) and float(model['min_operating_temperature_c']) <= float(req['operating_temperature_c']) and ready <= deadline:
                    edges.append((i, j, ready))
        n = len(edges)
        a = lil_matrix((len(required) + len(offers) + len(suppliers), n + len(suppliers)))
        lo = np.full(a.shape[0], -np.inf)
        hi = np.zeros(a.shape[0])
        cost = np.zeros(n + len(suppliers))
        for i in range(len(required)):
            lo[i] = hi[i] = 1
        for j, offer in enumerate(offers):
            hi[len(required) + j] = int(offer['stock_quantity'])
        for k, supplier in enumerate(suppliers):
            cost[n + k] = int(supplier['delivery_cost_rub'])
            a[len(required) + len(offers) + k, n + k] = -len(required)
        for z, (i, j, _) in enumerate(edges):
            offer = offers[j]
            k = next(k for k, s in enumerate(suppliers) if s['supplier_id'] == offer['supplier_id'])
            cost[z] = int(offer['unit_price_rub'])
            a[i, z] = 1
            a[len(required) + j, z] = 1
            a[len(required) + len(offers) + k, z] = 1
        result = milp(cost, integrality=np.ones(len(cost)), bounds=Bounds(0, 1),
                      constraints=LinearConstraint(a.tocsr(), lo, hi), options={'mip_rel_gap': 0})
        if result.status == 2:
            continue
        if result.status != 0:
            raise RuntimeError(result.message)
        plan = []
        for z, (i, j, ready) in enumerate(edges):
            if result.x[z] < .5:
                continue
            req, offer = required[i], offers[j]
            model, supplier = models[offer['equipment_id']], supply[offer['supplier_id']]
            plan.append({'key': req['key'], 'object_id': req['object_id'], 'equipment_type': req['equipment_type'],
                         'offer_id': offer['offer_id'], 'equipment_id': offer['equipment_id'], 'model_name': model['model_name'],
                         'supplier_id': offer['supplier_id'], 'supplier_name': supplier['supplier_name'],
                         'unit_price_rub': int(offer['unit_price_rub']), 'ready_days': ready})
        equipment = sum(r['unit_price_rub'] for r in plan)
        delivery = sum(int(supply[key]['delivery_cost_rub']) for key in {r['supplier_id'] for r in plan})
        restored = []
        for key, name, flag, _ in objects:
            active = not flag or key in selected
            day = max([r['ready_days'] for r in plan if r['object_id'] == key], default=0)
            restored.append({'object_id': key, 'name': name, 'optional': flag, 'restored': active, 'ready_days': day if active else None})
        variants.append({'status': 'ok', 'method': 'exact', 'optimal': False, 'validated': False, 'demo': True,
                         'selected': selected, 'plan': plan, 'cost': equipment + delivery, 'equipment_cost': equipment,
                         'delivery_cost': delivery, 'optional_count': len(selected), 'objects': restored,
                         'ready_days': max((r['ready_days'] for r in plan), default=0), 'diagnosis': diagnosis,
                         'budget': 1900000, 'deadline': deadline, 'seconds': 0, 'dataset_id': 1, 'dataset_name': 'Полностью вымышленные демоданные'})
    best = {}
    for row in variants:
        count = row['optional_count']
        if count not in best or row['cost'] < best[count]['cost']:
            best[count] = row
    frontier = [{'optional_count': k, 'cost': v['cost'], 'selected': v['selected']} for k, v in sorted(best.items())]
    for row in best.values():
        row['frontier'] = frontier
    return list(best.values())


scenarios = []
for deadline, disabled in [(10, []), (12, []), (14, []), (18, []), (14, ['D01']), (14, ['D02']), (14, ['D04'])]:
    scenarios.append({'deadline': deadline, 'disabled': disabled, 'plans': plans_for(deadline, disabled)})
assert all(s['plans'] for s in scenarios)
data = {'equipment': catalog, 'suppliers': suppliers, 'offers': offers, 'station': station,
        'dataset': {'id': 1, 'name': 'Полностью вымышленные демоданные'}}
output = {'provenance': 'Entirely fictional data generated from constants in create-demo-data.py. No uploaded files, CSV, database or API were read.',
          'data': data, 'scenarios': scenarios}
path = Path(__file__).resolve().parents[1] / 'src/demo/data.json'
path.write_text(json.dumps(output, ensure_ascii=False, separators=(',', ':')))
print('Fictional UI fixtures:', len(catalog), 'models,', len(offers), 'offers;', len(scenarios), 'scenarios')
