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
          +-- allowed / warned  ---> LightRAG or compatible store
          |
          +-- quarantined       ---> local quarantine log only
          |
          +-- blocked           ---> reject with ADC
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

For the most reliable first run, use the smoke script. It builds the binaries,
starts the mock memory store and WriteFence on temporary ports, verifies the
operator UI, checks blocked and allowed writes, runs the canonical demo, and
cleans up background processes.

```bash
./demo/smoke-quickstart.sh
```

Manual path:

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

## Docker quickstart

Docker Compose starts WriteFence with the included mock memory store:

```bash
docker compose up --build
```

Then open:

```text
http://127.0.0.1:9622/_writefence
```

Stop and remove the local stack with:

```bash
docker compose down
```

This leaves the named demo volumes in place. To start fully fresh:

```bash
docker compose down -v
```

If host port `9622` is busy:

```bash
WRITEFENCE_PORT=19622 docker compose up --build
```

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
2. optional semantic quarantine -> human approve/reject when semantic dedup dependencies are configured
3. replay -> policy diff over WAL

Notes:
- quarantine requires semantic dedup to be enabled on the running proxy
- without embeddings and Qdrant, the demo prints a note and continues with deterministic local rules
- the script assumes WriteFence is already running on `http://127.0.0.1:9622`

## Troubleshooting

- Use Go 1.25 or newer for source builds.
- If ports `9621` or `9622` are busy, either use `./demo/smoke-quickstart.sh`
  or pass different `--addr` / `--upstream` values.
- The smoke script uses ports `19621` and `19622` by default. Override with
  `WRITEFENCE_SMOKE_STORE_PORT` and `WRITEFENCE_SMOKE_PROXY_PORT`.
- Set `WRITEFENCE_DATA_DIR=/path/to/dir` to keep logs and WAL files in a
  predictable directory.
- In Docker Compose, only the WriteFence proxy is published to the host. The
  mock store stays internal so writes still pass through admission.
- Semantic quarantine is optional. It requires embeddings plus Qdrant; without
  them, WriteFence still runs deterministic local rules and replay.
- On Windows, use WSL or Docker for the alpha. Native Windows release archives
  are planned after the alpha.

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
cmd/writefence/            proxy server
cmd/writefence-cli/        operator CLI
cmd/writefence-mcp/        MCP server
internal/admission/        admission decision contract
internal/proxy/            HTTP reverse proxy
internal/quarantine/       local quarantine workflow
internal/replay/           WAL replay engine
internal/rules/            built-in policy rules
internal/wal/              write-ahead log
demo/                      canonical demo
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
