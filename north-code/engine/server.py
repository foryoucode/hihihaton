import json
import os
import sqlite3
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse

from neural import MODEL_PATH, Ranker
from planner import ROOT, find_equipment, load_data, solve


DB_PATH = Path(os.environ.get('NORTH_DB_PATH', ROOT / 'runtime' / 'plans.db'))
DB_PATH.parent.mkdir(parents=True, exist_ok=True)
LOCK = threading.Lock()


def connect():
    db = sqlite3.connect(DB_PATH)
    db.row_factory = sqlite3.Row
    db.execute('CREATE TABLE IF NOT EXISTS plans (id INTEGER PRIMARY KEY, name TEXT NOT NULL, result TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)')

    return db


def metadata():
    path = Path(__file__).parent / 'models' / 'metrics.json'

    return json.loads(path.read_text()) if path.exists() else None


def run_calculation(body):
    data = load_data()
    budget = body.get('budget', int(data['station'][0]['recommended_budget_rub']))
    deadline = body.get('deadline', int(data['station'][0]['deadline_days']))

    if type(budget) is not int or not 0 <= budget <= 100000000:
        raise ValueError('Бюджет должен быть целым числом от 0 до 100 млн ₽')

    if type(deadline) is not int or not 1 <= deadline <= 365:
        raise ValueError('Срок должен быть целым числом от 1 до 365 дней')

    disabled = body.get('disabled_suppliers', [])
    known = {r['supplier_id'] for r in data['suppliers']}

    if not isinstance(disabled, list) or any(not isinstance(key, str) or key not in known for key in disabled):
        raise ValueError('Неизвестный поставщик')

    data['offers'] = [r for r in data['offers'] if r['supplier_id'] not in disabled]
    method = body.get('method', 'exact')
    model = Ranker.load() if method == 'neural' and MODEL_PATH.exists() else None
    start = time.perf_counter()
    result = solve(data, budget, deadline, method, model)
    result['seconds'] = round(time.perf_counter() - start, 3)
    result['disabled_suppliers'] = disabled

    return result


class Handler(BaseHTTPRequestHandler):
    def send_json(self, status, body):
        raw = json.dumps(body, ensure_ascii=False, allow_nan=False).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def body(self):
        size = int(self.headers.get('Content-Length', '0'))

        if size > 1048576:
            raise ValueError('Слишком большой запрос')

        result = json.loads(self.rfile.read(size) or b'{}')

        if not isinstance(result, dict):
            raise ValueError('Ожидается JSON-объект')

        return result

    def do_GET(self):
        self.dispatch('GET')

    def do_POST(self):
        self.dispatch('POST')

    def do_PATCH(self):
        self.dispatch('PATCH')

    def do_DELETE(self):
        self.dispatch('DELETE')

    def dispatch(self, method):
        path = urlparse(self.path).path

        try:
            if method == 'GET' and path == '/health':
                with connect() as db:
                    db.execute('SELECT 1')

                return self.send_json(200, {'status': 'ok', 'model_ready': MODEL_PATH.exists()})

            if method == 'GET' and path == '/data':
                data = load_data()
                data['model'] = metadata()

                return self.send_json(200, data)

            if method == 'GET' and path == '/experiments':
                return self.send_json(200, {'report': metadata()})

            if method == 'POST' and path == '/search':
                body = self.body()
                capacity, temperature = float(body.get('capacity', 0)), float(body.get('temperature', -30))
                import math

                if not math.isfinite(capacity) or not math.isfinite(temperature) or capacity < 0:
                    raise ValueError('Некорректные характеристики')

                return self.send_json(200, {'equipment': find_equipment(load_data(catalog_only=True), body.get('equipment_type', 'Генератор'), capacity, temperature)})

            if method == 'POST' and path == '/calculate':
                body = self.body()

                with LOCK:
                    result = run_calculation(body)

                return self.send_json(200, result)

            if path == '/plans' and method == 'GET':
                with connect() as db:
                    rows = db.execute('SELECT * FROM plans ORDER BY id DESC LIMIT 100').fetchall()

                return self.send_json(200, {'plans': [self.plan_row(row) for row in rows]})

            if path == '/plans' and method == 'POST':
                body = self.body()
                name = self.name(body)

                with LOCK:
                    result = run_calculation(body)

                with connect() as db:
                    cursor = db.execute('INSERT INTO plans(name,result) VALUES (?,?)', (name, json.dumps(result, ensure_ascii=False)))
                    row = db.execute('SELECT * FROM plans WHERE id=?', (cursor.lastrowid,)).fetchone()

                return self.send_json(201, self.plan_row(row))

            if path.startswith('/plans/'):
                plan_id = int(path.rsplit('/', 1)[1])
                body = self.body() if method == 'PATCH' else {}

                with connect() as db:
                    row = db.execute('SELECT * FROM plans WHERE id=?', (plan_id,)).fetchone()

                    if row is None:
                        return self.send_json(404, {'message': 'План не найден'})

                    if method == 'GET':
                        return self.send_json(200, self.plan_row(row))

                    if method == 'PATCH':
                        db.execute('UPDATE plans SET name=? WHERE id=?', (self.name(body), plan_id))
                        row = db.execute('SELECT * FROM plans WHERE id=?', (plan_id,)).fetchone()
                    elif method == 'DELETE':
                        db.execute('DELETE FROM plans WHERE id=?', (plan_id,))
                    else:
                        return self.send_json(405, {'message': 'Метод не поддерживается'})

                return self.send_json(200, {'status': 'deleted'} if method == 'DELETE' else self.plan_row(row))

            self.send_json(404, {'message': 'Маршрут не найден'})
        except (ValueError, KeyError, TypeError) as error:
            self.send_json(400, {'message': str(error)})
        except Exception as error:
            print(type(error).__name__, str(error), flush=True)
            self.send_json(500, {'message': 'Ошибка расчёта. Проверьте журнал сервиса.'})

    def name(self, body):
        name = body.get('name', '')

        if not isinstance(name, str) or not name.strip() or len(name.strip()) > 100:
            raise ValueError('Название должно содержать от 1 до 100 символов')

        return name.strip()

    def plan_row(self, row):
        return {'id': row['id'], 'name': row['name'], 'created_at': row['created_at'], 'result': json.loads(row['result'])}


if __name__ == '__main__':
    load_data()
    port = int(os.environ.get('ENGINE_PORT', '8091'))
    print('Расчётный сервис: порт', port, flush=True)
    ThreadingHTTPServer(('127.0.0.1', port), Handler).serve_forever()
