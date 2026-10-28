#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

cd "$NORTH_ROOT"
mkdir -p runtime
if [[ ! -x backend/server || ! -f client/dist/index.html ]]; then
    echo 'Сначала выполните bash scripts/install.sh'
    exit 1
fi

neural_pid=''
backend_pid=''
cleanup() {
    if [[ -n "$backend_pid" ]]; then kill "$backend_pid" 2>/dev/null || true; fi
    if [[ -n "$neural_pid" ]]; then kill "$neural_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

export NEURAL_PORT="${NEURAL_PORT:-8091}"
export NEURAL_URL="${NEURAL_URL:-http://127.0.0.1:$NEURAL_PORT}"
export STATIC_DIR="$NORTH_ROOT/client/dist"
export GIN_MODE=release

if [[ "${START_NEURAL:-1}" != 0 ]]; then
    "$NORTH_PYTHON" -u engine/server.py > runtime/neural.log 2>&1 &
    neural_pid=$!
    "$NORTH_PYTHON" - "$NEURAL_PORT" <<'PY'
import json, sys, time
from urllib.request import ProxyHandler, build_opener
opener = build_opener(ProxyHandler({}))
for attempt in range(40):
    try:
        with opener.open('http://127.0.0.1:' + sys.argv[1] + '/health', timeout=1) as response:
            assert json.load(response)['status'] == 'ok'
        break
    except Exception:
        if attempt == 39:
            raise SystemExit('Сервис нейросети не запустился. См. runtime/neural.log')
        time.sleep(0.25)
PY
fi

backend/server > runtime/backend.log 2>&1 &
backend_pid=$!
echo 'Северный код запускается. Адрес по умолчанию: http://127.0.0.1:8080'
echo 'Go применяет миграции Goose автоматически. Журналы находятся в runtime. Остановка: Ctrl+C.'

if [[ -n "$neural_pid" ]]; then
    wait -n "$neural_pid" "$backend_pid"
else
    wait "$backend_pid"
fi
