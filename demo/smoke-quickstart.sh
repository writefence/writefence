#!/usr/bin/env bash
set -euo pipefail

StorePort="${WRITEFENCE_SMOKE_STORE_PORT:-19621}"
ProxyPort="${WRITEFENCE_SMOKE_PROXY_PORT:-19622}"
Host="${WRITEFENCE_SMOKE_HOST:-127.0.0.1}"
Keep="${WRITEFENCE_SMOKE_KEEP:-0}"
Root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WorkDir="${WRITEFENCE_SMOKE_DIR:-}"

if [[ -z "$WorkDir" ]]; then
  WorkDir="$(mktemp -d "${TMPDIR:-/tmp}/writefence-smoke-XXXXXX")"
fi

DataDir="$WorkDir/data"
StoreDir="$WorkDir/mock-store"
LogDir="$WorkDir/logs"
MockPID=""
ProxyPID=""

cleanup() {
  local code=$?
  if [[ -n "$ProxyPID" ]] && kill -0 "$ProxyPID" >/dev/null 2>&1; then
    kill "$ProxyPID" >/dev/null 2>&1 || true
    wait "$ProxyPID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$MockPID" ]] && kill -0 "$MockPID" >/dev/null 2>&1; then
    kill "$MockPID" >/dev/null 2>&1 || true
    wait "$MockPID" >/dev/null 2>&1 || true
  fi
  if [[ "$Keep" != "1" && -z "${WRITEFENCE_SMOKE_DIR:-}" ]]; then
    rm -rf "$WorkDir"
  elif [[ "$code" != "0" || "$Keep" == "1" ]]; then
    printf 'Smoke artifacts kept at %s\n' "$WorkDir"
  fi
}
trap cleanup EXIT

section() {
  printf '\n== %s ==\n' "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

port_open() {
  (:</dev/tcp/"$Host"/"$1") >/dev/null 2>&1
}

require_free_port() {
  local port="$1"
  if port_open "$port"; then
    printf 'Port %s:%s is already in use. Set WRITEFENCE_SMOKE_%s_PORT to override.\n' "$Host" "$port" "$2" >&2
    exit 1
  fi
}

wait_http() {
  local url="$1"
  local name="$2"
  local i
  for i in $(seq 1 80); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  printf '%s did not become ready at %s\n' "$name" "$url" >&2
  printf '\n-- mock-store.log --\n' >&2
  sed -n '1,160p' "$LogDir/mock-store.log" >&2 || true
  printf '\n-- writefence.log --\n' >&2
  sed -n '1,160p' "$LogDir/writefence.log" >&2 || true
  exit 1
}

require_cmd go
require_cmd curl
require_cmd sed

mkdir -p "$Root/bin" "$DataDir" "$StoreDir" "$LogDir"

section "Preflight"
printf 'Repo      : %s\n' "$Root"
printf 'Work dir  : %s\n' "$WorkDir"
printf 'Mock store: http://%s:%s\n' "$Host" "$StorePort"
printf 'Proxy     : http://%s:%s\n' "$Host" "$ProxyPort"
go version
require_free_port "$StorePort" "STORE"
require_free_port "$ProxyPort" "PROXY"

section "Build"
(cd "$Root" && go build -o bin/writefence ./cmd/writefence)
(cd "$Root" && go build -o bin/writefence-cli ./cmd/writefence-cli)

section "Start services"
(cd "$Root" && WRITEFENCE_DEMO_STORE_DIR="$StoreDir" go run ./demo/mock-memory-store.go -addr "$Host:$StorePort" >"$LogDir/mock-store.log" 2>&1) &
MockPID=$!
wait_http "http://$Host:$StorePort/healthz" "mock memory store"

(cd "$Root" && WRITEFENCE_DATA_DIR="$DataDir" ./bin/writefence --addr "$Host:$ProxyPort" --upstream "http://$Host:$StorePort" >"$LogDir/writefence.log" 2>&1) &
ProxyPID=$!
wait_http "http://$Host:$ProxyPort/_writefence" "WriteFence operator UI"

section "Admission checks"
BlockedCode="$(curl -sS -o "$WorkDir/blocked.json" -w '%{http_code}' \
  -X POST "http://$Host:$ProxyPort/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"status without prefix","description":"smoke blocked write"}')"
if [[ "$BlockedCode" != "422" ]]; then
  printf 'Expected blocked write HTTP 422, got %s\n' "$BlockedCode" >&2
  cat "$WorkDir/blocked.json" >&2 || true
  exit 1
fi
grep -q '"decision":"blocked"' "$WorkDir/blocked.json"

AllowedCode="$(curl -sS -o "$WorkDir/allowed.json" -w '%{http_code}' \
  -X POST "http://$Host:$ProxyPort/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"[STATUS] smoke corrected write","description":"smoke allowed write"}')"
if [[ "$AllowedCode" != "200" ]]; then
  printf 'Expected allowed write HTTP 200, got %s\n' "$AllowedCode" >&2
  cat "$WorkDir/allowed.json" >&2 || true
  exit 1
fi

section "Canonical demo"
(cd "$Root" && \
  WRITEFENCE_DATA_DIR="$DataDir" \
  WRITEFENCE_URL="http://$Host:$ProxyPort" \
  WRITEFENCE_WAL="$DataDir/writefence-wal.jsonl" \
  ./demo/canonical-demo.sh) | tee "$WorkDir/canonical-demo.log"
grep -q 'Replay Report' "$WorkDir/canonical-demo.log"

section "Result"
printf 'WriteFence quickstart smoke passed.\n'
printf 'Verified UI endpoint: http://%s:%s/_writefence\n' "$Host" "$ProxyPort"
