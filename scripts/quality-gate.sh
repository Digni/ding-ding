#!/usr/bin/env bash

set -euo pipefail

step() {
  local name="$1"
  shift
  echo "==> ${name}"
  "$@"
}

echo "Running ding-ding quality gate..."

step "go test ./..." go test ./...
step "go vet ./..." go vet ./...

echo "==> gofmt -l ."
gofmt_output="$(gofmt -l .)"
if [[ -n "${gofmt_output}" ]]; then
  echo "Formatting check failed. Run gofmt on:" >&2
  printf '%s\n' "${gofmt_output}" >&2
  exit 1
fi

echo "==> internal no-exit guardrail"
if rg --line-number --glob '!**/*_test.go' 'log\.Fatal|os\.Exit|panic\(' internal >/tmp/ding-ding-quality-gate-rg.out 2>/dev/null; then
  echo "Forbidden process-exit pattern found in internal/ (log.Fatal, os.Exit, panic):" >&2
  cat /tmp/ding-ding-quality-gate-rg.out >&2
  exit 1
fi

echo "QUALITY GATE PASS"
