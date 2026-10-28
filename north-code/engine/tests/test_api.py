import json
import sys
import tempfile
import threading
import unittest
from http.server import ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import ProxyHandler, build_opener

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import server


class ApiTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.old_path = server.DB_PATH
        server.DB_PATH = Path(cls.temp.name) / 'test.db'
        cls.http = ThreadingHTTPServer(('127.0.0.1', 0), server.Handler)
        cls.thread = threading.Thread(target=cls.http.serve_forever, daemon=True)
        cls.thread.start()
        cls.opener = build_opener(ProxyHandler({}))

    @classmethod
    def tearDownClass(cls):
        cls.http.shutdown()
        cls.http.server_close()
        cls.thread.join()
        server.DB_PATH = cls.old_path
        cls.temp.cleanup()

    def request(self, method, path, body=None):
        from urllib.request import Request
        raw = None if body is None else json.dumps(body).encode()
        req = Request('http://127.0.0.1:' + str(self.http.server_port) + path, raw,
                      {'Content-Type': 'application/json'}, method=method)
        try:
            with self.opener.open(req, timeout=60) as res:
                return res.status, json.load(res)
        except HTTPError as error:
            return error.code, json.load(error)

    def test_plan_crud_and_database_persistence(self):
        status, created = self.request('POST', '/plans', {'name': 'Проверка', 'method': 'exact'})
        self.assertEqual(status, 201)
        key = created['id']
        self.assertEqual(created['result']['cost'], 2482000)
        status, row = self.request('GET', '/plans/' + str(key))
        self.assertEqual(status, 200)
        self.assertEqual(row['name'], 'Проверка')
        with server.connect() as db:
            self.assertEqual(db.execute('SELECT name FROM plans WHERE id=?', (key,)).fetchone()['name'], 'Проверка')
        status, row = self.request('PATCH', '/plans/' + str(key), {'name': 'Новый сценарий'})
        self.assertEqual(status, 200)
        self.assertEqual(row['name'], 'Новый сценарий')
        self.assertTrue(any(r['id'] == key for r in self.request('GET', '/plans')[1]['plans']))
        self.assertEqual(self.request('DELETE', '/plans/' + str(key))[0], 200)
        self.assertEqual(self.request('GET', '/plans/' + str(key))[0], 404)

    def test_bad_input_and_unavailable_suppliers(self):
        for body in [{'method': 'wrong'}, {'budget': -1}, {'deadline': 0}, {'budget': 2.5}, {'disabled_suppliers': ['unknown']}]:
            self.assertEqual(self.request('POST', '/calculate', body)[0], 400)
        status, result = self.request('POST', '/calculate', {'disabled_suppliers': ['S%02d' % i for i in range(1, 13)]})
        self.assertEqual(status, 200)
        self.assertEqual(result['status'], 'infeasible')

    def test_neural_recommendation(self):
        status, result = self.request('POST', '/calculate', {'method': 'neural'})
        self.assertEqual(status, 200)
        self.assertTrue(result['validated'])
        self.assertEqual(result['method'], 'neural')
        self.assertFalse(result['optimal'])


if __name__ == '__main__':
    unittest.main()
