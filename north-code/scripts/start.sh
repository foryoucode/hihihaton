#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

cd "$NORTH_ROOT"
mkdir -p runtime
if [[ ! -x backend/server || ! -f client/dist/index.html ]]; then
    echo 'Сначала выполните bash scripts/install.sh'
    exit 1
fi

solver_pid=''
backend_pid=''
cleanup() {
    if [[ -n "$backend_pid" ]]; then kill "$backend_pid" 2>/dev/null || true; fi
    if [[ -n "$solver_pid" ]]; then kill "$solver_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

export SOLVER_PORT="${SOLVER_PORT:-8091}"
export SOLVER_URL="${SOLVER_URL:-http://127.0.0.1:$SOLVER_PORT}"
export STATIC_DIR="$NORTH_ROOT/client/dist"
export GIN_MODE=release

if [[ "${START_ORTOOLS:-1}" != 0 ]]; then
    "$NORTH_PYTHON" -u solver/server.py > runtime/solver.log 2>&1 &
    solver_pid=$!
    "$NORTH_PYTHON" - "$SOLVER_PORT" <<'PY'
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
            raise SystemExit('Сервис OR-Tools не запустился. См. runtime/solver.log')
        time.sleep(0.25)
PY
fi

backend/server > runtime/backend.log 2>&1 &
backend_pid=$!
echo 'Северный код запускается. Адрес по умолчанию: http://127.0.0.1:8080'
echo 'Go подключается к PostgreSQL и применяет миграции Goose автоматически. Журналы находятся в runtime. Остановка: Ctrl+C.'

if [[ -n "$solver_pid" ]]; then
    wait -n "$solver_pid" "$backend_pid"
else
    wait "$backend_pid"
fi
