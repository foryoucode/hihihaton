import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

import ortools
from optimizer import solve


LOCK = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    def send_json(self, status, body):
        raw = json.dumps(body, ensure_ascii=False, allow_nan=False).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        if urlparse(self.path).path == '/health':
            return self.send_json(200, {'status': 'ok', 'solver': 'OR-Tools CP-SAT', 'version': ortools.__version__})
        self.send_json(404, {'message': 'Маршрут не найден'})

    def do_POST(self):
        if urlparse(self.path).path != '/solve':
            return self.send_json(404, {'message': 'Маршрут не найден'})
        try:
            size = int(self.headers.get('Content-Length', '0'))
            if not 0 < size <= 8 * 1024 * 1024:
                raise ValueError('Некорректный размер запроса')
            body = json.loads(self.rfile.read(size))
            if not isinstance(body, dict):
                raise ValueError('Ожидается JSON-объект')
            if not LOCK.acquire(blocking=False):
                return self.send_json(503, {'message': 'Решатель занят. Повторите запрос.'})
            try:
                result = solve(body)
            finally:
                LOCK.release()
            self.send_json(200, result)
        except TimeoutError as error:
            self.send_json(504, {'message': str(error)})
        except (ValueError, KeyError, TypeError) as error:
            self.send_json(400, {'message': str(error)})
        except Exception as error:
            print(type(error).__name__, str(error), flush=True)
            self.send_json(500, {'message': 'Ошибка OR-Tools'})


if __name__ == '__main__':
    host = os.environ.get('SOLVER_HOST', '127.0.0.1')
    port = int(os.environ.get('SOLVER_PORT', '8091'))
    print('OR-Tools CP-SAT:', host, port, flush=True)
    ThreadingHTTPServer((host, port), Handler).serve_forever()
