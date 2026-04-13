#!/usr/bin/env bash
set -euo pipefail

echo "Running pre-commit checks..."

# Only run Go checks if Go files changed
if echo "${CHANGED_FILES:-}" | grep -q '\.go$'; then
    echo "Go files changed — running make check"
    make check
else
    echo "No Go files changed — skipping build checks"
fi

echo "Pre-commit checks passed"
