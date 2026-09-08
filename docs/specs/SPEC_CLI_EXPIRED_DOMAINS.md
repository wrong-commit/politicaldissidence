# Spec: CLI batch domain watchlist

## Goal

Introduce a single persisted boolean on each domain:

```go
Alert bool `json:"alert"`
```

`Alert == true` means **a human should review this domain for sniping** (buy / drop / abandon candidate).

**Both** the TUI and a new CLI batch tool must set / clear `Alert` from the same rule whenever WHOIS, DNS, or HTTPS results are updated (background scan, **`u`**, domain-added, and the CLI pass). The CLI additionally **prints** every domain where `alert` is true after its run.

HTTPS probe behaviour and panel layout: [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md).

This closes the DONTREADME TODO *“Add script for running WHOIS checks against all MP domains and outputting expired domains”* and the related out-of-scope note in [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md) (*CLI batch script for empty+expired domains*).

## Motivation

| Need | TUI today | Gap |
|------|-----------|-----|
| Fresh WHOIS / DNS | Background + **`u`** | Results are stored, but nothing stamps a sniping watchlist flag |
| Empty DNS / near expiry | Markers + expiry text | No single `alert` boolean for “review for sniping” |
| One-shot report of all hits | Interactive only | No CLI that force-checks everything and lists `alert` domains |

One boolean keeps JSON and UI simple: anything that needs human sniping attention is `alert: true`. Detailed *why* can still appear on the CLI report; the stored field is only the boolean.

## Definition: when `Alert` is true

After checks for a domain complete, compute sniping reasons, then:

```text
Alert = (expired OR soon OR dns-empty OR https-expired OR https-missing)
```

| Reason | Condition |
|--------|-----------|
| **expired** | Parsed expiry date is **strictly before** “now” (start of local day is fine; document choice in code) |
| **soon** | Parsed expiry is **on or before** `now + soonWindow` (default **90 days**) and not already past (past → **expired**) |
| **dns-empty** | Latest DNS outcome is empty / nxdomain per [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md) (`dns.Empty == true`) |
| **https-expired** | Latest **aggregated** HTTPS status is `expired` per [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md) (best of apex + www) |
| **https-missing** | Latest **aggregated** HTTPS status is `missing` per [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md) (both apex and www missing) |

Notes:

- Domains with **unparseable / missing expiry** do **not** get expiry-based alert bits. Optionally stderr WARNING; do not invent “expired”.
- Hard WHOIS or DNS **errors** do not set alert by themselves. Log ERROR and continue; do not abort the run. Prior `Alert` is **recomputed** from whatever expiry/DNS/HTTPS state is available after this pass (error on DNS → treat as not dns-empty for this classification).
- HTTPS probes **both** apex and `www.`; alert uses the **aggregated** status. Dual-host `missing` (including dial/timeout on both) contributes **https-missing**; never-probed (`https` nil) does not. Either host `enabled` clears HTTPS alert bits.
- **Always set** `Alert` to `true` or `false` after classification — clear it when no reasons apply so a domain that recovered (renewed, DNS filled, cert fixed) drops off the watchlist on the next check (TUI or CLI).
- Do **not** rely on `Domain.Expired` for alert — compute from the expiry string. Optionally sync `Expired` when cheap (see §6).
- A domain may match **multiple** reasons; `Alert` stays a single bool. Reasons are for the CLI stdout report / logs only, not persisted as a separate field in v1.

Default **soon window:** `90 * 24 * time.Hour`. Use the same constant for TUI and CLI (e.g. `data.AlertSoonWindow`); CLI may override via `-soon-days`.

### Persist on `data.Domain`

```go
type Domain struct {
    Hostname    string
    Expiry      string
    Expired     bool
    Alert       bool      `json:"alert"` // true → human should review for sniping
    LastChecked time.Time
    Whois       *WhoisRecord
    DNS         *DnsRecord
    Https       *HttpsRecord // see SPEC_HTTPS_CERT.md
}
```

JSON:

```json
{
  "hostname": "example.com.au",
  "expiry": "2026-10-01",
  "expired": false,
  "alert": true,
  "lastChecked": "...",
  "whois": { },
  "dns": { "empty": true, "outcome": "empty" }
}
```

- Missing `alert` in old `mp_data.json` loads as `false` (Go zero value).
- Prefer **always writing** `alert` on save (no `omitempty`) so `false` is explicit after a check pass. If that churns the whole file awkwardly, `omitempty` is acceptable for v1 as long as `true` is persisted and cleared domains become absent/`false` on next write.

Put the classifier next to domain data (e.g. `Domain.RefreshAlert(now, soonWindow)` or `data.ComputeAlert(d, now, soon) bool`) so **CLI and TUI share one rule**.

## Desired behaviour

### 1. TUI: refresh `Alert` after WHOIS, DNS, or HTTPS updates

Whenever a domain’s WHOIS, DNS, and/or HTTPS state is updated in the TUI, recompute `Alert` from the **current** `Expiry` + `DNS` + `Https` (and clear it when the sniping signals are gone).

| Path | When to call `RefreshAlert` |
|------|-----------------------------|
| Domains panel **`u`** (`checkDomain`) | After each forced WHOIS / DNS / HTTPS update for that domain (same action runs all three) |
| Background WHOIS (`whoisRunner` / `refresh`) | After each WHOIS update (recompute using new expiry + existing DNS/HTTPS) |
| Background DNS (`dnsRunner` / `dnsrefresh`) | After each DNS update (recompute using existing expiry/HTTPS + new DNS) |
| Background HTTPS (`httpsRunner` / `httpsrefresh`) | After each HTTPS update (recompute using existing expiry/DNS + new HTTPS) — see [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md) |
| Domain-added jobs (`NewWhoisOnAdd` / `NewDnsOnAdd` / `NewHttpsOnAdd`) | After each job’s update on that domain |

Rules:

- Call the **same** `RefreshAlert` / `ComputeAlert` as the CLI, with the shared default soon window (`data.AlertSoonWindow`, 90 days) unless tests inject another value.
- WHOIS, DNS, and HTTPS runners stay independent ([SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md), [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md)): after a WHOIS-only pass, alert may flip from expiry alone; after a DNS-only pass, from emptiness alone; after an HTTPS-only pass, from cert status alone; no runner must wait for the others.
- Persist `alert` with the normal save for that path (end-of-background-scan `WriteMps`, domain-added save, or the save used by **`u`**). Do not add a separate alert-only writer.
- Lookup **errors** still trigger recompute (so a prior true alert can clear if the remaining good data no longer qualifies — or stay true if expiry/DNS/HTTPS still qualify). WHOIS/DNS errors alone never force `alert` true; HTTPS stored as `missing` does contribute **https-missing**.

### 1b. TUI domain list: show `[x]` when `Alert`

In `DrawListDomainPanel` (Member Domains / `DOMAIN_PANEL`), mirror the existing `Expired` → `[!]` marker:

| Field | Marker | When |
|-------|--------|------|
| `Expired` | `[!]` | already today — append after the expiry text |
| `Alert` | `[x]` | **new** — append when `domain.Alert` is true |

Suggested row shapes:

```text
1. example.com.au 2027-01-01  checked 26-03-01  dns:ok
1. soon.example 2026-10-01[x]  checked 26-03-01  dns:ok
1. empty.example 2027-01-01[x]  checked 26-03-01  dns:empty
1. gone.example 2025-01-01[!][x]  checked 26-03-01  dns:ok
```

Rules:

- Append `[x]` on the **expiry token** the same way `[!]` is appended today (after `Expiry` or `<?>`).
- If both `Expired` and `Alert` are true, show **both** markers: `[!][x]` (order: `[!]` then `[x]`).
- Do **not** show `[x]` when `Alert` is false.
- Redraw the domain list (and Domain Information panel if it already refreshes on the same path) whenever a check updates `Alert`, so **`u`** / background / domain-added flips are visible without changing MP selection.
- Domain Information panel text for `alert` is optional in v1; the list `[x]` is required.

### 2. CLI entry point

Add a small Go program under `cmd/`, e.g.:

```text
cmd/checkdomains/main.go
```

Invoke from the project root (same machine / Go path as [AGENTS.md](../AGENTS.md) / DONTREADME):

```powershell
go run ./cmd/checkdomains
```

Or after build:

```powershell
go build -o checkdomains.exe ./cmd/checkdomains
.\checkdomains.exe
```

Do **not** change the default `main` / TUI entry (`go run .` / `politicaldissidence.exe`) in v1. Keep the batch tool as its own `main`.

Working directory: expect `mp_data.json` beside the process cwd (same as `db` package today). Optional `--data path` later; **out of scope for v1** unless already trivial via existing helpers.

### 3. Flags (v1)

| Flag | Default | Meaning |
|------|---------|---------|
| `-soon-days` | `90` | Days ahead that count as “soon” for alert (overrides `data.AlertSoonWindow` for this run only) |
| `-delay` | `1s` | Sleep between domain check triples (WHOIS+DNS+HTTPS), matching background refresh politeness |
| `-dry-run` | `false` | Load JSON, classify from **persisted** `Expiry` / `dns` / `https`, compute what `alert` *would* be; **no** network; **no** save; still print would-be alerts |
| `-force` | live always force | Live mode always checks every domain (ignore 10-day throttle) |
| `-save` | `true` | After live run, write WHOIS/DNS/HTTPS/`alert` via `db.WriteMps` once at end if anything changed. `-save=false` prints report but leaves disk unchanged |

**Decided defaults:** live checks, 1s delay, save once, soon = 90 days.

Stdout is the **report** (domains with `alert` true). Progress / errors go to **stderr**.

### 4. CLI run algorithm

```text
1. Load MPs via db (validated read preferred; fail fast with clear stderr if invalid JSON).
2. Count total domains; log INFO to stderr: checking N domains.
3. For each MP → each domain (MP order, then domain order):
   a. Unless -dry-run:
        - Run Domain.UpdateExpiryInfo()
        - Run Domain.UpdateDns()
        - Run Domain.UpdateHttps()
        - Sleep -delay (skip after last)
   b. domain.RefreshAlert(now, soonWindow)   // shared with TUI (includes HTTPS reasons)
   c. If domain.Alert: append to report rows (include reasons for humans).
4. If live && -save && any mutation (whois/dns/https/alert): db.WriteMps once.
5. Print report to stdout (alert == true only).
6. Exit 0 always in v1.
```

Reuse existing packages — do **not** reimplement WHOIS/DNS/HTTPS:

- `db` load/save
- `data.Domain.UpdateExpiryInfo` / `UpdateDns` / `UpdateHttps`
- Shared `RefreshAlert` / `ComputeAlert` on `data` (used by TUI paths in §1)

Prefer `cmd/checkdomains` + small helpers; do not import `ui`.

### 5. Output format

Print domains where `Alert` is true (or would be, in dry-run):

```text
example.com.au	mp=Jane Doe	alert=true	expiry=2026-10-01	reasons=soon,dns-empty	dns=empty	https=enabled
expired.example	mp=John Smith	alert=true	expiry=2025-01-01	reasons=expired	dns=ok	https=enabled
nx.example.au	mp=Alex Lee	alert=true	expiry=2027-06-01	reasons=dns-empty	dns=nxdomain	https=missing
cert.example.au	mp=Sam Park	alert=true	expiry=2028-01-01	reasons=https-expired	dns=ok	https=expired
```

Rules:

- Prefer `key=value` tokens (tab-separated groups OK).
- Optional `#` header; summary on stderr or as `# summary` on stdout:

```text
# checked=42 alert=4 expired=1 soon=2 dns-empty=2 https-expired=1 https-missing=1 errors=1
```

(`alert` = domains with `Alert` true; reason counts may exceed that.)

No hits: summary only (or `# none`). Do not invent sample domains.

### 6. Expiry parsing

WHOIS expiry strings are not one layout. v1:

- Try layouts already produced by `whois` (prefer ISO `2006-01-02`).
- Empty / unparseable → no expired/soon contribution to `Alert`.
- Compare calendar dates in **local** time unless the value includes a timezone.

Optional stretch: on successful WHOIS update, set `Domain.Expired = expiry.Before(now)`. If that is more than a small change in `UpdateExpiryInfo`, leave the existing TODO and keep alert classification self-contained.

### 7. Logging (stderr)

| Level | Example |
|-------|---------|
| INFO | `INFO checkdomains: checking all (42) MP domains (soon-days=90)` |
| DEBUG | `DEBUG checkdomains: Jane Doe example.com.au alert=true` (gate behind `-v`) |
| ERROR | `ERROR checkdomains: example.com.au whois: i/o timeout` |
| INFO | `INFO checkdomains: saved mp_data.json` / `skip save` |
| INFO | `INFO checkdomains: done alert=3 errors=1` |

Quiet by default: INFO + ERROR. `-v` enables DEBUG.

## Architecture sketch

```text
  TUI: u / whoisRunner / dnsRunner / domain-added
  CLI: go run ./cmd/checkdomains
            │
            v
     UpdateExpiryInfo and/or UpdateDns
            │
            v
     data.RefreshAlert (shared) ──► Domain.Alert
            │
            ├─► TUI: persist via existing WriteMps paths
            ├─► CLI stdout: lines where Alert == true
            └─► CLI stderr: progress / errors
```

## Acceptance criteria

- [ ] `data.Domain` has `Alert bool` persisted as `alert` in `mp_data.json`
- [ ] Shared `RefreshAlert` / `ComputeAlert` sets `Alert` true iff expired, soon (default 90d), dns-empty, https-expired, or https-missing; false otherwise
- [ ] TUI **`u`** recomputes `Alert` after WHOIS+DNS for the selected domain
- [ ] Background WHOIS runner recomputes `Alert` after each domain WHOIS update
- [ ] Background DNS runner recomputes `Alert` after each domain DNS update
- [ ] Domain-added WHOIS/DNS jobs recompute `Alert` after their updates
- [ ] Domain list appends `[x]` after expiry when `Alert` is true (same place as `[!]` for `Expired`)
- [ ] Both markers can appear together as `[!][x]` when `Expired` and `Alert` are both true
- [ ] `cmd/checkdomains` runs from CLI without starting the TUI
- [ ] CLI walks **all** MP domains; live mode runs WHOIS **and** DNS with configurable delay
- [ ] After each domain in CLI, `Alert` is updated (set or cleared)
- [ ] Stdout lists domains with `alert` true (reasons informational only)
- [ ] Network errors logged; run continues; errors alone do not force `alert` true
- [ ] `-dry-run` classifies without network/save
- [ ] Live `-save` (default true) persists WHOIS, DNS, and `alert` once at end
- [ ] Report on stdout; progress/errors on stderr
- [ ] Unit tests for alert true/false cases without live network
- [ ] Linked from DONTREADME; TODO points at this spec

## Out of scope (v1)

- Extra Domain Information panel copy for `alert` beyond the domain-list `[x]` marker
- Starting or controlling the TUI from the CLI command
- Email / desktop notifications
- Persisting reason strings on the domain (only the boolean)
- JSON/CSV report formats beyond the text lines above
- Non-zero exit code when `alert` domains exist
- Custom `--data` path (unless trivial)
- Treating parking nameservers as empty (same as DNS spec)

## Unit test plan

Inject clock + sample `Domain` snapshots; no network.

| ID | Case |
|----|------|
| A1 | Expiry yesterday → `Alert=true` (reason expired) |
| A2 | Expiry in 30 days, soon-days=90 → `Alert=true` (soon) |
| A3 | Expiry in 120 days, soon-days=90, dns ok → `Alert=false` |
| A4 | Far expiry + `dns.Empty` → `Alert=true` |
| A5 | Empty / unparseable expiry, dns ok → `Alert=false` |
| A6 | Soon + empty → `Alert=true` (both reasons in classify output) |
| A7 | DNS error record, far expiry → `Alert=false` |
| A8 | Previously `Alert=true`, now far expiry + dns ok → `Alert=false` (cleared) |
| A9 | JSON round-trip preserves `alert` |
| A10 | Dry-run classifier uses existing fields only |
| A11 | After WHOIS-only update with soon expiry, `Alert=true` even if DNS untouched |
| A12 | After DNS-only empty update, `Alert=true` even if WHOIS untouched |
| P1 | Domain row with `Alert=true` includes `[x]` after expiry text |
| P2 | `Alert=false` → no `[x]` |
| P3 | `Expired` + `Alert` → expiry shows `[!][x]` |

## Implementation order (suggested)

1. Add `Alert` + `AlertSoonWindow` on `data` + `RefreshAlert` / tests (A1–A12).
2. Wire `RefreshAlert` into TUI paths: **`u`**, whois refresh, dns refresh, https refresh, domain-added jobs.
3. Domain list `[x]` marker in `DrawListDomainPanel` + panel tests (P1–P3).
4. `cmd/checkdomains` main: load, loop, set alert, print, flags, save.
5. Manual: TUI **`u`** on a known empty/near-expiry domain flips `alert` and shows `[x]`; CLI lists the same.
6. Tick DONTREADME TODO when done.

## Relation to other specs

| Spec | Relationship |
|------|----------------|
| [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md) | After each WHOIS update, also `RefreshAlert`; CLI is force-all |
| [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md) | After each DNS update, also `RefreshAlert`; `Alert` consumes `dns.Empty` |
| [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md) | After each HTTPS update, also `RefreshAlert`; `Alert` consumes https-expired / https-missing; CLI runs `UpdateHttps` |
| [SPEC_DOMAIN_ADDED_JOBS.md](SPEC_DOMAIN_ADDED_JOBS.md) | WHOIS/DNS on-add jobs refresh `Alert` |
| [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md) | Same whois/dns persistence; **new** top-level `alert` on domain |
