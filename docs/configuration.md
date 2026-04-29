# Configuration

WriteFence alpha is configured with a YAML file plus optional CLI flag
overrides. The current policy surface is intentionally small: users can tune
the built-in rules, but cannot define arbitrary custom rules yet.

## Example

```yaml
proxy:
  addr: "127.0.0.1:9622"
  upstream: "http://127.0.0.1:9621"
  state_file: "/tmp/writefence-alpha/session-state.json"
  violations_log: "/tmp/writefence-alpha/writefence-violations.jsonl"
  wal_log: "/tmp/writefence-alpha/writefence-wal.jsonl"
  quarantine_log: "/tmp/writefence-alpha/writefence-quarantine.jsonl"
  metrics_enabled: true

rules:
  english:
    threshold: 0.05
  prefix:
    allowed:
      - "[STATUS]"
      - "[DECISION]"
      - "[SETUP]"
      - "[CONFIG]"
      - "[RUNBOOK]"
  semantic_dedup:
    threshold: 0.98
    embed_url: "http://127.0.0.1:11434"
    embed_model: "qwen3-embedding:8b"
    qdrant_url: "http://127.0.0.1:6333"
```

Run with:

```bash
./bin/writefence --config ./writefence.yaml
```

CLI flags override YAML values when they are provided explicitly:

```bash
./bin/writefence \
  --config ./writefence.yaml \
  --addr 127.0.0.1:9622 \
  --upstream http://127.0.0.1:9621
```

Use `--metrics=false` to disable `/metrics` regardless of the YAML value.

## Proxy settings

- `addr` controls where WriteFence listens.
- `upstream` points to the memory store that receives accepted writes.
- `state_file` stores local session state.
- `violations_log` stores blocked, warned, and quarantined write events.
- `wal_log` stores the write-ahead log used by replay.
- `quarantine_log` stores pending, approved, and rejected quarantine entries.
- `metrics_enabled` controls whether Prometheus-compatible metrics are served
  at `/metrics`.

If `WRITEFENCE_DATA_DIR` is set, default local files are created under that
directory. Otherwise WriteFence uses `~/.writefence`.

## Rule settings

### `english`

`threshold` controls when mixed Cyrillic/English content becomes a hard block.
Small amounts above the warning threshold may be admitted with a warning; larger
amounts are blocked with a retryable admission decision.

### `prefix`

`allowed` is the list of required document prefixes. A write to
`POST /documents/text` must start with one of these values. This gives memory
entries a predictable category before they become durable state.

### `semantic_dedup`

Semantic deduplication is enabled only when both `embed_url` and `qdrant_url`
are configured. If either dependency is absent, WriteFence keeps running with
deterministic local rules.

- `threshold` controls the similarity score for a hard duplicate block.
- Scores below the hard block threshold but above the quarantine threshold are
  held for human review.
- `embed_model` is passed to the embedding provider.

## Alpha scope

The alpha does not include:

- a custom policy DSL;
- external rule plugin loading;
- hosted policy management;
- multi-tenant auth or billing;
- generic adapters for every memory backend.

Those are later product decisions. The alpha goal is to validate the local
write-admission loop: block, warn, quarantine, replay, and inspect decisions.
