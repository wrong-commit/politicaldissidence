# Spec: mergeDatabases CLI

## Goal

Merge two MP JSON databases into a **new** output file, deduplicating members by name. Never overwrite an existing file. When two records share a name, keep the later member’s party/biographical details, union their domains, and clear WHOIS / HTTPS (HTTP status) / DNS check state on merged domains so background jobs re-probe.

## CLI

Package: `cmd/mergeDatabases`

```text
go run ./cmd/mergeDatabases -a <dbA.json> -b <dbB.json> [-o <out.json>]
```

| Flag | Required | Default | Meaning |
| ---- | -------- | ------- | ------- |
| `-a` | yes | — | First input MP JSON (validated like load) |
| `-b` | yes | — | Second input MP JSON (treated as **newer** for party details) |
| `-o` | no | `mps_merged.json` | Output path |

Exit codes: `2` usage / missing flags; `1` validation, merge, or write failure; `0` success.

## No overwrite

Before writing, if `-o` already exists (any file), abort with an error and leave disk unchanged. Do **not** call `db.WriteMpsTo` / replace when the destination exists. Inputs are read-only.

Default `-o` is `mps_merged.json` (not `mp_data.json`) so a careless run does not target the live TUI DB; if that default path already exists, the run still fails until the user picks another `-o`.

## Dedup key

Members match when `strings.EqualFold(strings.TrimSpace(mp.Name()), …)` — i.e. `FirstName` + `" "` + `Surname` (`data.MP.Name()`), case-insensitive, trimmed.

Same name inside one file, or across A then B, collapses to one record.

## Merge algorithm

1. Validate and load A, then B via `db.ValidateMPJSONFile` (same structural rules as [SPEC_JSON_VALIDATION.md](SPEC_JSON_VALIDATION.md)).
2. Walk A in order, then B in order.
3. For each MP:
   - If name not yet seen → append a copy to the result.
   - If name already seen → **merge into** the existing result entry (see below).
4. Write the result with `db.WriteMpsTo` only after the no-overwrite check passes.

### Party / biographical fields (newer wins)

When merging B into an existing record from A (or a later duplicate within the same file), copy these fields from the **incoming** (later) MP onto the kept record:

- `honorific`, `firstName`, `surnname`, `otherName`, `preferredName`
- `electorate`, `party`, `state`, `level`

Domains are handled separately (union), not replaced wholesale.

### Domains

1. Concatenate domain lists (existing then incoming).
2. Deduplicate by hostname with `strings.EqualFold(strings.TrimSpace(hostname), …)`; keep the **first** spelling of the hostname.
3. For **every** domain on a member that participated in a name-merge (two+ source records collapsed), emit domains with checks cleared:

| Field | Cleared to |
| ----- | ---------- |
| `expiry` | `""` |
| `expired` | `false` |
| `alert` | `false` |
| `lastChecked` | zero / omitted |
| `whois` | omitted (`nil`) |
| `dns` | omitted (`nil`) |
| `https` | omitted (`nil`) — includes aggregated HTTP status |

A member that appears only once (no name collision) keeps domains and check state unchanged.

If the same hostname appears twice on a single unmerged member, still dedupe hostnames but **do not** clear checks unless that member was name-merged with another record. (v1: optional tidy — prefer clearing only on name-merge; hostname-only dupes within one record: keep first domain’s check blob, drop later dupes.)

## Logging

Stderr, same INFO/ERROR style as `csv2json` / `checkdomains`:

- `INFO mergeDatabases: loaded N from <path>`
- `INFO mergeDatabases: merged name="…" (party details from later record; domains cleared)`
- `INFO mergeDatabases: wrote N MPs to <path>`
- `ERROR mergeDatabases: …` on failure

## Out of scope (v1)

- Three-or-more input files
- Matching on preferred name / honorific alone
- Preserving any check state when members merge
- In-place update of `mp_data.json`
- Interactive conflict resolution

## Acceptance criteria

- [x] `-a` and `-b` required; both validated as MP JSON arrays
- [x] Output refused if `-o` already exists
- [x] Same name (case-insensitive) collapses; later party/bio fields win
- [x] Domains unioned by hostname; checks cleared when a name-merge occurred
- [x] Unique-only members keep domains and WHOIS/DNS/HTTPS intact
- [x] Unit tests cover dedupe, party overwrite, domain union + clear, and no-overwrite
- [x] README documents build/run examples (see project `README.md`)
