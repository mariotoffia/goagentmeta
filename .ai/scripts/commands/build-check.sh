#!/usr/bin/env bash
set -euo pipefail
echo "Running full build check..."
make check
echo "Build check passed"
