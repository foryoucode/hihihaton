#!/usr/bin/env bash

NORTH_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export NORTH_ROOT
export OPENBLAS_NUM_THREADS=1
export GOCACHE="${GOCACHE:-$NORTH_ROOT/runtime/go-cache}"
export GOMAXPROCS="${GOMAXPROCS:-2}"
if [[ -x /workspace/.tools/go/bin/go ]]; then
    export PATH="/workspace/.tools/go/bin:/workspace/.tools:$PATH"
fi
NORTH_PYTHON="${NORTH_PYTHON:-python3}"
if [[ -x "$NORTH_ROOT/.venv/bin/python" ]]; then
    NORTH_PYTHON="$NORTH_ROOT/.venv/bin/python"
fi
export NORTH_PYTHON
