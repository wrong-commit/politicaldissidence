# Spec: DNS emptiness check

## Goal

Today the app only runs **WHOIS** to judge whether an MP domain looks eligible for purchase (expiry, registrar, name servers). That misses a common abandonment signal: the domain is still registered, but **DNS has been emptied** (no useful records resolving).

Add a **DNS check** that queries public DNS for the domain, persists the latest result on each `data.Domain`, surfaces it in the TUI, and runs on the same trigger paths as WHOIS (startup scan, **`u`** / check domain, domain-added).

WHOIS answers “is it registered / when does it expire?”. DNS emptiness answers “is anything still pointing at this name?”. Together they better flag drop / park / abandon candidates.

## Motivation (investigation)

| Signal | Source today | Gap |
|--------|--------------|-----|
| Expiry date | WHOIS → `Domain.Expiry` / `WhoisRecord` | Does not say whether the site is live |
| Name servers (from WHOIS) | Stored on `WhoisRecord.NameServers` | Declared NS ≠ live DNS; can be stale or parking NS |
| Live A / AAAA / MX / NS / TXT | **None** | No check whether records were cleared |

A registered domain with emptied DNS often means the owner stopped hosting (or never set records). That is useful even when expiry is months away. NXDOMAIN / no NS can also appear during redemption / pending delete after WHOIS status changes — complementary to expiry.

Go’s standard library is enough (`net.Resolver` / `LookupIP`, `LookupNS`, `LookupMX`, `LookupTXT`). No new module dependency required for v1.

## Definition: “DNS empty”

A hostname is **DNS-empty** when **all** of the following live lookups find nothing useful:

| Record | Empty when |
|--------|------------|
| **NS** | Lookup fails or returns zero name servers |
| **A / AAAA** | No IPv4 and no IPv6 addresses |
| **MX** | Lookup fails or returns zero MX hosts |
| **TXT** | Lookup fails or returns zero strings |

Notes:

- **CNAME-only** apex is rare for registrable domains; if an A/AAAA chase via CNAME yields addresses, treat as **not** empty.
- **NXDOMAIN** (name does not exist in DNS) counts as empty for eligibility purposes, but store the distinct outcome so the UI can say `NXDOMAIN` vs `NODATA`.
- **SERVFAIL / timeout / network error** is **not** empty — store an error; do not flip `dnsEmpty` to true.
- **www.** stripping should match WHOIS (`whois.clean`): look up the apex hostname used for WHOIS, not `www.`-prefixed forms.

Derived boolean:

```text
dnsEmpty = (outcome is NXDOMAIN or NODATA) AND error == ""
```

Optional later (out of scope for v1): treat “only parking / sinkhole NS” as a softer signal. v1 is strict emptiness only.

## Desired behaviour

### 1. New package: `dnscheck` (name TBD)

Mirror `whois/`:

| Piece | Role |
|-------|------|
| `Lookup(hostname) (Info, error)` | Live DNS via injectable resolver |
| `LookupWith(hostname, Resolver)` | Test seam |
| `Info` | Structured record sets + outcome + message |

Suggested `Info`:

```go
type Outcome string // "ok", "empty", "nxdomain", "error"

type Info struct {
    Hostname string
    Outcome  Outcome
    Empty    bool     // true only for empty / nxdomain with no transport error
    A        []string // IPv4/IPv6 string forms
    NS       []string
    MX       []string // host names (priority optional in display)
    TXT      []string // each string truncated to MaxTXTChars for persistence
    Message  string   // set on hard failure
}

// MaxTXTChars is the per-string cap when storing TXT in DnsRecord / Info.
const MaxTXTChars = 250
```

Implementation sketch:

- Prefer a single `net.Resolver` with a bounded timeout (e.g. 5s context).
- **Resolver (decided):** use **fixed public resolvers**, not the OS default, so results are consistent across machines. v1 dials:
  - `1.1.1.1:53` (Cloudflare)
  - `8.8.8.8:53` (Google) as fallback if the first dial/query fails
  - Implement via `net.Resolver{PreferGo: true, Dial: ...}` (or equivalent) so lookups do not depend on local DNS config.
- Run NS, IP, MX, TXT lookups (sequential is fine for v1; parallel OK if timeouts stay sane).
- Classify:
  - resolver error → `Outcome=error`, `Empty=false`
  - NXDOMAIN (or equivalent: no such host on all lookups) → `Outcome=nxdomain`, `Empty=true`
  - all sets empty / no records → `Outcome=empty`, `Empty=true`
  - any A/AAAA/NS/MX/TXT present → `Outcome=ok`, `Empty=false`
- **TXT persistence (decided):** keep every TXT string returned by the lookup, but truncate **each** string to the first **250** characters (`MaxTXTChars`). Truncation does not change emptiness classification (a truncated string still counts as present).

Do **not** put this inside the `whois` package; keep network concerns separate so refresh jobs can run independently.

### 2. Persist latest DNS on the domain

Each `data.Domain` may carry (parallel to `whois`):

```json
"dns": {
  "checkedAt": "...",
  "empty": true,
  "outcome": "empty",
  "a": [],
  "ns": [],
  "mx": [],
  "txt": [],
  "error": ""
}
```

Rules (same persistence pattern as [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md)):

- **Only the latest** record is kept (overwrite each attempt).
- Written with normal MP saves (`db.WriteMps`), including end of background scan.
- `checkedAt` is wall clock when the lookup ran.
- On **success** (including empty/nxdomain/ok): fill fields; clear `error`; set `empty` from classification.
- On **hard failure**: leave prior DNS summary fields if desired, or replace with `checkedAt` + `error` only (prefer **replace with checkedAt + error**, matching WHOIS failure behaviour).
- Domains never looked up omit `dns` (`omitempty`).

Optional top-level convenience (either is fine; pick one in implementation):

- Rely on `dns.empty` only, **or**
- Mirror a short flag on `Domain` (e.g. `DnsEmpty bool`) for the domain list row — only if it simplifies panel code without dual sources of truth. Prefer reading `Domain.DNS.Empty` in the UI to avoid drift.

Throttle field: either reuse a shared `LastChecked` (not ideal — DNS and WHOIS ages differ) or add `DnsLastChecked` / use `dns.checkedAt` via `NeedsDns(at, maxAge)`. **Prefer `NeedsDns` based on `dns.checkedAt`**, independent of WHOIS `LastChecked`.

Suggested max age: same **10 days** as WHOIS (`DnsMaxAge = WhoisMaxAge`) unless tuning shows otherwise.

### 3. Trigger paths

Align with existing WHOIS wiring:

| Path | Behaviour |
|------|-----------|
| Startup background scan | After (or alongside) WHOIS: walk all MPs/domains; skip if DNS fresh; delay between lookups; save once at end |
| Domains panel **`u`** (`checkDomain`) | Forced DNS for selected domain (no throttle), same as forced WHOIS |
| Domain-added (`jobs`) | New `NewDnsOnAdd` job next to `NewWhoisOnAdd` |

Env skip (mirror WHOIS):

- `SKIP_BACKGROUND_DNS_LOOKUP=true` → do not start the background DNS scan.
- Manual **`u`** and domain-added remain unaffected.

Whether **`u`** runs WHOIS and DNS in one action: **yes for v1** — one “check domain” should refresh both. Domain-added kicks both jobs.

**Background concurrency (decided):** use **two independent runners** — existing `whoisRunner` plus a new `dnsRunner`. Each has its own single-flight mutex, throttle, delay, env skip, and log stream. A long WHOIS pass must not block DNS (and vice versa). This also keeps future concerns (different max age, rate limits, cancel, or extra check types) from coupling the two pipelines.

Logging prefixes (same INFO/DEBUG/ERROR style as [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md)):

| Level | Example |
|-------|---------|
| INFO | `INFO checking DNS for all (42) MP domains` |
| DEBUG | `DEBUG dns Jane Doe MP domain example.com.au` |
| INFO | `INFO dns checked 12 MPs and updated 18 domains` |
| ERROR | `ERROR dns example.com.au: i/o timeout` |
| INFO | `INFO background DNS skipped (SKIP_BACKGROUND_DNS_LOOKUP=true)` |

### 4. UI

#### Domain list (`DOMAIN_PANEL`)

Extend the row so emptiness is visible at a glance, without crowding:

```text
1. example.com.au 2027-01-01  checked 26-03-01  dns:empty
1. example.com.au 2027-01-01[!]  checked 26-03-01  dns:ok
1. example.com.au <?>  checked never  dns:?
```

Suggested markers:

| State | Marker |
|-------|--------|
| Never checked | `dns:?` |
| Empty / NXDOMAIN | `dns:empty` (optionally emphasize like `[!]` later) |
| Has records | `dns:ok` |
| Last check errored | `dns:err` |

#### Domain Information panel (renamed from WHOIS information)

**Decided:** change the panel title from `"WHOIS information"` to **`"Domain Information"`** (amends [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md)). Keep the existing view id / constant (`WHOIS_PANEL` / `"whois"`) in v1 to avoid a needless rename churn; only the **visible title** changes.

Append a **DNS** section under the WHOIS block for the selected domain:

```text
Checked: 26-09-08 15:04
example.com.au
MP: Jane Doe
...
Name servers:
  ns1.example.net

DNS (26-09-08 15:05): empty
  A: (none)
  NS: (none)
  MX: (none)
```

If no DNS record yet: omit section or show `DNS: not checked yet`.

### 5. Eligibility (v1: display only)

v1 does **not** auto-set `Domain.Expired` from DNS. Expiry/`[!]` stay WHOIS-driven (and the existing TODO to compare expiry to “now” remains separate).

Document the intended **manual** reading:

| WHOIS | DNS | Interpretation |
|-------|-----|----------------|
| Far expiry, ok status | `ok` | Active / hosted — low interest |
| Far expiry | `empty` / `nxdomain` | Abandoned hosting — watchlist |
| Near / past expiry | any | Purchase candidate (existing focus) |
| pendingDelete / redemption (status) | `empty` | Strong drop signal |

A combined “eligibility score” or filter (`Ctrl+F` for empty DNS) is **out of scope** for v1 but should be easy once `dns` is persisted.

## Architecture sketch

```text
                    ┌─────────────┐
  u / startup / add │  ui / jobs  │
                    └──────┬──────┘
              ┌────────────┴────────────┐
              v                         v
       whoisRunner               dnsRunner
       refresh.Run               dnsrefresh.Run
              │                         │
              v                         v
       data.UpdateExpiry*        data.UpdateDns*
              │                         │
              v                         v
       whois.Lookup              dnscheck.Lookup
              │                         │
              v                         v
       WhoisRecord               DnsRecord  → mp_data.json
              │                         │
              └────────────┬────────────┘
                           v
                  Domain Information panel + domain list
```

Reuse patterns from `refresh` (Deps, Logger, Runner, delay, Force, Save-once) in a thin dedicated `dnsrefresh` package for v1 — do not fold DNS into the WHOIS runner or a single combined pass. A later generic “domain walker” is optional cleanup, not required for v1.

## Acceptance criteria

- [x] `dnscheck` package looks up NS / A|AAAA / MX / TXT with injectable resolver; classifies ok / empty / nxdomain / error
- [x] `Domain` persists latest `dns` record in `mp_data.json` (omitempty when never checked)
- [x] Background DNS scan via its own `dnsRunner` (independent of `whoisRunner`): all MPs, 10-day skip via `NeedsDns`, delay between lookups, save once, env skip
- [x] **`u`** forces WHOIS **and** DNS for the selected domain
- [x] Domain-added kicks a DNS job as well as WHOIS
- [x] Domain list shows `dns:?` / `dns:ok` / `dns:empty` / `dns:err`
- [x] WHOIS panel title is `"Domain Information"`; shows a DNS summary for the selected domain
- [x] Hard DNS errors do not mark the domain empty
- [x] TXT values persisted with per-string cap of 250 characters
- [x] DNS lookups use fixed public resolvers `1.1.1.1` and `8.8.8.8` (not OS default)
- [x] Unit tests cover classification and persistence without live network

## Out of scope (v1)

- Changing WHOIS libraries or panel layout beyond appending DNS and renaming the title to Domain Information
- Renaming the `WHOIS_PANEL` / `"whois"` view constant (title only in v1)
- Authoritative-only queries / per-user custom resolver config (v1 is fixed 1.1.1.1 + 8.8.8.8 only)
- DNSSEC, CAA, SRV, HTTPS/SVCB
- Treating parking nameservers as “empty”
- Combined eligibility score / new MP filter
- History of DNS checks
- CLI batch script for empty+expired domains (see [SPEC_CLI_EXPIRED_DOMAINS.md](SPEC_CLI_EXPIRED_DOMAINS.md))
- Alerting when DNS flips from ok → empty

## Unit test plan

Keep network I/O out of unit tests via an injectable resolver interface.

### 1. `dnscheck` classification

| ID | Case |
|----|------|
| C1 | All record sets non-empty → `ok`, `Empty=false` |
| C2 | All empty / no records → `empty`, `Empty=true` |
| C3 | NXDOMAIN-style failure → `nxdomain`, `Empty=true` |
| C4 | Timeout / SERVFAIL → `error`, `Empty=false`, Message set |
| C5 | A present, NS empty → `ok` (not empty) |
| C6 | `www.` stripped before lookup (fetcher sees apex) |
| C7 | Each TXT string longer than 250 chars is stored truncated to 250; still `ok` / not empty |
| C8 | Default live path is configured for fixed public resolvers (1.1.1.1 / 8.8.8.8); unit tests still inject a fake resolver |

### 2. `data.Domain` DNS update

| ID | Case |
|----|------|
| D1 | Success stores `DnsRecord` + `checkedAt` |
| D2 | Failure stores error record; does not set `Empty=true` |
| D3 | JSON round-trip preserves `dns`; missing field → nil |
| D4 | `NeedsDns` never / fresh / stale boundary (same as WHOIS throttle tests) |

### 3. Background orchestration

Mirror SPEC_BACKGROUND_WHOIS §4 (count, skip, delay, continue on error, single-flight, env skip, save once) for DNS.

### 4. Panel display

| ID | Case |
|----|------|
| P1 | Domain row markers for ? / ok / empty / err |
| P2 | Domain Information panel appends DNS block when present; title is `Domain Information` |
| P3 | No DNS record → no false “empty” claim |

## Implementation order (suggested)

1. `dnscheck` + tests (no UI).
2. `DnsRecord` on `Domain` + `UpdateDns` + JSON tests.
3. `dnsrefresh` (or equivalent) + background / env skip.
4. Wire **`u`**, domain-added job, startup.
5. Domain list markers + Domain Information panel DNS section + title rename.
6. Manual TUI pass on a known empty domain and a live one.
