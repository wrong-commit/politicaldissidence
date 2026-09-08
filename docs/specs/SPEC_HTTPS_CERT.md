# Spec: HTTPS certificate status

## Goal

Probe each MP domain over HTTPS, classify the TLS certificate state, persist the latest result on `data.Domain`, and surface it in the **Domain Information** panel **below the WHOIS block and above the DNS section**:

```text
HTTPS Status: enabled/expired/missing
Certificate Expiry: 2026-XX-XX
```

Run the probe on the same trigger paths as WHOIS / DNS: **startup background scan**, Domains panel **`u`**, and **domain-added**. Recompute shared `Domain.Alert` whenever HTTPS (or WHOIS / DNS) state changes, including **`https-expired`** and **`https-missing`** reasons. The CLI batch tool (`cmd/checkdomains`) force-probes HTTPS too.

WHOIS answers registration / registrar expiry. DNS answers whether records still resolve. HTTPS answers whether a live TLS site still presents a usable certificate — a third abandonment / neglect signal that also feeds the sniping watchlist.

## Motivation

| Signal | Source today | Gap |
|--------|--------------|-----|
| Registrar expiry | WHOIS → `Domain.Expiry` / `WhoisRecord` | Domain can still be “live” or already abandoned hosting |
| Live DNS | `DnsRecord` | Records can exist while HTTPS is broken / cert dead |
| TLS cert validity | **None** | No check whether HTTPS works or the leaf cert has expired |
| Watchlist `Alert` | expired / soon / dns-empty | Cert neglect does not flag sniping review |

An expired or missing cert on an otherwise registered domain often means the owner stopped renewing TLS (or never stood up HTTPS). Useful alongside WHOIS + DNS for sniping / watchlist judgment.

Go’s standard library is enough (`crypto/tls`, `net`). No new module dependency for v1.

## Definition: HTTPS status

Unlike WHOIS / DNS (which look up the **apex** only after stripping a leading `www.`), HTTPS **always probes both** name forms on `:443`:

1. Normalize the stored hostname to an **apex** (strip one leading `www.` if present; case-fold as elsewhere).
2. Probe **apex** → e.g. `example.com.au:443` with SNI `example.com.au`.
3. Probe **www** → e.g. `www.example.com.au:443` with SNI `www.example.com.au`.

Each probe is an independent TLS client handshake. Do **not** require a successful verified handshake to inspect the leaf — use a dial that can still return peer certificates when the chain is expired or otherwise untrusted (`InsecureSkipVerify` on the inspect path), then classify locally.

### Per-host status

| Status | Meaning |
|--------|---------|
| **`enabled`** | Handshake yields at least one peer certificate, and the **leaf** `NotAfter` is strictly after “now” |
| **`expired`** | At least one peer certificate is present, and the leaf `NotAfter` is at or before “now” |
| **`missing`** | No usable peer certificate (connection refused, timeout, TLS with empty peer certs, plain TCP with no TLS, dial error, etc.) |

### Aggregate status (what the panel / `Alert` use)

Rank per-host results **`enabled` > `expired` > `missing`**. Overall status = **best** of the two hosts (apex and www).

| Apex | www | Overall | Rationale |
|------|-----|---------|-----------|
| enabled | anything | **enabled** | Site still presents a live cert on at least one common name |
| expired | missing / expired | **expired** | Cert present but dead; no live cert on either name |
| missing | missing | **missing** | No usable cert on either name |

`Certificate Expiry` for the summary line comes from the **winning** host (the one that supplied the overall status). If both are `enabled`, prefer the **earlier** `NotAfter` (soonest expiry). If both are `expired`, prefer the **earlier** `NotAfter` as well. If overall is `missing`, show `-`.

Notes:

- **Leaf** = `ConnectionState.PeerCertificates[0]` when present.
- **Hostname mismatch / unknown CA / self-signed** with a still-valid `NotAfter`: treat that host as **`enabled`** for v1 (status is about presence + calendar expiry, not full PKI trust).
- **Redirects / HTTP-only on :80**: v1 only checks **:443**. No HTTP upgrade / HSTS chase.
- **Timeout**: short dial timeout per host (suggested **5s**); that host → **`missing`** (store error text on that host result).
- Both dials run every check (no “stop early if apex enabled”) so persisted per-host detail stays complete; optional later: short-circuit for speed only.
- Input already stored as `www.…` still yields the same apex + www pair (do not probe `www.www.…`).

Derived display (aggregated; panel stays two lines):

```text
HTTPS Status: <enabled|expired|missing>
Certificate Expiry: <YYYY-MM-DD>   // from winning host; "-" when overall missing
```

| Overall status | `Certificate Expiry` line |
|----------------|---------------------------|
| `enabled` / `expired` | `Certificate Expiry: 2026-09-09` (date only, `2006-01-02`) |
| `missing` | `Certificate Expiry: -` |

## Definition: when `Alert` includes HTTPS

Amend the shared classifier in [SPEC_CLI_EXPIRED_DOMAINS.md](SPEC_CLI_EXPIRED_DOMAINS.md):

```text
Alert = (expired OR soon OR dns-empty OR https-expired OR https-missing)
```

| Reason | Condition |
|--------|-----------|
| **https-expired** | Latest **aggregated** `Https.Status == "expired"` |
| **https-missing** | Latest **aggregated** `Https.Status == "missing"` (both hosts missing, including dial/timeout) |

Notes:

- Domains **never probed** (`Https == nil`) contribute **neither** HTTPS reason.
- Aggregated `enabled` (either host live) never contributes an HTTPS alert bit — apex-only or www-only success is enough to stay off the HTTPS watchlist reasons.
- When **both** hosts are `missing`, **https-missing** applies (unlike DNS transport errors, which do not set dns-empty). Transient dual timeouts may flip alert until a later enabled check — accept for v1.
- After every HTTPS update (and existing WHOIS / DNS updates), call the same `RefreshAlert` so recovery (`enabled` again, or record cleared only by overwrite) can clear HTTPS bits when other reasons no longer apply.
- New constants: `AlertReasonHTTPSExpired = "https-expired"`, `AlertReasonHTTPSMissing = "https-missing"`.
- CLI stdout `reasons=` and summary counters include the new tokens; dry-run classifies from persisted `https` when present.

## Desired behaviour

### 1. New package: `httpscheck` (name TBD)

Mirror `whois/` / `dnscheck/`:

| Piece | Role |
|-------|------|
| `Lookup(hostname) (Info, error)` | Normalize to apex; probe apex **and** `www.`+apex via injectable dialer; aggregate |
| `LookupWith(hostname, Dialer)` | Test seam |
| `Info` | Aggregated status + per-host results |

Suggested types:

```go
type Status string // "enabled", "expired", "missing"

type HostInfo struct {
    Hostname string    // exact name dialed / SNI
    Status   Status
    NotAfter time.Time // zero when missing / unknown
    Message  string    // dial / hard failure for this host
}

type Info struct {
    Apex     HostInfo // e.g. example.com.au
    WWW      HostInfo // e.g. www.example.com.au
    Status   Status   // aggregated (best of Apex, WWW)
    NotAfter time.Time // from winning host; zero when overall missing
    Message  string    // optional roll-up when overall missing (e.g. join host messages)
}
```

Prefer: **lookup returns Info always for transport outcomes**; per-host `Status=missing` + `Message` on dial failure. Unexpected panics recovered at the `data` layer (same as WHOIS).

### 2. Persist latest HTTPS record on the domain

Each `data.Domain` may carry:

```json
"https": {
  "checkedAt": "...",
  "status": "enabled",
  "notAfter": "2026-09-09T12:00:00Z",
  "error": "",
  "apex": { "hostname": "example.com.au", "status": "enabled", "notAfter": "..." },
  "www":  { "hostname": "www.example.com.au", "status": "missing", "error": "..." }
}
```

Suggested Go:

```go
type HttpsHostRecord struct {
    Hostname string    `json:"hostname,omitempty"`
    Status   string    `json:"status,omitempty"`
    NotAfter time.Time `json:"notAfter,omitempty"`
    Error    string    `json:"error,omitempty"`
}

type HttpsRecord struct {
    CheckedAt time.Time       `json:"checkedAt,omitempty"`
    Status    string          `json:"status,omitempty"` // aggregated: enabled | expired | missing
    NotAfter  time.Time       `json:"notAfter,omitempty"`
    Error     string          `json:"error,omitempty"` // optional aggregate message
    Apex      *HttpsHostRecord `json:"apex,omitempty"`
    WWW       *HttpsHostRecord `json:"www,omitempty"`
}

// on Domain:
Https *HttpsRecord `json:"https,omitempty"`
```

Rules (same persistence pattern as [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md) / [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md)):

- **Only the latest** record is kept (overwrite on each attempt).
- Stored under the domain; written with normal MP saves (`db.WriteMps`).
- `checkedAt` is wall clock when that dual probe ran.
- Always store both `apex` and `www` host records from the lookup; top-level `status` / `notAfter` / `error` are the **aggregate**.
- Domains never probed omit `https` (`omitempty`).
- **Alert / panel summary** read aggregated `Status` / `NotAfter` only; per-host fields are for debugging and future UI detail.

`UpdateHttps` / `UpdateHttpsInfo` on `data.Domain`, with injectable lookup for tests (same pattern as `lookupDns`). **Always** call `RefreshAlert(checked, AlertSoonWindow)` at the end of the update (success or missing/error), matching WHOIS / DNS.

Throttle:

```go
const HttpsMaxAge = WhoisMaxAge // 10 days

func (d Domain) NeedsHttps(at time.Time, maxAge time.Duration) bool {
    if d.Https == nil || d.Https.CheckedAt.IsZero() {
        return true
    }
    return !d.Https.CheckedAt.After(at.Add(-maxAge))
}
```

Independent of WHOIS `LastChecked` and DNS `checkedAt`.

### 3. Background HTTPS runner

**Decided:** third independent runner — existing `whoisRunner` + `dnsRunner` + new **`httpsRunner`**. Each has its own single-flight mutex, throttle, delay, env skip, and log stream. A long WHOIS/DNS pass must not block HTTPS (and vice versa).

Mirror [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md) / DNS:

| Piece | Behaviour |
|-------|-----------|
| Trigger | Once after MPs load (`UI.Load` / `InitApp`), goroutine |
| Env skip | `SKIP_BACKGROUND_HTTPS_LOOKUP=true` (case-insensitive) → do not start; log once. Manual **`u`** unaffected |
| Scope | All `ui.state.all` MPs/domains (not filter/visible) |
| Skip | Fresh if `!NeedsHttps(now, HttpsMaxAge)` |
| Delay | **1s** between lookups that actually run |
| Persist | Do not save per domain; **save once** at end if any updates |
| UI | Off gocui thread; redraw via `g.Update` |
| Package | Thin `httpsrefresh` (or equivalent) reusing refresh Deps / Logger / Runner patterns — **do not** fold into whois or dns runners |

Logging prefixes (same INFO/DEBUG/ERROR style):

| Level | Example |
|-------|---------|
| INFO | `INFO checking all (42) MP domains HTTPS` |
| DEBUG | `DEBUG checking Jane Doe MP domain example.com.au HTTPS` |
| INFO | `INFO checked 12 MPs and updated 18 domains HTTPS` |

After each domain update in the walker: `RefreshAlert` (already inside `UpdateHttps*`).

### 4. Domains panel **`u`** (`checkDomain`)

**`u`** forces WHOIS **and** DNS **and** HTTPS for the selected domain (no throttle). Same “check domain” action refreshes all three. Prefer kicking three parallel paths or sequential forced updates — either is fine if the panel redraws when each finishes and `Alert` is recomputed after each update (HTTPS-only flip must not wait for WHOIS).

### 5. Domain-added job

Extend [SPEC_DOMAIN_ADDED_JOBS.md](SPEC_DOMAIN_ADDED_JOBS.md):

| Piece | Role |
|-------|------|
| `NewHttpsOnAdd` | Wraps UI HTTPS-on-add path as a `Job` (name e.g. `"https-on-add"`) |

Wire-up:

- Append `jobs.NewHttpsOnAdd(ui.refreshDomainHttps)` next to WHOIS / DNS in `UI.domainAddedJobs` (`NewUI`).
- `addDomain` → `jobs.Kick` already fans out; no change to Kick itself.
- `refreshDomainHttps(mpIndex, domainIdx)`: force probe, persist via existing on-add save path, redraw Domain Information when selection matches (`g.Update`, same as DNS on-add).
- `RefreshAlert` runs inside `UpdateHttps*`.

### 6. CLI (`cmd/checkdomains`)

Amend live algorithm in [SPEC_CLI_EXPIRED_DOMAINS.md](SPEC_CLI_EXPIRED_DOMAINS.md):

```text
Unless -dry-run:
  - Run Domain.UpdateExpiryInfo()
  - Run Domain.UpdateDns()
  - Run Domain.UpdateHttps()   // new
  - Sleep -delay
domain.RefreshAlert(...)       // now sees HTTPS reasons too
```

- `-dry-run` classifies from persisted Expiry / dns / **https** (no network).
- `-save` persists WHOIS, DNS, HTTPS, and `alert` once at end.
- Report `reasons=` may include `https-expired` / `https-missing`; summary counts those reasons.
- Optional column `https=enabled|expired|missing` on alert rows (nice-to-have; at least include in `reasons=`).

### 7. Domain Information panel

Amend the panel draw path (`DrawWhoisPanel` or successor) so the layout is:

```text
Checked: 26-09-08 15:04
example.com.au
MP: Jane Doe
...
Name servers:
  ns1.example.net

HTTPS Status: enabled
Certificate Expiry: 2026-09-09

DNS (26-09-08 15:05): empty
  A: (none)
  NS: (none)
  MX: (none)
```

Rules:

- HTTPS block sits **after** WHOIS content and **before** the DNS section (`appendHttpsSection` then `appendDnsSection`).
- Exact labels: `HTTPS Status:` and `Certificate Expiry:`.
- Status token is one of `enabled` / `expired` / `missing` (lowercase).
- If no HTTPS record yet: always show

  ```text
  HTTPS Status: not checked yet
  Certificate Expiry: -
  ```

- Do not fold HTTPS into the DNS block or the WHOIS “Expiry” line (registrar expiry ≠ cert expiry).

Signature change sketch:

```go
func DrawWhoisPanel(hostname, mpName string, w *data.WhoisRecord, https *data.HttpsRecord, dns *data.DnsRecord) string
```

Domain list `[x]` continues to mean `Alert` (any reason, including HTTPS). Optional later: `https:ok` / `https:exp` / `https:miss` markers — **out of scope for v1**.

### 8. Eligibility reading (manual)

| WHOIS | DNS | HTTPS | Interpretation |
|-------|-----|-------|----------------|
| Far expiry | `ok` | `enabled` | Active site — low interest (`Alert` false unless other bits) |
| Far expiry | `ok` | `expired` / `missing` | Hosting neglect — **`Alert` true** (HTTPS reasons) |
| Far expiry | `empty` | `missing` | Abandoned — **`Alert` true** (dns-empty + https-missing) |
| Near / past expiry | any | any | Purchase candidate (`Alert` from expired/soon) |

## Architecture sketch

```text
                    ┌─────────────┐
  u / startup / add │  ui / jobs  │
                    └──────┬──────┘
         ┌─────────────────┼─────────────────┐
         v                 v                 v
  whoisRunner        dnsRunner         httpsRunner
  refresh.Run        dnsrefresh.Run    httpsrefresh.Run
         │                 │                 │
         v                 v                 v
  UpdateExpiry*      UpdateDns*        UpdateHttps*
         │                 │                 │
         v                 v                 v
  whois.Lookup       dnscheck.Lookup   httpscheck.Lookup
         │                 │                 │
         └────────────┬────┴────────┬────────┘
                      v             v
              RefreshAlert    HttpsRecord → mp_data.json
                      │
                      v
         Domain Information (WHOIS → HTTPS → DNS) + [x] on Alert
```

CLI live path runs the three `Update*` calls then the same `RefreshAlert`.

## Acceptance criteria

- [ ] `httpscheck` package probes **both** apex and `www.`+apex on `:443` with injectable dialer; classifies per-host and aggregates (`enabled` > `expired` > `missing`)
- [ ] `Domain` persists latest `https` record (aggregate + `apex` / `www` host detail) in `mp_data.json` (`omitempty` when never checked)
- [ ] `NeedsHttps` / `HttpsMaxAge` (10 days) throttle background skips
- [ ] Background HTTPS scan via its own `httpsRunner`: all MPs, delay between lookups, save once, `SKIP_BACKGROUND_HTTPS_LOOKUP` env skip
- [ ] **`u`** forces WHOIS **and** DNS **and** HTTPS for the selected domain
- [ ] Domain-added kicks `NewHttpsOnAdd` alongside WHOIS / DNS
- [ ] `RefreshAlert` / `AlertReasons` include `https-expired` and `https-missing`; clear when status is `enabled` or HTTPS nil and no other reasons
- [ ] CLI live mode runs `UpdateHttps`; dry-run uses persisted `https`; report/summary include new reasons
- [ ] Domain Information panel shows the two HTTPS lines **below WHOIS and above DNS**
- [ ] `Certificate Expiry` uses `YYYY-MM-DD`; `-` when status is `missing` or not checked
- [ ] Unit tests cover classification, dual-host aggregation, persistence, alert reasons, and panel order without live network
- [ ] Input `www.…` normalizes to the same apex + www pair (no `www.www.…`)

## Out of scope (v1)

- Full certificate chain display, issuer, SANs, stapling, TLS version
- Treating hostname mismatch / untrusted CA as a separate status
- HTTP (:80) or redirect following
- Domain-list markers (`https:ok` / `https:exp` / …) beyond existing `[x]` from `Alert`
- History of HTTPS checks
- OCSP / CRL revocation
- Periodic re-run of background HTTPS beyond once-per-session startup (same as WHOIS/DNS v1)

## Unit test plan

Keep network I/O out of unit tests via an injectable dialer that returns a canned `tls.ConnectionState` (or leaf cert + error).

### 1. `httpscheck` classification

| ID | Case |
|----|------|
| H1 | Single-host leaf `NotAfter` in the future → that host `enabled` |
| H2 | Single-host leaf `NotAfter` in the past → that host `expired` |
| H3 | Single-host dial error / timeout / no peer certs → that host `missing`, Message set |
| H4 | Lookup dials **both** apex and `www.`+apex (dialer sees both names) |
| H5 | Input `www.example.com` → probes `example.com` and `www.example.com` only |
| H6 | Apex enabled, www missing → aggregate `enabled` |
| H7 | Apex missing, www enabled → aggregate `enabled` |
| H8 | Apex expired, www missing → aggregate `expired` |
| H9 | Both missing → aggregate `missing` |
| H10 | Both enabled → aggregate `enabled`; `NotAfter` is the earlier of the two |
| H11 | Self-signed but `NotAfter` future → that host `enabled` (v1 trust policy) |

### 2. `data.Domain` HTTPS update + alert

| ID | Case |
|----|------|
| D1 | Success stores `HttpsRecord` + `checkedAt` + status |
| D2 | Failure stores `missing` + error; does not invent NotAfter |
| D3 | JSON round-trip preserves `https`; missing field → nil |
| D4 | `NeedsHttps` never / fresh / stale boundary |
| A1 | `Status=expired` → reasons include `https-expired`, `Alert=true` |
| A2 | `Status=missing` → reasons include `https-missing`, `Alert=true` |
| A3 | `Status=enabled` → no HTTPS reasons; `Alert` follows other bits only |
| A4 | `Https==nil` → no HTTPS reasons |
| A5 | After HTTPS-only update to `enabled`, prior https-missing alert clears if no other reasons |
| A6 | WHOIS-soon + https-expired → both reasons present |

### 3. Background orchestration

Mirror SPEC_BACKGROUND_WHOIS §4 (count, skip, delay, continue on error, single-flight, env skip, save once) for HTTPS.

### 4. Jobs

| ID | Case |
|----|------|
| J1 | `NewHttpsOnAdd` invokes run with MP/domain indices from Context |
| J2 | nil run is a no-op (same as WHOIS/DNS on-add) |

### 5. Panel display

| ID | Case |
|----|------|
| P1 | HTTPS block appears after WHOIS text and before DNS section |
| P2 | `enabled` / `expired` / `missing` render exact status strings |
| P3 | Expiry line formats date as `YYYY-MM-DD`; `-` when missing / not checked |
| P4 | No HTTPS record → `not checked yet`, not a false `enabled` |

### 6. CLI

| ID | Case |
|----|------|
| C1 | Live path invokes HTTPS update (injectable / fake) before RefreshAlert |
| C2 | Dry-run with persisted `https.expired` prints alert with `https-expired` |
| C3 | Summary counts include https reason tallies |

## Implementation order (suggested)

1. `httpscheck` + tests (no UI).
2. `HttpsRecord` on `Domain` + `UpdateHttps` + `NeedsHttps` + JSON tests.
3. Extend `AlertReasons` / constants + alert tests (A1–A6).
4. `httpsrefresh` + background / env skip; wire startup.
5. Wire **`u`**, `NewHttpsOnAdd`, domain-added.
6. Domain Information panel HTTPS section.
7. CLI `UpdateHttps` + report/summary reasons.
8. Manual TUI + `checkdomains` pass on live / expired / refused :443 hosts.

## Related specs

| Spec | Relationship |
|------|----------------|
| [SPEC_DOMAIN_ADDED_JOBS.md](SPEC_DOMAIN_ADDED_JOBS.md) | Add `NewHttpsOnAdd` to the kick list |
| [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md) | Panel order: WHOIS → **HTTPS** → DNS; third independent runner |
| [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md) | Background throttle / delay / env-skip pattern |
| [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md) | Domain Information panel host |
| [SPEC_CLI_EXPIRED_DOMAINS.md](SPEC_CLI_EXPIRED_DOMAINS.md) | **Amended:** Alert formula + CLI live HTTPS probe |
