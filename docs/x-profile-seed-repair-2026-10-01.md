# X-profile website seed repair — 2026-10-01

The original source stopped after 29 rows because row 30 is a Bitly URL unsupported by the existing normalizer. All 454 original records are preserved: 437 unchanged rows remain enabled, and 17 unchanged rows are quarantined in an unresolved file that is not enabled.

Chrome was used only to open supplied URLs and observe navigation. No destination buttons, logins or forms were used. Only destination URLs and review outcomes are recorded; no profile data or follower lists are retained. Discord invites were not opened.

## All 17 quarantined rows

| Original row | X handle | Original URL | Observed destination | Result | Reason |
| ---: | --- | --- | --- | --- | --- |
| 30 | anyaplg | http://bit.ly/3j9Jtw5 | https://www.youtube.com/@AnyaPLG/featured | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 52 | baabel_arp | https://bit.ly/3l1H5sp | https://www.youtube.com/@Baabel_ARP/?sub_confirmation=1 | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 54 | baku_arp | https://bit.ly/3svbQoX | https://www.youtube.com/channel/UCO6R8Pc5g2R7ObJQPBGppdg?sub_confirmation=1 | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 99 | coolrinc | https://www.youtube.com/coolrinch | https://www.youtube.com/coolrinch | unresolved | Legacy YouTube path remains in the address bar; no supported destination URL was observed. No handle or ID was inferred. |
| 105 | dacapo_arp | https://bit.ly/3T7M7QD | https://www.youtube.com/@Dacapo_ARP | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 138 | fumihausu | https://bit.ly/3MwgQUw | https://www.youtube.com/c/fumihausu/featured | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 166 | hokuplg | http://bit.ly/44vfNvi | https://www.youtube.com/@HokuPLG | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 240 | lynna_vtuber | https://discord.gg/rP2hUggW2v | Not opened | unresolved | Discord invite retained without opening, as instructed. |
| 322 | paruismeparu | https://www.youtube.com/watch?v=AOCqJpg6g-UwKtFartJ5FmkAcY1UclI3eBj7SMBv6 | https://www.youtube.com/watch?v=AOCqJpg6g-U | dead | YouTube reports the video is no longer available. |
| 339 | red_fartear | http://youtube.com/RedFartearCh | https://www.youtube.com/RedFartearCh | unresolved | Legacy YouTube path remains in the address bar; no supported destination URL was observed. No handle or ID was inferred. |
| 343 | rexaplg | https://bit.ly/3wCs2Yd | https://www.youtube.com/@RexaPLG/featured | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 348 | rionaplg | https://bit.ly/3kHfSKN | https://www.youtube.com/@RionaPLG/featured | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 365 | schneider_arp | https://bit.ly/3LeLIK3 | https://www.youtube.com/@schneider_arp | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 369 | selene_arp | https://bit.ly/3rB6uJA | https://www.youtube.com/channel/UChBuxXl8poN1Jz8kZDdsfhw | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 396 | tamamiplg | https://bit.ly/3Hc43nx | https://www.youtube.com/@TamamiPLG/featured | seeded | Automatic Bitly redirect; supported YouTube account URL. |
| 399 | thequillmon1112 | https://www.youtube.com/TheQuillmon | https://www.youtube.com/TheQuillmon | unresolved | Legacy YouTube path remains in the address bar; no supported destination URL was observed. No handle or ID was inferred. |
| 445 | yueqinvtuber | https://discord.gg/6ATeDFaqt7 | Not opened | unresolved | Discord invite retained without opening, as instructed. |

## Counts and activation

- Original records: 454.
- Enabled original source: 437 records.
- Disabled unresolved archive: 17 records (11 Bitly, 2 Discord, 4 unsupported YouTube paths). No Facebook records occur in these 17, correcting the preliminary host tally.
- Enabled browser-resolved source: 11 records; provenance and other fields are preserved and description includes the requested resolution suffix.
- Outcomes: seeded 11, dead 1, unresolved 5, bio_link 0, website 0.
- Expected active observations per cycle: 437 + 11 = 448, before identity deduplication. These are candidate observations, not verified personas.

## Validation and production handoff

CGO-enabled Ubuntu CI runs race tests, vet, build and demo. TestXProfileWebsiteSeedsParseCompletely reads both enabled files through the unchanged parser, checks exact lengths and prevents enabling the quarantine. Production execution and downstream sync occur after passing checks and the approved merge; observed runtime counts are reported separately.

## Separate parser proposal (not implemented)

Consider a separate change that reports invalid JSONL rows and retains subsequent valid rows, similar to the JSON adapter, with explicit partial-source reporting and tests. This repair changes neither normalizer nor parser behavior and never promotes canonical personas/accounts/links.
