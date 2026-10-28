import itertools
import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

from neural import MODEL_PATH, Ranker, neural_subset


LOCK = threading.Lock()


def recommend(body):
    needs = body['needs']
    candidates = body['candidates']
    optional = body['optional']
    deadline = body['deadline']

    if not isinstance(needs, list) or not isinstance(candidates, dict):
        raise ValueError('Некорректные признаки для модели')

    if not isinstance(optional, list) or len(optional) > 10 or len(set(optional)) != len(optional):
        raise ValueError('Некорректный список объектов')

    if type(deadline) is not int or not 1 <= deadline <= 365:
        raise ValueError('Некорректный срок')

    model = Ranker.load()
    variants = []

    for mask in itertools.product([False, True], repeat=len(optional)):
        selected = [key for key, flag in zip(optional, mask) if flag]
        plan = neural_subset(needs, candidates, selected, deadline, model)

        if plan is not None:
            variants.append({'selected': selected, 'plan': plan})

    return variants


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
            return self.send_json(200, {'status': 'ok', 'model_ready': MODEL_PATH.exists()})

        self.send_json(404, {'message': 'Маршрут не найден'})

    def do_POST(self):
        if urlparse(self.path).path != '/recommend':
            return self.send_json(404, {'message': 'Маршрут не найден'})

        try:
            size = int(self.headers.get('Content-Length', '0'))
            if not 0 < size <= 8 * 1024 * 1024:
                raise ValueError('Некорректный размер запроса')

            body = json.loads(self.rfile.read(size))
            if not isinstance(body, dict):
                raise ValueError('Ожидается JSON-объект')

            if not MODEL_PATH.exists():
                return self.send_json(503, {'message': 'Модель не обучена'})

            with LOCK:
                variants = recommend(body)

            self.send_json(200, {'variants': variants})
        except (ValueError, KeyError, TypeError) as error:
            self.send_json(400, {'message': str(error)})
        except Exception as error:
            print(type(error).__name__, str(error), flush=True)
            self.send_json(500, {'message': 'Ошибка нейросети'})


if __name__ == '__main__':
    port = int(os.environ.get('NEURAL_PORT', '8091'))
    print('Сервис нейросети: порт', port, flush=True)
    ThreadingHTTPServer(('127.0.0.1', port), Handler).serve_forever()
