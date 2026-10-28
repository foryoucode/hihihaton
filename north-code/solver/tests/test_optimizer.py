import sys
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from optimizer import solve


def offer(key, supplier, price, shipping, stock=1):
    return {'offer_id': key, 'supplier_id': supplier, 'equipment_id': key, 'unit_price_rub': price,
            'stock_quantity': stock, 'ready_days': 10,
            'model': {'model_name': key}, 'supplier': {'supplier_name': supplier, 'delivery_cost_rub': shipping}}


def need(key, optional='no'):
    return {'key': key, 'object_id': key, 'equipment_type': 'Генератор', 'optional': optional}


class SolverTests(unittest.TestCase):
    def test_shared_shipping_matches_example(self):
        body = {'needs': [need('G'), need('H')], 'optional': [], 'candidates': {
            'G': [offer('GA', 'A', 300000, 90000), offer('GB', 'B', 320000, 60000)],
            'H': [offer('HB', 'B', 180000, 60000), offer('HC', 'C', 150000, 80000)]}}
        result = solve(body)
        self.assertTrue(result['complete'])
        plan = result['variants'][0]['plan']
        self.assertEqual({r['supplier_id'] for r in plan}, {'B'})
        self.assertEqual(sum(r['unit_price_rub'] for r in plan) + 60000, 560000)

    def test_stock_is_shared(self):
        item = offer('ONE', 'A', 100, 20)
        result = solve({'needs': [need('B1'), need('B2')], 'optional': [], 'candidates': {'B1': [item], 'B2': [item]}})
        self.assertEqual(result['variants'], [])
        self.assertEqual(result['infeasible_counts'], [0])

    def test_optional_object_requires_all_needs(self):
        first = need('M', 'yes')
        second = {**first, 'key': 'M:heat'}
        result = solve({'needs': [first, second], 'optional': ['M'], 'candidates': {'M': [offer('G', 'A', 100, 20)], 'M:heat': []}})
        self.assertEqual(result['infeasible_counts'], [1])
        self.assertEqual(result['variants'], [{'selected': [], 'plan': []}])

    def test_timeout_never_claims_optimality(self):
        with patch('optimizer.time.monotonic', side_effect=[0, 100]):
            with self.assertRaises(TimeoutError):
                solve({'needs': [], 'optional': [], 'candidates': {}})


if __name__ == '__main__':
    unittest.main()
