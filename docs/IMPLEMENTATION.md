# Implementation plan and verification

1. Model/storage: failing tests for URL normalization, stable identity dedupe, unresolved-to-resolved upgrades, persistence and quota; implement bound-parameter SQLite wrapper/store.
2. Discovery: failing HTTP fixture tests for curated rosters, Bluesky pagination, YouTube quota/resolve, malformed pages, non-public destinations and retries; implement bounded adapters.
3. Sheets: failing tests for exact schema, duplicate detection, curator-column preservation, idempotent retry and reviewed proposal validation; implement service-account OAuth and row-addressed sync.
4. CLI/runtime: configuration, process lock, one-shot/continuous worker, health, JSONL import/export, offline demo, Docker/Compose, CI, deployment documentation.
5. Run go test -race ./..., go vet ./..., build binary, offline end-to-end demo and repeated sync simulation. Record measured results; publish only source/config/tests/docs, no credentials or harvested data.

## Execution rulings
- Existing repository was confirmed empty; use its configured default branch `master` without rewriting any history.
- External network/DNS is not available to the build container. Use installed native SQLite rather than an untestable downloaded driver. Live connector inspection confirmed the Google Sheet v2 headers.
- No cloud deployment or billing is performed in this task.
