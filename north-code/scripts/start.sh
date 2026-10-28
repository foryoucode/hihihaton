#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

cd "$NORTH_ROOT"
mkdir -p runtime
if [[ ! -f engine/models/ranker.npz ]]; then
    echo 'Модель отсутствует. Обучите её: bash scripts/train.sh'
    exit 1
fi
if [[ ! -x backend/server || ! -f client/dist/index.html ]]; then
    echo 'Сначала выполните bash scripts/install.sh'
    exit 1
fi
engine_pid=''
backend_pid=''
cleanup() {
    if [[ -n "$backend_pid" ]]; then kill "$backend_pid" 2>/dev/null || true; fi
    if [[ -n "$engine_pid" ]]; then kill "$engine_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

ENGINE_PORT="${ENGINE_PORT:-8091}"
export ENGINE_PORT
export ENGINE_URL="http://127.0.0.1:$ENGINE_PORT"
export STATIC_DIR="$NORTH_ROOT/client/dist"
export GIN_MODE=release
"$NORTH_PYTHON" -u engine/server.py > runtime/engine.log 2>&1 &
engine_pid=$!
"$NORTH_PYTHON" - "$ENGINE_PORT" <<'PY'
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
            raise SystemExit('Расчётный сервис не запустился. См. runtime/engine.log')
        time.sleep(0.25)
PY
backend/server > runtime/backend.log 2>&1 &
backend_pid=$!
echo 'Проект запускается. Адрес по умолчанию: http://127.0.0.1:8080'
echo 'Журналы: runtime/engine.log и runtime/backend.log. Остановка: Ctrl+C.'
wait -n "$engine_pid" "$backend_pid"
