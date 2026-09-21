# ThaiVtuberFinder v0.1

Implementation of the agreed Go + SQLite + Docker Finder, separate from ThaiVtuberSNA.

- Go standard library, native SQLite through a small bound-parameter C API adapter. No Go module downloads; Linux Docker is the portable deployment path. No Python runtime.
- Public creator/account discovery only: curated HTML rosters/directories, JSONL/CSV imports, Bluesky lists/starter packs, optional YouTube search and channel resolution.
- Stable platform IDs identify accounts; display names never join identities. A new persona/model/re-debut remains a different persona. No inferred operator identity.
- SQLite holds deduplicated candidates, per-source run state, resolver cache, and a persistent daily YouTube request budget. One worker/writer per database/workbook.
- ThaiVtuber_DATA remains canonical. Finder reads ACCOUNTS and writes FINDER_INBOX only. It preserves curator columns L:S. No separate EVIDENCE table: source_url is stored inline.
- Source failures remain visible, are isolated, and cannot become empty-success results. Page/request/time limits are explicit. Read requests retry transient errors; deterministic row writes are idempotent under the single-writer requirement.
- Reviewed rows can be exported as schema-v2 proposals. Export never changes either upstream repository or canonical tables. An upstream consumer must validate/apply the proposal contract.
- Secrets and runtime data stay outside Git. Offline demo/tests contain synthetic accounts only. Do not confuse 'runnable/deployable' with 'deployed/live credentials configured'.

## Tradeoffs
Native SQLite avoids transpilation overhead and extra services; it requires a C compiler/libsqlite3 for local builds. Docker installs those. This is a bounded crawler, not a browser automation engine: JavaScript-only or blocked directories need an explicit supported adapter/feed. No performance multiplier is asserted without a benchmark.
