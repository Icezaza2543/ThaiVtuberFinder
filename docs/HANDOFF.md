# Finder proposal schema v2

The upstream canonical schema uses separate personas, accounts and account_links.
Finder exports proposed changes only; the receiver decides canonical IDs and performs its own transaction/review checks.

```json
{
  "schema_version": 2,
  "producer": "ThaiVtuberFinder",
  "generated_at": "2026-09-22T00:00:00Z",
  "proposals": [
    {
      "candidate_id": "candidate_synthetic_example",
      "action": "link_persona",
      "target_persona_id": "persona_existing_example",
      "persona_name": "Example Ch.",
      "account": {
        "platform": "youtube",
        "platform_id": "UCaaaaaaaaaaaaaaaaaaaaaa",
        "handle": "@example",
        "display_name": "Example Ch.",
        "canonical_url": "https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa"
      },
      "account_type": "persona",
      "source_url": "https://example.org/roster",
      "reviewer": "owner:manual-review",
      "reviewed_at": "2026-09-22T00:00:00Z"
    }
  ]
}
```

Rules:

- Treat `candidate_id` as the idempotency key for the review decision, not a persona ID.
- Account identity is `platform + platform_id`; handles/display names are metadata.
- `create_persona` means a genuinely separate persona; do not name-match it onto an existing one.
- `link_persona` references an explicit existing canonical ID.
- `account_only` records an organization/group/otherwise unlinked account, with no implied persona.
- No `EVIDENCE` table or evidence foreign key is required; retain `source_url` and review attribution inline.
- The receiver must re-check foreign keys, duplicate accounts, valid-from/to boundaries, model/persona rules and source authorization.
- This payload does not rewrite, delete or replace historical registry data. It does not contain viewer identities, comments or messages.
- This is not the older ThaiVtuberSNA review-step wire format. Build an explicit target adapter from the current upstream schema before applying; do not rename a generic proposal to claim compatibility.
