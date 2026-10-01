# Tipme public source review — 2026-10-01

Finder supports explicit `https://tipme.in.th/<slug>` creator links through bio extraction and the existing JSONL import flow. Tipme slugs are mutable handles, not stable platform IDs. A donation page does not establish a persona link.

## Discovery and evidence

The owner's public YouTube channel-link census (`yt_channel_links.jsonl`, last seen 2026-10-01) contains 100 distinct Tipme URLs attached to 100 stable YouTube channel IDs. Only Tipme URLs and those stable channel URLs were selected. No display-name matching was used. The runtime import records preserve `https://www.youtube.com/channel/<UC…>` as inline `source_url` and remain human-review candidates. The collected import is stored outside Git in ignored `data/tipme-leads.jsonl`.

Examples of explicit public channel evidence:

| Tipme link | Stable first-party channel evidence |
| --- | --- |
| https://tipme.in.th/ziika | https://www.youtube.com/channel/UC0D7SuIVLYoD1hU3dxNxdMg |
| https://tipme.in.th/bianca | https://www.youtube.com/channel/UC3TQ2myWZR8pZiUZ6Cs0-Sw |
| https://tipme.in.th/amuvch | https://www.youtube.com/channel/UC4zIwNmfURhIO5Pj-ccmKBA |

These are channel-description links from the census, not a claim that the Tipme pages are still live. Creator pages could not be read by the public web tool, and signed-out Chrome was unavailable during the review. Public X search provided no first-party evidence suitable for importing additional links. The [public homepage](https://tipme.in.th/) showed no discover directory; this review does not claim that none exists.

## Aggregate totals and visibility controls

| Question | Finding |
| --- | --- |
| Do signed-out creator pages expose donation totals? | Unknown. No successfully inspected signed-out creator page established a total, time window, or currency. No aggregate data was collected. |
| Does the creator have donation statistics? | The [official support article](https://support.tipme.in.th/th/article/4lib4liy4lij4lii4li34lmi4liz4lig4liy4lip4li14lma4lih4li04liz4lme4liu4lmj4liq4liz4lir4lij4lix4lia4liq4liv4lij4li14lih4lma4lih4lit4lij4lmm-1cpyovl/) describes the statistics page for the creator. This is not evidence of public visibility. |
| Can the creator show or hide public aggregate totals? | Unknown. No public documentation found establishes a totals visibility toggle. |
| What public-page customization is documented? | [Official BBCode documentation](https://support.tipme.in.th/th/article/bbcode-3cje7k/) describes configurable donation-page text and success/failure messages. That is not a documented totals toggle. |

No authenticated creator dashboard, internal API, payment record, supporter identity or message was accessed. Finder stores the public account URL and provenance only. This work does not write FINANCE.

## Verification

Regression tests cover Tipme URL normalization, reserved non-profile routes, bio extraction and JSONL preservation of stable channel evidence. The URL and bio tests failed before support was added and passed afterwards. The full native SQLite/race checks require Linux CI: local Windows has no CGO compiler, and the installed WSL distribution references a missing disk.

The runtime import must first run with Sheet sync disabled, then use the existing Finder writer for FINDER_INBOX, followed by `finder verify` with `canonical_invariant_ok: true` and `duplicate_inbox_rows: 0`. Inbox write and production verification counts will be recorded after that flow succeeds; discovery counts alone are not an inbox write claim.
