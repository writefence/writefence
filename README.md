# WriteFence

WriteFence is an admission controller for agent memory writes.

It sits between an agent and its persistent memory store, evaluates every write attempt, and decides what is allowed to become long-term memory before anything reaches the store.

## Why it exists

Agents can write:
- malformed memory
- duplicated memory
- mixed-language or low-signal operational noise
- contradictory or suspicious long-term state

WriteFence governs the write path with a four-state admission model:
- `allowed`
- `warned`
- `quarantined`
- `blocked`

## Architecture

```text
Agent / MCP client / app
          |
          v
    WriteFence proxy :9622
          |
          +--> allowed / warned      -> forward to store
          |
          +--> quarantined           -> local quarantine log only
          |
          +--> blocked               -> reject with ADC
          v
   LightRAG or compatible store
```

## Admission Decision Contract

Every write returns a machine-readable admission decision contract.

Example blocked response:

```json
{
  "decision": "blocked",
  "rule_id": "prefix_required",
  "reason_code": "missing_prefix",
  "message": "Document text must start with one of: [STATUS], [DECISION], [SETUP], [CONFIG], [RUNBOOK].",
  "suggested_fix": "[STATUS] current work",
  "retryable": true,
  "retry_after": "",
  "review_required": false,
  "trace_id": "adm_3f8a9c..."
}
```

## Current rule set

- `english_only`
- `prefix_required`
- `context_shield`
- `status_dedup`
- `semantic_dedup`

## Local alpha quickstart

For a complete local preview without LightRAG, Qdrant, Ollama, auth, billing,
or hosted services, use the mock memory store:

```bash
go build -o bin/writefence ./cmd/writefence
go build -o bin/writefence-cli ./cmd/writefence-cli
go run ./demo/mock-memory-store.go -addr 127.0.0.1:9621
```

In another shell:

```bash
WRITEFENCE_DATA_DIR=/tmp/writefence-alpha \
  ./bin/writefence --addr 127.0.0.1:9622 --upstream http://127.0.0.1:9621
```

Open the local operator UI:

```text
http://127.0.0.1:9622/_writefence
```

Run the demo flow:

```bash
WRITEFENCE_DATA_DIR=/tmp/writefence-alpha \
  WRITEFENCE_WAL=/tmp/writefence-alpha/writefence-wal.jsonl \
  ./demo/canonical-demo.sh
```

See [docs/quickstart.md](docs/quickstart.md) for the full local alpha path,
[docs/configuration.md](docs/configuration.md) for policy configuration, and
[docs/compatibility.md](docs/compatibility.md) for upstream compatibility.

## LightRAG quickstart

```bash
go build -o bin/writefence ./cmd/writefence
go build -o bin/writefence-cli ./cmd/writefence-cli
./bin/writefence --addr :9622 --upstream http://127.0.0.1:9621
```

In another shell:

```bash
./bin/writefence-cli rules list
./bin/writefence-cli replay
./demo/canonical-demo.sh
```

The UI is local-only and reads the existing WAL, quarantine log, replay engine,
and runtime config. It is not a hosted SaaS surface.

## Canonical demo

The repo includes a single demo script:

```bash
./demo/canonical-demo.sh
```

It shows:
1. block -> suggested fix -> corrected write admitted
2. quarantine -> human approve/reject
3. replay -> policy diff over WAL

Notes:
- quarantine requires semantic dedup to be enabled on the running proxy
- the script assumes WriteFence is already running on `http://127.0.0.1:9622`

## Local UI Screenshots

Deterministic screenshot fixtures can be generated with:

```bash
WRITEFENCE_DEMO_OVERWRITE=1 ./demo/ui-demo-data.sh /tmp/writefence-ui-demo
WRITEFENCE_DATA_DIR=/tmp/writefence-ui-demo ./bin/writefence --addr :9622 --upstream http://127.0.0.1:9621
./demo/capture-ui-screenshots.py --url http://127.0.0.1:9622/_writefence --out docs/assets/ui
```

Current screenshots are stored in [docs/assets/ui](docs/assets/ui).

## CLI

Available commands:

```text
writefence-cli status
writefence-cli rules list
writefence-cli violations tail
writefence-cli violations report
writefence-cli quarantine list
writefence-cli quarantine approve <trace_id>
writefence-cli quarantine reject <trace_id>
writefence-cli replay [--wal path] [--config path]
```

## Configuration

WriteFence alpha supports YAML configuration for proxy settings and built-in
rule parameters. Users can tune thresholds, allowed prefixes, log paths, and
optional semantic dedup dependencies. Arbitrary custom rule definitions and
hosted policy management are not part of the alpha scope.

See [docs/configuration.md](docs/configuration.md).

## Repository layout

```text
cmd/writefence/              proxy server
cmd/writefence-cli/          operator CLI
cmd/writefence-mcp/          MCP server
internal/admission/     admission decision contract
internal/proxy/         HTTP reverse proxy
internal/quarantine/    local quarantine workflow
internal/replay/        WAL replay engine
internal/rules/         built-in policy rules
internal/wal/           write-ahead log
demo/                   canonical demo
```

## Status

WriteFence is pre-1.0 software. The core local admission-controller workflow is
in place:
- four-state admission model
- admission decision contract
- local quarantine workflow
- WAL replay engine v1
- canonical demo script
- local operator UI
- deterministic UI screenshots

APIs, CLI output, and on-disk log formats may still change before a stable
release.

## Contributing

External pull requests require a signed Contributor License Agreement before
merge. See [CONTRIBUTING.md](CONTRIBUTING.md) and [CLA.md](CLA.md).

## Security

Please report suspected vulnerabilities privately. See
[SECURITY.md](SECURITY.md).

## License

Apache-2.0. See [LICENSE](LICENSE).
