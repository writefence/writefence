# Local Alpha Quickstart

This quickstart runs WriteFence locally with a tiny mock memory store. It does
not require LightRAG, Qdrant, Ollama, auth, billing, or hosted services.

## 1. Build the binaries

```bash
go build -o bin/writefence ./cmd/writefence
go build -o bin/writefence-cli ./cmd/writefence-cli
```

## 2. Start the mock memory store

```bash
go run ./demo/mock-memory-store.go -addr 127.0.0.1:9621
```

The mock store accepts `POST /documents/text` and keeps accepted writes in a
local JSONL file under `/tmp/writefence-mock-store` by default.

## 3. Start WriteFence

In another shell:

```bash
WRITEFENCE_DATA_DIR=/tmp/writefence-alpha \
  ./bin/writefence --addr 127.0.0.1:9622 --upstream http://127.0.0.1:9621
```

Open the local operator UI:

```text
http://127.0.0.1:9622/_writefence
```

## 4. Run the demo flow

In a third shell:

```bash
WRITEFENCE_DATA_DIR=/tmp/writefence-alpha \
  WRITEFENCE_WAL=/tmp/writefence-alpha/writefence-wal.jsonl \
  ./demo/canonical-demo.sh
```

The demo shows:

- a blocked write with a suggested fix;
- a corrected write that is admitted;
- replay over the local WAL.

The semantic quarantine path requires embeddings plus Qdrant. Without those
optional services, the demo prints a note and continues.

## 5. Load deterministic UI data

For a screenshot-friendly local UI preview:

```bash
WRITEFENCE_DEMO_OVERWRITE=1 ./demo/ui-demo-data.sh /tmp/writefence-ui-demo
WRITEFENCE_DATA_DIR=/tmp/writefence-ui-demo \
  ./bin/writefence --addr 127.0.0.1:9622 --upstream http://127.0.0.1:9621
```

Then open:

```text
http://127.0.0.1:9622/_writefence
```
