#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"

if ! command -v sqlc >/dev/null || [[ "$(sqlc version)" != 'v1.29.0' ]]; then
    echo 'Для генерации нужен sqlc v1.29.0. Сгенерированный код уже включён в репозиторий.'
    exit 1
fi

cd "$NORTH_ROOT/backend"
sqlc generate
