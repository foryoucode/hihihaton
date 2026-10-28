import copy
import sys
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from neural import Ranker
from planner import diagnose, exact_subset, find_equipment, load_data, make_scenario, prepare, solve, validate_data, validate_plan


class PlannerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.data = load_data()
        cls.exact = solve(cls.data, 2600000, 14)

    def test_reference_plan_and_frontier(self):
        result = self.exact
        self.assertEqual(result['cost'], 2482000)
        self.assertEqual(result['selected'], ['M01', 'M02', 'M05'])
        self.assertEqual(result['equipment_cost'], 2212000)
        self.assertEqual(result['delivery_cost'], 270000)
        self.assertEqual(result['ready_days'], 14)
        self.assertEqual([r['cost'] for r in result['frontier']], [1824000, 2027000, 2231000, 2482000, 2782000, 3633000])
        self.assertEqual(validate_plan(self.data, result['plan'], result['selected'], 14), [])

    def test_diagnosis_temperature_and_missing_equipment(self):
        rows = diagnose(self.data)
        self.assertEqual(sum(r['needs_replacement'] for r in rows), 14)
        heater = next(r for r in rows if r['object_id'] == 'B02' and r['equipment_type'] == 'Топливный отопитель')
        self.assertEqual(heater['reasons'], ['Не подходит температура эксплуатации'])
        missing = next(r for r in rows if r['object_id'] == 'M01' and r['equipment_type'] == 'Генератор')
        self.assertIn('Оборудование отсутствует', missing['reasons'])

    def test_budget_priorities(self):
        result = solve(self.data, 2782000, 14)
        self.assertEqual(result['optional_count'], 4)
        result = solve(self.data, 1000000, 14)
        self.assertEqual(result['cost'], 1824000)
        self.assertEqual(result['overrun'], 824000)
        self.assertTrue(all(r['restored'] for r in result['objects'] if not r['optional']))

    def test_impossible_deadline(self):
        self.assertEqual(solve(self.data, 2600000, 1)['status'], 'infeasible')

    def test_stock_is_shared(self):
        needs, candidates = prepare(self.data, 14)
        req = next(r for r in needs if r['object_id'] == 'M01')
        one = copy.deepcopy(candidates[req['key']][0])
        one['stock_quantity'] = 1
        other = {**req, 'key': 'OTHER:Генератор', 'object_id': 'OTHER'}
        self.assertIsNone(exact_subset([req, other], {req['key']: [one], other['key']: [one]}, ['M01', 'OTHER']))

    def test_checker_rejects_forged_plan(self):
        plan = copy.deepcopy(self.exact['plan'])
        plan[0]['unit_price_rub'] = 1
        self.assertTrue(validate_plan(self.data, plan, self.exact['selected'], 14))
        plan = copy.deepcopy(self.exact['plan'])
        plan.append(plan[0])
        self.assertTrue(validate_plan(self.data, plan, self.exact['selected'], 14))

    def test_model_uses_no_exact_solver(self):
        with patch('planner.milp', side_effect=AssertionError('MILP вызван в нейросетевом режиме')):
            result = solve(self.data, 2600000, 14, 'neural', Ranker.load())
        self.assertEqual(result['status'], 'ok')
        self.assertEqual(result['optional_count'], 3)
        self.assertFalse(result['optimal'])
        self.assertGreaterEqual(result['cost'], self.exact['cost'])
        self.assertEqual(validate_plan(self.data, result['plan'], result['selected'], 14), [])

    def test_missing_model_never_silently_falls_back(self):
        with self.assertRaises(ValueError):
            solve(self.data, 2600000, 14, 'neural')

    def test_generator_is_repeatable_and_consistent(self):
        first = make_scenario(self.data, 77)
        self.assertEqual(first, make_scenario(self.data, 77))
        self.assertNotEqual(first, make_scenario(self.data, 78))
        validate_data(first)
        self.assertEqual(self.data['station'][0]['deadline_days'], '14')

    def test_search_temperature_direction(self):
        models = find_equipment(self.data, 'Генератор', 8, -30)
        self.assertTrue(models)
        self.assertTrue(all(float(r['capacity']) >= 8 and float(r['min_operating_temperature_c']) <= -30 for r in models))
        self.assertEqual(find_equipment(self.data, 'Генератор', 10000, -30), [])

    def test_catalog_works_without_station_or_suppliers(self):
        with patch('planner.ROOT', Path(__file__).resolve().parents[2]):
            data = load_data(catalog_only=True)
        self.assertEqual(set(data), {'equipment'})
        self.assertTrue(find_equipment(data, 'Генератор', 8, -30))


if __name__ == '__main__':
    unittest.main()
