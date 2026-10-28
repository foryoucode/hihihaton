#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

cd "$NORTH_ROOT"
go version
node --version
python3 -m venv .venv
.venv/bin/python -m pip install -r engine/requirements.txt
cd backend
go mod download
go build -o server ./cmd
cd ../client
npm ci --no-audit --no-fund
npm run build
echo 'Установка завершена. Запуск: bash scripts/start.sh'
