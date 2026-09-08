# Spec: background WHOIS refresh

## Goal

While the TUI is running, a background job refreshes WHOIS expiry for every loaded MP domain that is due for a check, persists each update, and logs progress to the console (log panel).

## Prerequisites (data + UI)

1. **Persist last-checked time on each domain**
  - Extend `data.Domain` with `LastChecked` (`json:"lastChecked"`).
  - Empty / zero value means “never checked” (same idea as empty `Expiry`).
  - Set `LastChecked` to now inside `Domain.UpdateExpiry()` on every successful WHOIS call (manual `Ctrl+U`, add-domain, and the background job).
  - Existing `mp_data.json` entries without the field load as never-checked.
2. **Show last-checked in the domain panel**
  - Update `panel.DrawListDomainPanel` so each row shows hostname, expiry, and last-checked (e.g. `1. example.com.au 2027-01-01  checked 26-03-01` or `never`).
  - Prefer a short date format consistent with the UI (console uses `06-01-02 …`).
  - Note: this is the **domain** panel (`DOMAIN_PANEL` / `listDomainPanel.go`), not `LOGO_PANEL`.



## Background process

**Trigger:** start once after MPs are loaded (e.g. end of `UI.Load` / `InitApp`), as a goroutine. v1 default: **once per session after load**, non-blocking. Optional later: periodic re-run (e.g. daily).

**Disable via env:** if `SKIP_BACKGROUND_WHOIS_LOOKUP=true` (case-insensitive), do **not** start the background refresh. Log a single INFO (or DEBUG) that background WHOIS was skipped because of the env var. Manual **Ctrl+U** is unaffected and still runs WHOIS.

**Scope:** iterate `ui.state.all` (all loaded MPs), not the current filter / `visible` list.

**Per domain:**

1. Count all MP domains, then log start.
2. For each MP → each domain:
  - **Skip** WHOIS if `LastChecked` is set and `now - LastChecked < 10 days` (do not re-check fresh lookups).
  - **Run** WHOIS via `Domain.UpdateExpiry()` if never checked or last check is **≥ 10 days** ago.
  - On success: update in-memory domain (`Expiry` / `Expired` / `LastChecked`) and **save immediately** with `db.WriteMps` (same path as `Ctrl+S` / `UI.Save`).
  - On failure: log an error for that domain; continue (do not abort the run).
3. Log a summary when finished.

**UI thread safety:** WHOIS and disk I/O stay off the gocui main loop. Console updates and domain-panel redraw go through `g.Update(...)` (same pattern as `SearchAndDisplay`).

**Concurrency:** only one refresh run at a time (mutex / “running” flag). Manual `Ctrl+U` may race with background save in v1 (last writer wins); share a lock later if needed.

## Console log messages

Use level prefixes until a real leveled logger exists (`ui.log` today is only error vs not).

These messages apply to **both** the background refresh job and manual **Ctrl+U** (`checkDomain`). Prefer one shared helper so the wording stays identical.


| Level | When                        | Example                                                                                                                    |
| ----- | --------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| INFO  | Start of run                | `INFO checking all (42) MP domains` — total domains in scope for this run                                                  |
| DEBUG | About to WHOIS a due domain | `DEBUG checking Jane Doe MP domain example.com.au`                                                                         |
| INFO  | End of run                  | `INFO checked 12 MPs and updated 18 domains` — MPs with ≥1 lookup attempted; domains that actually ran WHOIS (not skipped) |


**Ctrl+U:** same three lines for the selected domain only — typically `checking all (1) MP domains`, one DEBUG for that MP/hostname, then `checked 1 MPs and updated 1 domains` (or `updated 0` on failure). Ctrl+U does **not** apply the 10-day skip; it always attempts WHOIS.

Skipped (fresh) domains on the background job do not emit DEBUG. Failed lookups still count toward “checked”; optionally add a separate ERROR line with hostname + reason.

## Acceptance criteria

- [ ] `Domain` stores and JSON-persists `lastChecked`
- [ ] Successful WHOIS (any path) updates `lastChecked`
- [ ] Domain panel rows show last-checked (or `never`)
- [ ] Background job walks all MPs/domains, skips if checked within 10 days, WHOIS + save otherwise
- [ ] `SKIP_BACKGROUND_WHOIS_LOOKUP=true` prevents the background job from running (Ctrl+U still works)
- [ ] Log panel shows the INFO / DEBUG / INFO messages above for the background job **and** for Ctrl+U
- [ ] TUI remains usable while the job runs



## Out of scope (v1)

- Rate-limit / backoff beyond the 10-day skip
- Alerting when expiry is soon or a domain disappears
- Changing `LOGO_PANEL`

## Unit test plan

Write and pass tests **before** wiring the background job into the TUI. Order below is the intended implementation / coverage order: WHOIS package first, then domain last-checked behavior, then refresh orchestration.

There are currently no `*_test.go` files in the repo. Prefer table-driven tests. Keep network I/O out of unit tests via seams described under each area.

---

### 1. WHOIS lookup (`whois` package) — required first

**Testability note:** `GetExpiry` today calls `whois.NewClient().Whois` and `whoisparser.Parse` directly. Before meaningful unit tests, introduce a thin seam, e.g.:

- injectable `WhoisFetcher` / `func(hostname string) (raw string, err error)` for the network call, and/or
- export or inject parse + `clean` so success/error paths can be exercised with fixture WHOIS text.

Live WHOIS against the public internet is **out of scope** for unit tests (optional manual / integration later).

| ID | Area | Cases |
|----|------|--------|
| W1 | `clean` | strips leading `www.`; leaves bare host unchanged; does not strip `www` in mid-label (e.g. `wwwexample.com`); empty string stays empty |
| W2 | `GetExpiry` success | given fixture raw WHOIS with a parseable expiration, returns that expiry string and `nil` error; does not return the “Could not get/parse” message strings |
| W3 | `GetExpiry` fetch error | when fetcher returns error, returns error and a message containing `Could not get WHOIS for <hostname>` (hostname as passed in, not necessarily cleaned) |
| W4 | `GetExpiry` parse error | when raw WHOIS is unparseable, returns error and a message containing `Could not parse <hostname>` |
| W5 | `GetExpiry` + clean | hostname `www.example.com` is cleaned before fetch (assert fetcher received `example.com`) |
| W6 | Empty / missing expiry | parse succeeds but expiration date empty → document expected behavior (empty string + nil, or error); lock that in a test |

**Fixtures:** store minimal sample WHOIS responses under `whois/testdata/` (or inline strings) for at least one `.com` / `.com.au`-shaped record with `ExpirationDate` populated, plus one garbage payload for W4.

---

### 2. Domain update + `LastChecked` (`data` package)

Depends on a mockable WHOIS boundary (same seam as §1, or a package-level `GetExpiry` var / interface used by `UpdateExpiry`).

| ID | Area | Cases |
|----|------|--------|
| D1 | Success updates fields | on successful WHOIS, `Expiry` set to returned date; `LastChecked` set to “now” (inject clock or assert within a short window) |
| D2 | Failure leaves prior state | on WHOIS error, existing `Expiry` / `LastChecked` unchanged; error propagated |
| D3 | Never-checked → checked | zero/`""` `LastChecked` becomes non-empty after success |
| D4 | JSON round-trip | marshal/unmarshal `Domain` preserves `lastChecked`; missing JSON field loads as never-checked |
| D5 | Panic recovery | if WHOIS path panics, `UpdateExpiry` recovers without crashing (current `defer recover`); assert safe return / no process abort |

---

### 3. Due / skip throttle (pure logic)

Extract a small helper used by the background job, e.g. `Domain.NeedsWhois(now time.Time, maxAge time.Duration) bool`, so throttle rules are unit-tested without goroutines or disk.

| ID | Area | Cases |
|----|------|--------|
| T1 | Never checked | empty/`zero` `LastChecked` → needs WHOIS (`true`) |
| T2 | Fresh | checked 0–9 days ago → skip (`false`) for 10-day max age |
| T3 | Stale boundary | checked exactly 10 days ago → needs WHOIS (`true`); checked just under 10 days → skip |
| T4 | Far past | checked months ago → needs WHOIS |
| T5 | Ctrl+U path | manual check path does **not** call `NeedsWhois` / always looks up (covered in §5 logging or a thin wrapper test) |

---

### 4. Background refresh orchestration

Target a package-level or `ui`-adjacent function that accepts dependencies (MP slice, WHOIS updater, saver, logger, clock) so tests do not need gocui.

| ID | Area | Cases |
|----|------|--------|
| B1 | Domain count | INFO start uses total domain count across all MPs (including those that will be skipped) |
| B2 | Iterates all MPs | uses full loaded list, not a filtered `visible` subset |
| B3 | Skip fresh | domains within 10 days: no WHOIS call, no DEBUG line, no save for that domain |
| B4 | Update stale / never | due domains: WHOIS called once each; on success saver invoked (at least once per success — “after each lookup”) |
| B5 | Continue on error | one domain fails WHOIS; later domains still processed; run completes with summary |
| B6 | Counters | end INFO: `checked` = MPs with ≥1 lookup attempt; `updated` = domains with successful WHOIS (not skips, not failures) |
| B7 | Empty input | no MPs / no domains: still emits start + end INFO with zeros; no panic |
| B8 | Single-flight | second overlapping run is no-op or rejected while first is in progress (mutex / running flag) |
| B9 | Save payload | saver receives MPs with updated `Expiry` / `LastChecked` for successful domains |
| B10 | Env skip | `SKIP_BACKGROUND_WHOIS_LOOKUP=true` → background start is a no-op (no WHOIS, no domain save); unset/`false` allows run |

---

### 5. Shared logging (background + Ctrl+U)

| ID | Area | Cases |
|----|------|--------|
| L1 | Message shape | start / debug / end strings match the console table (prefix `INFO` / `DEBUG`, wording) |
| L2 | Background logs | one start INFO; DEBUG only for domains actually looked up; one end INFO |
| L3 | Ctrl+U logs | same three-line pattern for one domain: `(1)` in start; one DEBUG; end `checked 1 MPs and updated 1` (or `updated 0` on failure) |
| L4 | No DEBUG on skip | background skip path produces no DEBUG for that hostname |
| L5 | Failure optional ERROR | if implemented, failed domain emits ERROR with hostname; start/end still present |

---

### 6. Domain panel display (lightweight)

| ID | Area | Cases |
|----|------|--------|
| P1 | Shows last-checked | `DrawListDomainPanel` includes formatted last-checked when set |
| P2 | Shows never | empty `LastChecked` renders `never` (or agreed placeholder) |
| P3 | Still shows expiry | existing expiry / `<?>` / `[!]` behavior unchanged when last-checked is added |

---

### Out of scope for this unit plan

- Full gocui keybinding / focus integration tests for Ctrl+U
- Real disk `mp_data.json` races under concurrent UI + background save
- Live WHOIS network calls
- Logo panel

### Done when

1. §1–§2 tests green (WHOIS + `UpdateExpiry` / `LastChecked`).
2. §3–§5 tests green (throttle, refresh orchestration, shared logs).
3. §6 tests green if panel formatting lands with the feature.
4. Spec acceptance criteria can then be verified manually in the TUI.

