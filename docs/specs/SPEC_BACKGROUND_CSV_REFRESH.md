# Spec: hourly background CSV refresh

## Goal

While the TUI is running, a background job can refresh MPs from a configured CSV: fetch the listing page over HTTPS, locate and download the configured file (e.g. `allsenel.csv`), parse it with the configured column map (`senators` / `members` / `custom`), merge into the in-memory database with the same rules as `cmd/mergeDatabases` / `data.MergeMPs`, and log additions / merges to the Console Log. **Do not** write `mp_data.json` automatically — the user saves with **Ctrl+S**.

**Triggers:** **Ctrl+L** (manual) and an optional **hourly** ticker. **Do not** run the job on startup / after load.

## Motivation

Manual CSV download + `csv2json` + `mergeDatabases` keeps the live DB stale between sessions. An in-app refresh (on demand or hourly) detects new members (and bio/party updates) without leaving the TUI, while leaving persistence under user control so a bad fetch cannot silently clobber disk. Skipping startup avoids a surprise network hit and merge before the user is ready.

## Config

Introduce a small config file (v1: JSON next to the app, e.g. `config.json` or `csv_refresh.json` — pick one name and stick to it). The job must **not** hardcode the listing-page URL in Go (today `fetcher.findAphHtml` does).

| Field | Required | Default / example | Meaning |
| ----- | -------- | ----------------- | ------- |
| `csvSourceURL` | yes | APH Address labels / CSV guidelines page | HTTPS URL of the HTML page that links to CSVs |
| `csvFilename` | no | `allsenel.csv` | Substring matched against `<a href>` to find the download link |
| `interval` | no | `1h` | How often the ticker runs (Go duration string) |
| `format` | no | `senators` | Which built-in column map / level to use: `senators`, `members`, or `custom` |
| `columns` | when `format` is `custom` | — | Header map object (same fields as `csv.ColumnMap`); ignored for built-in formats |
| `level` | no | derived from `format` | MP `level` string written on each parsed row; required (or strongly recommended) for `custom` |

### Format → parse mapping

Reuse `csv.ParseReader` (see [IMPORT_CSV.md](../IMPORT_CSV.md)). Built-in formats pick the existing column maps and default levels; `custom` uses the config-supplied map.

| `format` | Column map | Default `level` (if `level` omitted) |
| -------- | ---------- | ------------------------------------ |
| `senators` | `csv.SenatorColumns` | `data.Level.FedSenator` (`Federal Senator`) |
| `members` | `csv.MemberColumns` | `data.Level.FedRep` (`Federal House of Representatives Member`) |
| `custom` | `columns` from config (must be present and usable) | `level` from config, or `""` if omitted (same as today’s `ParseCustomReader`) |

`columns` object keys (JSON), matching `csv.ColumnMap`:

| JSON key | Maps to MP field |
| -------- | ---------------- |
| `honorific` | Honorific |
| `firstName` | FirstName |
| `surname` | Surname |
| `otherName` | OtherName |
| `preferredName` | PreferredName |
| `party` | Party |
| `state` | State |
| `electorate` | Electorate |

Values are the **exact CSV header names** in the downloaded file (same rules as `CustomColumns` / `-custom` in csv2json). Empty string for a key means that field is not mapped (parser behavior matches `csv` package today).

**Validation on load:**

- `format` must be one of `senators` | `members` | `custom` (case-insensitive OK if documented).
- If `format` is `custom`, `columns` is required; reject config if missing or if every header is empty (unusable map).
- If `format` is `senators` or `members`, ignore any `columns` object (or reject it as a config error — pick one; prefer **ignore** with optional DEBUG).
- Unknown `format` → ERROR, job disabled.

Examples:

Senators (default-shaped):

```json
{
  "csvSourceURL": "https://www.aph.gov.au/Senators_and_Members/Contacting_Senators_and_Members/Address_labels_and_CSV_files",
  "csvFilename": "allsenel.csv",
  "format": "senators",
  "interval": "1h"
}
```

House of Reps members:

```json
{
  "csvSourceURL": "https://www.aph.gov.au/Senators_and_Members/Contacting_Senators_and_Members/Address_labels_and_CSV_files",
  "csvFilename": "FamilynameRepsCSV.csv",
  "format": "members",
  "interval": "1h"
}
```

Custom headers + level:

```json
{
  "csvSourceURL": "https://example.org/contacts",
  "csvFilename": "roster.csv",
  "format": "custom",
  "level": "State Senator",
  "columns": {
    "honorific": "Title",
    "firstName": "First Name",
    "surname": "Surname",
    "otherName": "Other Name",
    "preferredName": "Preferred Name",
    "party": "Party",
    "state": "State",
    "electorate": "District"
  },
  "interval": "1h"
}
```

**Load:** once at app start (same window as other init). Missing / invalid config → log ERROR to Console Log; **Ctrl+L** and the hourly ticker are no-ops until config is fixed. Changing config requires restart (v1).

**Disable via env:** if `SKIP_BACKGROUND_CSV_REFRESH=true` (case-insensitive), do **not** start the hourly ticker; log a single INFO that the automatic refresh was skipped. **Ctrl+L** still runs when config is valid (same idea as `SKIP_BACKGROUND_WHOIS_LOOKUP` vs manual WHOIS). Manual CLI workflows (`csv2json`, `mergeDatabases`) are unaffected.

## Triggers

| Trigger | When | Notes |
| ------- | ---- | ----- |
| **Ctrl+L** | User keypress (global binding) | Starts one refresh run off the UI thread. Always allowed when config is valid (honours single-flight). |
| Hourly ticker | Every `interval` (default **1h**) after the ticker is armed | Arm after MPs are loaded if config is valid and env skip is unset. **First fire is after one full interval** — never on startup / immediately after load. |
| Startup / load | — | **Do not** trigger a refresh when the app starts or when MPs finish loading. |

Document **Ctrl+L** in [KEYBOARD_SHORTCUTS.md](../KEYBOARD_SHORTCUTS.md) and the in-app **Ctrl+H** help (global section) when implementing.

## Background process

**Single-flight:** only one refresh run at a time (mutex / “running” flag). If Ctrl+L or a tick fires while a run is in progress, skip and log INFO (e.g. `INFO CSV refresh: already running`).

**UI thread safety:** HTTP, parse, and merge stay off the gocui main loop. Console updates and any MP-list redraw go through `g.Update(...)` (same pattern as background WHOIS).

### Pipeline (stop on hard failure; always log)

1. **HTTPS GET** `csvSourceURL`.
   - Non-2xx, dial/TLS error, or empty body → ERROR to Console Log; abort run (no parse/merge).
2. **Parse HTML**; find an `<a href>` whose value **contains** `csvFilename` (reuse / generalize `fetcher.findAnchorWithPartialHref`).
   - Not found → ERROR; abort.
   - Resolve relative hrefs against the page origin (today: prefix `https://www.aph.gov.au` when href is path-absolute). Prefer proper URL join so the config host stays authoritative.
3. **HTTPS GET** the CSV URL; read body into memory (or temp; v1 need not persist the file on disk).
   - Failure → ERROR; abort.
4. **Parse CSV** with `csv.ParseReader` using the column map and level selected from config (`senators` → `SenatorColumns` + FedSenator; `members` → `MemberColumns` + FedRep; `custom` → config `columns` + config `level`). See [IMPORT_CSV.md](../IMPORT_CSV.md).
   - Header / row errors → ERROR (or WARN per-row if the parser grows that mode); abort merge if the parse returns a fatal error. Do not leave a half-applied merge.
5. **Merge** into the loaded in-memory list:
   - `A` = current `ui.state.all` (or equivalent live slice).
   - `B` = MPs freshly parsed from CSV (treated as **newer** for party/bio, same as `-b` in [SPEC_MERGE_DATABASES.md](SPEC_MERGE_DATABASES.md)).
   - Call `data.MergeMPs(A, B)` (shared with `cmd/mergeDatabases` — do not fork merge rules).
6. **Apply in memory only:** replace the live MP slice with `MergeResult.MPs`. Refresh visible list / panels as needed so the UI shows new members without a reload from disk.
7. **Do not call** `db.WriteMps` / `UI.Save`. Log that changes are unsaved if any add/merge occurred (see below).

If step 3 fails, steps 4–7 do not run. If CSV downloads but parse fails, do not merge.

## Detecting “new” vs “merged”

`MergeResult` today exposes `MergedNames` only. For console alerts, also determine **added** members: names present in the post-merge result (or in `B`) that were **not** in `A` before the run (same name key as merge: `strings.EqualFold(strings.TrimSpace(FirstName + " " + Surname))`).

Preferred approach: extend `MergeResult` (e.g. `AddedNames []string`) inside `data.MergeMPs` so CLI and TUI share one definition; or compute added names in the job from before/after name sets. Either is fine for v1 as long as logging is accurate.

| Event | Meaning |
| ----- | ------- |
| **Added** | Name appeared only after merge (new member from CSV) |
| **Merged** | Name existed in both A and B (or collapsed); party/bio from CSV; domains unioned / checks cleared per merge spec |

Members only in A (not in CSV) stay untouched — v1 does **not** remove absent members (no “dropped from list” pruning).

## Console log messages

Use level prefixes consistent with background WHOIS / merge CLI (`INFO` / `DEBUG` / `ERROR`).

| Level | When | Example |
| ----- | ---- | ------- |
| INFO | Automatic ticker skipped (env) | `INFO background CSV refresh ticker skipped (SKIP_BACKGROUND_CSV_REFRESH)` |
| INFO | Start of run | `INFO CSV refresh: fetching listing page` |
| DEBUG | Link resolved | `DEBUG CSV refresh: download URL …/allsenel.csv` |
| INFO | Download OK | `INFO CSV refresh: downloaded allsenel.csv (N bytes)` |
| ERROR | Fetch / link / download failure | `ERROR CSV refresh: …` |
| ERROR | Parse failure | `ERROR CSV refresh: parse: Expected column …` (or parser message) |
| INFO | Parse OK | `INFO CSV refresh: parsed N members from CSV (format=senators)` |
| INFO | Each new member | `INFO CSV refresh: added name="…"` |
| INFO | Each merged member | `INFO CSV refresh: merged name="…"` |
| INFO | End, no changes | `INFO CSV refresh: no new or merged members` |
| INFO | End, with changes | `INFO CSV refresh: added A, merged M (unsaved — Ctrl+S to persist)` |
| INFO | Config missing | `ERROR CSV refresh: config …` then job not started |

Do not spam per-row DEBUG on happy path. Failed runs still emit a clear ERROR; do not claim adds/merges if merge did not run.

## Relationship to existing code

| Piece | Role |
| ----- | ---- |
| `fetcher` | HTML GET + anchor-by-filename; refactor to accept page URL from config and injectable HTTP client for tests |
| `csv.ParseReader` / `ColumnMap` | Parse downloaded body with senators, members, or config-supplied custom columns + level |
| `data.MergeMPs` | Same merge semantics as [SPEC_MERGE_DATABASES.md](SPEC_MERGE_DATABASES.md) |
| `ui/handlers.go` | Global **Ctrl+L** → kick one refresh run (no startup kick) |
| Console Log / `ui.log` | Surface INFO/ERROR (via `g.Update` when GUI is live) |
| Ctrl+S / [SPEC_JSON_SAVE.md](SPEC_JSON_SAVE.md) | User persists after review |

Reuse `downloadCsv` in `csv/read.go` or move HTTP download next to the fetcher so errors go to the log callback instead of `fmt.Println`.

## Out of scope (v1)

- Refreshing multiple CSVs in one run (one source URL + one filename + one format per config)
- Auto-save or writing a side JSON like `mps_from_csv.json`
- Removing MPs missing from the CSV
- Territory / state member *sources* beyond what `format=custom` + `level` already allow
- Changing merge name-key rules
- Full gocui integration tests

## Acceptance criteria

- [ ] Config supplies listing-page URL, optional filename / interval, and `format` (`senators` | `members` | `custom`); `custom` requires `columns`; missing / invalid config makes Ctrl+L and the ticker no-ops (with ERROR on load / on Ctrl+L)
- [ ] Parse uses the configured column map and level (built-in senators/members or custom headers)
- [ ] **Ctrl+L** starts one refresh run; documented in keyboard shortcuts / Ctrl+H help
- [ ] Job does **not** run on startup or immediately after MP load
- [ ] Hourly ticker (default 1h) may run after one full interval; `SKIP_BACKGROUND_CSV_REFRESH=true` disables the ticker only (Ctrl+L still works)
- [ ] Single-flight, off the UI thread
- [ ] HTTPS fetch of listing page → find href containing configured filename → download CSV
- [ ] Fetch / link / download / parse errors appear in Console Log; merge only runs after a successful parse
- [ ] Successful parse merges into in-memory MPs via `data.MergeMPs` (CSV side is newer)
- [ ] Console Log reports each added and each merged member, plus an unsaved reminder when there were changes
- [ ] No automatic `mp_data.json` write; Ctrl+S persists
- [ ] TUI remains usable while the job runs

## Unit test plan

Prefer table-driven tests; keep live APH / network I/O out of unit tests via injectable HTTP (round-tripper or `Get(url) (body, err)` seams).

| ID | Area | Cases |
| ---- | ---- | ----- |
| C1 | Config | valid file loads URL/filename/interval/format; missing file / bad JSON / empty URL / unknown format → error |
| C3 | Format senators/members | `format=senators` → SenatorColumns + FedSenator; `format=members` → MemberColumns + FedRep |
| C4 | Format custom | `columns` required; maps headers into ParseReader; `level` applied to rows; missing/empty columns → config error |
| C2 | Env skip | `SKIP_BACKGROUND_CSV_REFRESH=true` → hourly ticker not armed; Ctrl+L still can run |
| T1 | No startup run | after load / InitApp, refresh orchestration is not invoked |
| T2 | Ctrl+L | key handler kicks one run (or shared `TryRun`); overlapping press → already-running INFO |
| F1 | Link find | fixture HTML with configured filename in href → resolved absolute URL |
| F2 | Link missing | fixture HTML without filename → error |
| F3 | Relative href | path-only href joins with page origin |
| D1 | Download error | non-2xx / transport error → ERROR path, no parse |
| P1 | Parse senators | fixture senators CSV + format senators → expected MP count / sample fields |
| P1b | Parse members | fixture members CSV + format members → expected fields / FedRep level |
| P1c | Parse custom | fixture CSV + custom columns/level → mapped fields and level |
| P2 | Parse error | missing required column → error, no merge |
| M1 | Merge apply | in-memory A + parsed B → result matches `MergeMPs`; Added / Merged logging sets correct |
| M2 | No auto-save | orchestration never calls Save/WriteMPs even when adds/merges > 0 |
| M3 | Unchanged | identical roster → end INFO with no added/merged; memory still consistent |
| B1 | Single-flight | overlapping run rejected / skipped |
| L1 | Log shape | start / error / added / merged / unsaved strings match the table above |

### Done when

1. Config + fetch/parse seams tested (C*, F*, D*, P*).
2. Merge + no-save + logging tested (M*, L*, B1).
3. Manual TUI check: Ctrl+L (and optionally shortened interval after waiting), see Console Log, confirm no run at startup and disk unchanged until Ctrl+S.
