#!/usr/bin/env bash
# scripts/test.sh
# Runs the full test suite on Linux/macOS.
# Usage: ./scripts/test.sh [--race] [--cover]

set -euo pipefail

RACE=""
COVER=""

for arg in "$@"; do
    case $arg in
        --race)  RACE="-race" ;;
        --cover) COVER="-coverprofile=coverage.out" ;;
    esac
done

echo "Running tests..."
# shellcheck disable=SC2086
go test ./... $RACE $COVER -timeout=120s -v

if [[ -n "$COVER" ]]; then
    go tool cover -html=coverage.out -o coverage.html
    echo "Coverage report: coverage.html"
fi

echo "All tests passed."