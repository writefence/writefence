# Contributing To WriteFence

WriteFence is pre-1.0 infrastructure. Keep contributions focused on the
admission-controller wedge: policy decisions before agent memory writes,
auditable quarantine, and WAL-backed replay.

## Before Opening A Pull Request

1. Open an issue or discussion for non-trivial changes.
2. Keep changes small and scoped.
3. Run:

```bash
go test ./...
go vet ./...
```

4. Update docs when behavior or CLI output changes.

## Contributor License Agreement

External pull requests require a signed Contributor License Agreement before
merge.

The CLA text is in [CLA.md](CLA.md). The hosted CLA Assistant gate must be live
before the repository accepts outside contributions.

Maintainers should reject or defer external pull requests until the CLA status
check passes.

## Pull Request Expectations

- Explain the behavior change and why it belongs in the core wedge.
- Include tests for behavior changes.
- Avoid broad refactors mixed with feature work.
- Do not add telemetry, network calls, or new external dependencies without a
  separate design decision.
- Do not turn WriteFence into a memory store, retrieval layer, or autonomous
  memory editor.

## Security

Do not open public issues for suspected vulnerabilities once the repository is
public. Follow [SECURITY.md](SECURITY.md).
