#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

cd "$NORTH_ROOT"
"$NORTH_PYTHON" -m unittest discover -s engine/tests -v
cd backend
go test ./...
go build -o server ./cmd
cd ../client
npm run build
