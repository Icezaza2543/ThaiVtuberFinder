# ThaiVtuberFinder

**ค้นหาบัญชี VTuber → เทียบฐานเดิม → ส่งให้ตรวจใน `ThaiVtuber_DATA`**

Go + native SQLite + Docker. Finder แยกจาก SNA และ canonical registry โดยชัดเจน ไม่มี Python runtime, Redis, browser engine หรือ Go dependencies ที่ต้องดาวน์โหลดเพิ่ม

```text
Kerlos/Chuysan JSON · Bluesky lists/starter packs · curated rosters · JSON/JSONL/CSV
                                  ↓
                     normalize → resolve → deduplicate
                                  ↓
                          SQLite candidate cache
                                  ↓
                      ThaiVtuber_DATA / FINDER_INBOX
                                  ↓ human review
                         schema-v2 proposal JSON
```

**เก็บ `source_url` ในแถวโดยตรง ไม่มีตาราง `EVIDENCE` แยก**. Name/handle changes do not create a new account when its stable platform ID is unchanged. Persona ownership is never inferred from matching display names, voices, or a presumed shared operator.

## เริ่มใช้ด้วย Docker

```bash
git clone https://github.com/Icezaza2543/ThaiVtuberFinder.git
cd ThaiVtuberFinder
cp .env.example .env
mkdir -p secrets
docker compose up -d --build
docker compose logs -f --tail=50 finder
```

Windows PowerShell ใช้ `Copy-Item .env.example .env` และ `New-Item -ItemType Directory -Force secrets` แทน `cp`/`mkdir -p`.

ค่าเริ่มต้นค้นจาก JSON feed ที่ Kerlos ใช้จริง ทุก 6 ชั่วโมง เก็บ candidate ลง SQLite ใน persistent Docker volume แต่ **ยังไม่เขียน Google Sheet** จนกว่าจะตั้ง credentials. Process ทำงานต่อเนื่องระหว่างรอบ ไม่ต้อง cron และ restart ได้โดยไม่ทิ้ง candidate/quota state.

### เชื่อม Google Sheet

1. Enable Google Sheets API in the Google Cloud project used by your service account.
2. Store the service-account JSON at `secrets/google-service-account.json`. Keep this file out of Git.
3. Share **เฉพาะ workbook `ThaiVtuber_DATA`** with the JSON's `client_email` as Editor. Do not make the workbook public.
4. Set `.env`:

```dotenv
SHEETS_SYNC=true
GOOGLE_SHEET_ID=your_workbook_id
GOOGLE_APPLICATION_CREDENTIALS=secrets/google-service-account.json
YOUTUBE_API_KEY=
```

Docker Compose maps the credential file to `/run/secrets/google-service-account.json` automatically. Restart after editing settings:

```bash
docker compose up -d --force-recreate
```

`YOUTUBE_API_KEY` เป็น optional: ใช้ resolve YouTube handles/video URLs และ optional search. Kerlos/Chuysan และ public Bluesky list ไม่ใช้ key. `youtube_daily_budget` ค่าเริ่มต้น 500 units/day เป็นงบของ Finder เท่านั้น ต้องแบ่งจาก quota project เดียวกันกับ worker อื่นด้วย. Search debits 100 units/request; channel/video resolution debits 1. Failed/retried requests are debited too; the ledger resets by America/Los_Angeles calendar day.

## สิ่งที่ทำได้ในรุ่นนี้

| Component | Behavior |
|---|---|
| Kerlos/Chuysan | Reads the actual `simple_list.json` feed; checks `result[]` and channel IDs. Labels directory membership as `DIRECTORY_LISTED`, not verified ownership. |
| Bluesky | Public `getList` with bounded pagination; starter-pack URL/AT-URI → list → member DIDs. |
| YouTube | Resolves `/channel/UC…`, `@handle`, legacy `/user/`, and video/Shorts/live URLs. `/c/` without stable ID stays unresolved. Optional explicitly configured channel search. |
| HTML roster | Extracts supported account links from explicitly configured static event/agency pages. Does not execute JavaScript or crawl arbitrary linked sites. |
| Imports | JSONL/CSV records; JSON arrays or `{ "results": [...] }` screening exports. |
| Dedupe | Exact stable platform ID first; unresolved URL identities are provisional. Display names never merge accounts/personas. |
| Canonical comparison | Reads `ACCOUNTS`, `ACCOUNT_LINKS`, `PERSONAS`; distinguishes `KNOWN_LINKED` from `KNOWN_ACCOUNT` still needing review. Also accepts registry/bootstrap JSON. |
| Sheets sync | Adds new inbox rows; updates machine columns A:K and T only on existing rows. Human decisions in L:S are preserved. Uses RAW values, bounded batches, retry/backoff and grid-growth safety checks. |
| Export | Exports only explicitly verified review decisions with reviewer, RFC3339 review time, source URL, stable account ID and valid persona target. Never auto-pushes to upstream repos. |
| Runtime | Process lock, SQLite transactions/WAL, persistent request budget, timeout/cancellation, health endpoints, non-root Docker. |

## วิธี review

`FINDER_INBOX` columns **L:S เป็นของคนตรวจ**:

- Set `review_status=verified` only after reviewing the account.
- Choose `action`: `create_persona`, `link_persona`, `account_only`, or `ignore`.
- For `link_persona`, `target_persona_id` must exist in canonical `PERSONAS`.
- For `create_persona`, leave target ID empty and fill `persona_name`.
- Set `account_type`, `reviewer`, and `reviewed_at` (for example `2026-09-22T10:30:00+07:00`).

Organization/group accounts use `account_only`, not a persona link. A new persona/model/re-debut remains a separate entity. Classification is a suggestion, never automatic verification.

Export reviewed proposals while the worker runs:

```bash
docker compose exec -T finder finder export -config /app/config/finder.json > handoff.json
```

The handoff is **Finder proposal schema v2**, not a drop-in legacy SNA review file. See [docs/HANDOFF.md](docs/HANDOFF.md). It intentionally does not mutate canonical tables or either upstream repository; their consumer must validate/apply the proposal. No upstream compatibility or live deployment is claimed merely because export succeeds.

## Configuration and local commands

Edit `config/finder.json`. Unconfigured optional sources are disabled. Enable a roster/list only after supplying its real URL/AT-URI. Add your own agency and event pages as `html` entries; a JavaScript-only page needs a JSON feed/adapter.

Build locally on Linux (Docker is the recommended Windows path):

```bash
sudo apt-get install build-essential pkg-config libsqlite3-dev
make check
./bin/finder demo
./bin/finder once
./bin/finder worker
```

Go language floor: 1.23; Docker/CI use Go 1.26.x. Local compilation needs CGO and native SQLite >= 3.24. The native wrapper is small and parameterized; it is not a general database/sql driver. Keep the Go/Debian images updated and rebuild for security fixes.

```bash
# Stop the worker before running another database-writing command.
./bin/finder import -file data/leads.jsonl -kind jsonl
./bin/finder import -file data/all_884_screening_results.json -kind json
./bin/finder compare -canonical data/bootstrap.json -out exports/canonical-diff.json
./bin/finder status
./bin/finder doctor
```

`status`, `compare` and `doctor` currently acquire the same process lock as other local database commands: stop the worker first. For a running container use the read-only health endpoints below. `export` only reads Sheets and can run concurrently.

Import schema (fictional example):

```json
{"url":"https://www.youtube.com/@example","display_name":"Example Ch.","source_url":"https://example.org/roster"}
```

## 24/7 operation

Deploy the Docker image on a host that runs continuously with persistent local disk. Use **one replica and one writer per workbook**. Do not share SQLite over network storage. A free sleeping service or ephemeral filesystem is not sufficient. This repository does not provision a host, enable billing, or upload credentials.

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

`/healthz` tests the process/database. `/readyz` is 503 before the first completed successful cycle or after an incomplete one. A live process does not prove sources are complete. Do not physically sort/insert/delete inbox rows during sync; use filter views. Human edits in L:S are safe from the worker's existing-row writes. Independent workers with separate database files cannot coordinate a shared workbook.

Back up the Docker volume with the worker stopped (or use an SQLite-aware online backup). Keep Google Sheets access limited to the project team. The public GitHub repo contains code/tests only: `.env`, credentials, databases and exports are excluded. API data retention and research authorization remain the operator's responsibility.

## Tests and limitations

```bash
go test -race -count=1 ./...
go vet ./...
go run ./cmd/finder demo
```

Tests cover account normalization/dedupe, restarts, quota persistence, partial sources, HTTP retries, Google JWT signing, repeated Sheets sync, curator-column preservation and proposal validation. Offline demo imports 3 synthetic observations into 2 unique accounts. Live YouTube/Google OAuth integration requires your credentials and must be checked after deployment. Docker build is tested by the included GitHub Actions workflow; check its actual run result rather than assuming it passed.

Discovery is bounded and source-dependent, not a complete census. Page caps and failures are reported; valid partial results are retained. Restart resumes through idempotent records and the resolver cache, not a saved cursor for every remote source. Handles on platforms without an implemented stable-ID resolver remain pending, and proposal export refuses them rather than inventing IDs. The first implementation has no automatic visual/model classifier, browser scraping fallback, or direct canonical promotion command.

## References

- [Kerlos data source definition](https://github.com/kerlos/thai-vtuber/blob/main/hooks/useVTuberData.ts)
- [Kerlos feed shape](https://github.com/kerlos/thai-vtuber/blob/main/types/vtuber.ts)
- [YouTube channels.list](https://developers.google.com/youtube/v3/docs/channels/list)
- [YouTube search.list](https://developers.google.com/youtube/v3/docs/search/list)
- [Bluesky public API](https://docs.bsky.app/docs/advanced-guides/api-directory)
- [Google Sheets API quotas](https://developers.google.com/workspace/sheets/api/limits)
- [Google service-account OAuth](https://developers.google.com/identity/protocols/oauth2/service-account)
- [SQLite C interface](https://www.sqlite.org/cintro.html)
