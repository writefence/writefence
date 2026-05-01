#!/usr/bin/env bash
set -euo pipefail

WriteFenceURL="${WRITEFENCE_URL:-http://127.0.0.1:9622}"
WriteFenceCLI="${WRITEFENCE_CLI:-./bin/writefence-cli}"
WALPath="${WRITEFENCE_WAL:-${HOME:-.}/.writefence/writefence-wal.jsonl}"
ReviewAction="${WRITEFENCE_REVIEW_ACTION:-approve}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

print_section() {
  printf '\n== %s ==\n' "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

extract_json_field() {
  local file="$1"
  local field="$2"
  sed -n "s/.*\"${field}\":[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$file" | head -n 1
}

require_cmd curl
require_cmd grep
require_cmd sed

if [[ ! -x "$WriteFenceCLI" ]]; then
  printf 'writefence-cli not found or not executable at %s\n' "$WriteFenceCLI" >&2
  exit 1
fi

print_section "Prerequisites"
printf 'Proxy URL: %s\n' "$WriteFenceURL"
printf 'CLI path : %s\n' "$WriteFenceCLI"
printf 'WAL path : %s\n' "$WALPath"
printf 'Review   : %s\n' "$ReviewAction"

print_section "Path 1: Block Before Persistence -> Suggested Fix -> Allow"
bad_code="$(curl -sS -o "$tmpdir/block.json" -w '%{http_code}' \
  -X POST "$WriteFenceURL/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"status without prefix"}')"
printf 'Bad write HTTP: %s\n' "$bad_code"
cat "$tmpdir/block.json"
printf '\n'
printf 'Expected: HTTP 422 means WriteFence rejected this before forwarding it to the memory store.\n'

good_code="$(curl -sS -o "$tmpdir/allow.json" -w '%{http_code}' \
  -X POST "$WriteFenceURL/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"[RUNBOOK] corrected write after ADC guidance","description":"canonical demo corrected write"}')"
printf 'Corrected write HTTP: %s\n' "$good_code"
cat "$tmpdir/allow.json"
printf '\n'

print_section "Path 2: Warn -> Forward With Headers"
warn_code="$(curl -sS -D "$tmpdir/warn.headers" -o "$tmpdir/warn.json" -w '%{http_code}' \
  -X POST "$WriteFenceURL/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"[RUNBOOK] mixed language note with яяя characters but otherwise English demo context","description":"canonical demo warning"}')"
printf 'Warned write HTTP: %s\n' "$warn_code"
grep -i '^X-WriteFence-' "$tmpdir/warn.headers" || true
cat "$tmpdir/warn.json"
printf '\n'
printf 'Expected: HTTP 200 with X-WriteFence-Decision: warned means the write was forwarded with an admission warning.\n'

print_section "Path 3: Quarantine -> Human Review"
first_quarantine_code="$(curl -sS -o "$tmpdir/quarantine-seed.json" -w '%{http_code}' \
  -X POST "$WriteFenceURL/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"[RUNBOOK] canonical replay seed for quarantine path","description":"canonical demo seed"}')"
printf 'Seed write HTTP: %s\n' "$first_quarantine_code"

second_quarantine_code="$(curl -sS -o "$tmpdir/quarantine.json" -w '%{http_code}' \
  -X POST "$WriteFenceURL/documents/text" \
  -H 'Content-Type: application/json' \
  --data '{"text":"[RUNBOOK] canonical replay seed for quarantine path with slight wording drift","description":"canonical demo duplicate candidate"}')"
printf 'Near-duplicate write HTTP: %s\n' "$second_quarantine_code"
cat "$tmpdir/quarantine.json"
printf '\n'

if [[ "$second_quarantine_code" != "202" ]]; then
  printf 'Quarantine path did not trigger. Semantic dedup may be disabled or current similarity is below the quarantine threshold.\n'
else
  trace_id="$(extract_json_field "$tmpdir/quarantine.json" trace_id)"
  printf 'Quarantine trace_id: %s\n' "$trace_id"
  "$WriteFenceCLI" quarantine list
  if [[ -n "$trace_id" ]]; then
    case "$ReviewAction" in
      approve)
        "$WriteFenceCLI" quarantine approve "$trace_id"
        ;;
      reject)
        "$WriteFenceCLI" quarantine reject "$trace_id"
        ;;
      *)
        printf 'Unknown WRITEFENCE_REVIEW_ACTION=%s (expected approve or reject)\n' "$ReviewAction" >&2
        exit 1
        ;;
    esac
  fi
fi

print_section "Path 4: Replay"
"$WriteFenceCLI" replay --wal "$WALPath"
