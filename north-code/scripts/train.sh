#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
cd "$NORTH_ROOT"
"$NORTH_PYTHON" engine/train.py "$@"
