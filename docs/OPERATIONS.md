# Operations checklist

## Before live sync

1. `go test -race ./...` and `finder demo` succeed.
2. Google Sheet v2 has exact FINDER_INBOX headers; canonical ACCOUNTS/ACCOUNT_LINKS/PERSONAS headers exist.
3. Service-account JSON is outside Git, the sheet is shared only with intended identities, and Sheets API is enabled.
4. Configure the correct workbook ID in `.env`. Do not copy example/synthetic accounts into the production sheet.
5. Coordinate YouTube project quota with the main collector. Finder uses its own conservative per-day ledger; it does not know other processes' usage.
6. Run one foreground cycle and review its JSON report before enabling continuous mode.
7. Check Google Sheets permissions, source terms and retention obligations for the actual deployment.

## Recovery

- SQLite identity and request budget persist in `/app/data`. Keep that volume across deploys.
- The process lock prevents two workers using one database. It cannot prevent independent workers using different volumes from writing the same workbook.
- A crash after a Google write but before the next local cycle does not use a blind append: the next run re-reads the inbox and writes fixed ranges.
- Reconciliation also checks stable account keys if local candidate IDs were lost. It retains the existing sheet candidate ID and earliest first_seen.
- Source timeout, malformed JSON, changed feed schema or pagination cap are visible errors, not silent empty successes.
- Stop the worker before running database-import/status/compare/doctor commands. Export and health checks are read-only and can run while the worker runs.
- Make SQLite-consistent backups; copying the .sqlite file alone while WAL is active can lose data.

## Security boundaries

Outbound crawler connections block non-public resolved addresses, nonstandard ports, embedded credentials and HTTPS-to-HTTP redirects. Requests and responses are bounded. Service-account OAuth has a fixed token endpoint and signs only the Sheets scope. API keys are sent in headers, and response bodies/URLs with credentials are not echoed in HTTP errors. Sheets values use RAW writes so strings beginning with `=` remain data.

No browser automation, viewer tracking, identity inference, automatic model/persona merging, automatic canonical promotion or repo-to-repo pushes are implemented.
