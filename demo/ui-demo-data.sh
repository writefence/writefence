#!/usr/bin/env bash
set -euo pipefail

DataDir="${1:-${WRITEFENCE_DATA_DIR:-${HOME:-.}/.writefence}}"
Overwrite="${WRITEFENCE_DEMO_OVERWRITE:-0}"

WALPath="${DataDir}/writefence-wal.jsonl"
QuarantinePath="${DataDir}/writefence-quarantine.jsonl"
ViolationsPath="${DataDir}/writefence-violations.jsonl"
StatePath="${DataDir}/session-state.json"

if [[ -e "$WALPath" || -e "$QuarantinePath" || -e "$ViolationsPath" || -e "$StatePath" ]]; then
  if [[ "$Overwrite" != "1" ]]; then
    printf 'demo data target is not empty: %s\n' "$DataDir" >&2
    printf 'set WRITEFENCE_DEMO_OVERWRITE=1 to replace demo JSONL/state files.\n' >&2
    exit 1
  fi
fi

mkdir -p "$DataDir"

cat >"$WALPath" <<'JSONL'
{"ts":"2026-04-28T10:00:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] corrected write after ADC guidance","description":"canonical demo corrected write"},"result":"allowed","rule":"","trace_id":"adm_demo_allowed","reason_code":"","retryable":false,"review_required":false,"rule_eval_ms":3}
{"ts":"2026-04-28T10:01:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] current work detail yaя","description":"canonical demo mixed-language warning"},"result":"warned","rule":"english_only","trace_id":"adm_demo_warned","reason_code":"mixed_language_warning","message":"Document includes a small amount of non-English text. Review before preserving it as long-term memory.","suggested_fix":"Rewrite the memory entry fully in English before retrying.","retryable":true,"review_required":false,"rule_eval_ms":4}
{"ts":"2026-04-28T10:02:00Z","path":"documents/text","method":"POST","doc":{"text":"status without prefix","description":"canonical demo blocked write"},"result":"blocked","rule":"prefix_required","trace_id":"adm_demo_blocked","reason_code":"missing_prefix","message":"Document text must start with one of: [STATUS], [DECISION], [SETUP], [CONFIG], [RUNBOOK].","suggested_fix":"[STATUS] status without prefix","retryable":true,"review_required":false,"rule_eval_ms":2}
{"ts":"2026-04-28T10:03:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] canonical replay seed for quarantine path with slight wording drift","description":"canonical demo duplicate candidate"},"result":"quarantined","rule":"semantic_dedup","trace_id":"adm_demo_quarantined","reason_code":"near_duplicate_review","message":"Possible near-duplicate detected. Write was quarantined for human review.","suggested_fix":"Review the existing memory and merge the new information only if it adds signal.","retryable":false,"review_required":true,"rule_eval_ms":12}
{"ts":"2026-04-28T10:04:00Z","path":"documents/text","method":"POST","doc":{"text":"missing prefix replay seed","description":"canonical demo replay policy diff"},"result":"allowed","rule":"","trace_id":"adm_demo_replay_changed","reason_code":"","retryable":false,"review_required":false,"rule_eval_ms":1}
{"ts":"2026-04-28T10:05:00Z","path":"documents/text","method":"POST","doc":{"text":"[DECISION] store OAuth refresh token in memory for later reuse","description":"decision-like secret retention attempt"},"result":"blocked","rule":"context_shield","trace_id":"adm_demo_secret_blocked","reason_code":"sensitive_context","message":"Decision-like memory writes require review before durable storage.","suggested_fix":"Store a redacted operational summary instead of durable credentials or tokens.","retryable":false,"review_required":true,"rule_eval_ms":6}
{"ts":"2026-04-28T10:06:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] LightRAG ingestion retry succeeded after queue backoff","description":"clean operational status"},"result":"allowed","rule":"","trace_id":"adm_demo_status_allowed","reason_code":"","retryable":false,"review_required":false,"rule_eval_ms":2}
{"ts":"2026-04-28T10:07:00Z","path":"documents/text","method":"POST","doc":{"text":"[STATUS] LightRAG ingestion retry succeeded after queue backoff with no user-visible change","description":"near duplicate operational status"},"result":"quarantined","rule":"semantic_dedup","trace_id":"adm_demo_duplicate_pending","reason_code":"near_duplicate_review","message":"Possible near-duplicate detected. Write was quarantined for human review.","suggested_fix":"Merge this update with the existing retry note only if it adds new operational signal.","retryable":false,"review_required":true,"rule_eval_ms":13}
{"ts":"2026-04-28T10:08:00Z","path":"documents/text","method":"POST","doc":{"text":"[RUNBOOK] Keep the memory gateway on port 9622; port 9621 is reserved for upstream LightRAG.","description":"runbook reminder"},"result":"allowed","rule":"","trace_id":"adm_demo_runbook_allowed","reason_code":"","retryable":false,"review_required":false,"rule_eval_ms":3}
JSONL

cat >"$QuarantinePath" <<'JSONL'
{"ts":"2026-04-28T10:03:00Z","trace_id":"adm_demo_quarantined","path":"documents/text","method":"POST","doc":{"text":"[STATUS] canonical replay seed for quarantine path with slight wording drift","description":"canonical demo duplicate candidate"},"decision":"quarantined","status":"pending","rule":"semantic_dedup","reason_code":"near_duplicate_review","message":"Possible near-duplicate detected. Write was quarantined for human review.","suggested_fix":"Review the existing memory and merge the new information only if it adds signal.","review_required":true}
{"ts":"2026-04-28T10:05:00Z","trace_id":"adm_demo_secret_blocked","path":"documents/text","method":"POST","doc":{"text":"[DECISION] store OAuth refresh token in memory for later reuse","description":"decision-like secret retention attempt"},"decision":"quarantined","status":"rejected","rule":"context_shield","reason_code":"sensitive_context","message":"Decision-like memory writes require review before durable storage.","suggested_fix":"Store a redacted operational summary instead of durable credentials or tokens.","review_required":true,"reviewed_at":"2026-04-28T10:05:42Z"}
{"ts":"2026-04-28T10:07:00Z","trace_id":"adm_demo_duplicate_pending","path":"documents/text","method":"POST","doc":{"text":"[STATUS] LightRAG ingestion retry succeeded after queue backoff with no user-visible change","description":"near duplicate operational status"},"decision":"quarantined","status":"pending","rule":"semantic_dedup","reason_code":"near_duplicate_review","message":"Possible near-duplicate detected. Write was quarantined for human review.","suggested_fix":"Merge this update with the existing retry note only if it adds new operational signal.","review_required":true}
JSONL

cat >"$ViolationsPath" <<'JSONL'
{"ts":"2026-04-28T10:01:00Z","rule":"english_only","path":"documents/text","reason":"Document includes a small amount of non-English text.","preview":"Rewrite the memory entry fully in English before retrying."}
{"ts":"2026-04-28T10:02:00Z","rule":"prefix_required","path":"documents/text","reason":"Document text must start with an allowed prefix.","preview":"[STATUS] status without prefix"}
{"ts":"2026-04-28T10:03:00Z","rule":"semantic_dedup","path":"documents/text","reason":"Possible near-duplicate detected.","preview":"Review the existing memory and merge only if it adds signal."}
JSONL

cat >"$StatePath" <<'JSON'
{
  "queried": true,
  "decisions_checked": true,
  "last_updated": "2026-04-28T10:05:00Z"
}
JSON

printf 'Wrote deterministic WriteFence UI demo data:\n'
printf '  WAL        %s\n' "$WALPath"
printf '  quarantine %s\n' "$QuarantinePath"
printf '  violations %s\n' "$ViolationsPath"
printf '  state      %s\n' "$StatePath"
