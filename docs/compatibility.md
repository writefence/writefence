# Compatibility

WriteFence is a local admission controller for memory-write traffic. The alpha
target is intentionally narrow: put WriteFence in front of a memory store that
accepts HTTP document writes, then inspect decisions through the CLI, WAL,
quarantine log, replay engine, and local UI.

For policy and runtime settings, see [configuration.md](configuration.md).

## Works today

- HTTP reverse-proxy deployment.
- LightRAG-style `POST /documents/text` writes.
- LightRAG-style `POST /documents/paginated` reads for local dedup checks.
- LightRAG-style `DELETE /documents/delete_document` for dedup merge cleanup.
- Local WAL and violation logs.
- Local quarantine log with approve/reject review.
- Optional semantic dedup with Ollama embeddings and Qdrant.
- Local operator UI at `/_writefence`.
- MCP server exposing operator tools.

## Expected upstream endpoints

Write admission is enforced on:

```text
POST /documents/text
```

The accepted request body should include:

```json
{
  "text": "[STATUS] memory text",
  "description": "optional description"
}
```

For dedup support, the upstream should also provide:

```text
POST /documents/paginated
DELETE /documents/delete_document
```

Other paths are proxied through without admission decisions.

## Optional semantic dedup

Semantic dedup is enabled only when both embedding and Qdrant URLs are
configured:

```bash
./bin/writefence \
  --addr 127.0.0.1:9622 \
  --upstream http://127.0.0.1:9621 \
  --embed-url http://127.0.0.1:11434 \
  --qdrant-url http://127.0.0.1:6333
```

If either dependency is absent, semantic dedup is disabled and WriteFence keeps
running with deterministic local rules.

## Not in the alpha scope

- Hosted SaaS.
- Billing.
- Multi-tenant administration.
- User authentication.
- Cloud-managed memory stores.
- Generic adapters for every memory API shape.

Adapters for additional memory backends should be small and explicit: normalize
the backend's write path into WriteFence's document-write contract, then let
the existing admission, WAL, quarantine, replay, and UI surfaces do the work.
