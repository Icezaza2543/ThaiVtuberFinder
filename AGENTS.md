# Working on ThaiVtuberFinder

- Keep Finder separate from canonical data and SNA analytics.
- Go + native SQLite + Docker. Use parameterized statements and a single workbook writer.
- Persona != account. Never infer shared operators from names, voices or handles.
- Public creator discovery only; no viewer/comment/live-chat account harvesting.
- Keep source_url inline. Do not reintroduce an EVIDENCE table.
- FINDER_INBOX L:S are curator-owned; sync may update A:K and T only on existing rows.
- A new persona/model/re-debut stays distinct. Source classification is only a hint, never verification.
- Don't invent stable IDs. Require stable platform_id and explicit reviewed decisions before proposal export.
- Don't bypass errors by changing unknown accounts to rejected or verified.
- No API keys, service-account credentials, runtime databases, or collected datasets in Git.
- Run `go test -race ./...`, `go vet ./...`, `go build ./cmd/finder`, and `go run ./cmd/finder demo` before claiming completion.
- Do not auto-apply proposals to the upstream repositories; target adapters require their current schema and separate authorization.
