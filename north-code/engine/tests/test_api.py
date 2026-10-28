import json
import sys
import threading
import unittest
from http.server import ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import ProxyHandler, Request, build_opener
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import server
from planner import load_data, prepare, validate_plan


class NeuralApiTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.http = ThreadingHTTPServer(('127.0.0.1', 0), server.Handler)
        cls.thread = threading.Thread(target=cls.http.serve_forever, daemon=True)
        cls.thread.start()
        cls.opener = build_opener(ProxyHandler({}))

    @classmethod
    def tearDownClass(cls):
        cls.http.shutdown()
        cls.http.server_close()
        cls.thread.join()

    def request(self, method, path, body=None):
        raw = None if body is None else json.dumps(body).encode()
        req = Request('http://127.0.0.1:' + str(self.http.server_port) + path, raw,
                      {'Content-Type': 'application/json'}, method=method)

        try:
            with self.opener.open(req, timeout=60) as res:
                return res.status, json.load(res)
        except HTTPError as error:
            return error.code, json.load(error)

    def test_model_health(self):
        status, body = self.request('GET', '/health')
        self.assertEqual(status, 200)
        self.assertTrue(body['model_ready'])

    def test_only_neural_routes_are_exposed(self):
        for path in ['/plans', '/data', '/experiments']:
            self.assertEqual(self.request('GET', path)[0], 404)

        self.assertEqual(self.request('POST', '/calculate', {})[0], 404)
        self.assertEqual(self.request('POST', '/recommend', {})[0], 400)

    def test_inference_has_no_exact_solver(self):
        data = load_data()
        needs, candidates = prepare(data, 14)
        body = {'needs': needs, 'candidates': candidates, 'optional': ['M01', 'M02', 'M03', 'M04', 'M05'], 'deadline': 14}

        with patch('planner.milp', side_effect=AssertionError('Точный решатель вызван при inference')):
            status, result = self.request('POST', '/recommend', body)

        self.assertEqual(status, 200)
        self.assertTrue(result['variants'])

        for variant in result['variants']:
            self.assertEqual(validate_plan(data, variant['plan'], variant['selected'], 14), [])


if __name__ == '__main__':
    unittest.main()
