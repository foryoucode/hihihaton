#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

cd "$NORTH_ROOT"
if [[ -z "${TEST_DATABASE_URL:-}${DATABASE_URL:-}${PGHOST:-}" ]]; then
    echo 'Для интеграционных тестов нужен PostgreSQL: задайте TEST_DATABASE_URL или PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE.'
    exit 1
fi
"$NORTH_PYTHON" -m unittest discover -s solver/tests -v
cd backend
go test ./...
go build -o server ./cmd
cd ../client
npm run build
