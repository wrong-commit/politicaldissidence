# Spec: TLD registrar availability lookup (GoDaddy first)

## Goal

Run a **registrar availability lookup** on the same trigger paths as WHOIS / DNS / HTTPS:


| Path                      | Behaviour                                                                  |
| ------------------------- | -------------------------------------------------------------------------- |
| Startup background scan   | Throttled (`RegistrarMaxAge`, same 10-day window)                          |
| Periodic 30-minute ticker | Same throttle; subject to `SKIP_PERIODIC_DOMAIN_CHECKS`                    |
| **Ctrl+P**                | Force all matching domains (ignore freshness; ignore per-check skip env)   |
| Domains panel `**u**`     | Force selected domain (no throttle)                                        |
| Domain-added              | Force new domain via `jobs.Kick`                                           |
| `cmd/checkdomains`        | Force every domain (live mode); classify from persisted data in `--dry-run` |


For each hostname, **sources come from `domain_lookups.json` by TLD**. v1 implements **GoDaddy** only. The shipped config **includes `namecheap`**; until that client exists, selecting it logs **ERROR** and skips that source (other sources in the list still run).

Persist per source on the domain and show results **at the top of the Domain Information panel** (immediately after Alert Details when present; otherwise as the first block).

Recorded fields per source (string tri-state, not booleans):


| Field             | Values                 | Meaning                                                                                        |
| ----------------- | ---------------------- | ---------------------------------------------------------------------------------------------- |
| **purchaseable**  | `yes` / `no` / `weird` | Clear buy / clear not-buy / **in-between** (could not determine cleanly)                       |
| **weirdResponse** | `yes` / `no` / `weird` | Clearly abnormal response / clean response / **in-between** (ambiguous / partially understood) |


The `**weird**` value on either field is an explicit sniping signal: something strange happened. It feeds `**Domain.Alert**`.

## Motivation


| Signal                          | Source today                | Gap                                                                             |
| ------------------------------- | --------------------------- | ------------------------------------------------------------------------------- |
| WHOIS expiry / status           | `WhoisRecord`               | Does not say whether a registrar will sell the name **now**                     |
| DNS empty / HTTPS neglect       | `DnsRecord` / `HttpsRecord` | Abandonment signals, not cart-availability                                      |
| “Can I buy this on GoDaddy?”    | **None**                    | For `.com.au` especially, cart availability is a strong sniping / watchlist cue |
| Ambiguous registrar API answers | **None**                    | Errors and half-parsed bodies were either ignored or forced into a false “no”   |


WHOIS can show a registered domain; GoDaddy may still refuse sale, or occasionally report available when WHOIS says otherwise. A third `**weird**` state keeps uncertain answers out of both “buy it” and “definitely not” without losing the alert.

## Definition: purchaseable / weirdResponse (tri-state)

Both fields use the same enum stored as JSON strings:

```go
type TriState string // "yes", "no", "weird"

const (
    TriYes   TriState = "yes"
    TriNo    TriState = "no"
    TriWeird TriState = "weird"
)
```

### purchaseable


| Value     | When                                                                                                                                                                              |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **yes**   | HTTP success **and** a clear boolean “available / can purchase” from the source is **true**                                                                                       |
| **no**    | HTTP success **and** clear boolean is **false**                                                                                                                                   |
| **weird** | Availability cannot be determined cleanly (errors, malformed body, missing field, auth failure, timeout, or other non-clean outcomes). **In-between** — not a confident yes or no |


### weirdResponse


| Value     | When                                                                                                                                                                                             |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **no**    | Parsed a normal availability payload and mapped purchaseable to **yes** or **no** unambiguously                                                                                                  |
| **yes**   | Clearly abnormal: non-2xx HTTP; empty body; JSON parse failure; source “error” object; missing credentials; transport failure                                                                    |
| **weird** | Ambiguous / partially understood: e.g. 200 with unexpected types, extra error alongside a boolean, unknown enum values, or other “something’s off but not a hard failure” shapes. **In-between** |


### Classification matrix (normative)


| Scenario                                                      | purchaseable | weirdResponse |
| ------------------------------------------------------------- | ------------ | ------------- |
| Clean `available: true`                                       | `yes`        | `no`          |
| Clean `available: false`                                      | `no`         | `no`          |
| Non-2xx / timeout / dial error / missing credentials          | `weird`      | `yes`         |
| Empty body / JSON parse failure / missing `available`         | `weird`      | `yes`         |
| 200 but ambiguous shape (partial parse, contradictory fields) | `weird`      | `weird`       |


Rules:

- **Never** set `purchaseable=yes` unless the response is clean (`weirdResponse=no`).
- Hard failures use `purchaseable=weird` + `weirdResponse=yes` (not `purchaseable=no`) so uncertain cases stay visible for alerting.
- Always set `checkedAt` on every attempt (success or failure).
- Domains never looked up omit the source blob (`omitempty`).

### Alert integration

Amend the shared classifier:

```text
Alert = (…existing reasons… OR registrar-weird OR registrar-purchaseable)
```


| Reason                       | Condition                                                                                                                       |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `**registrar-weird**`        | Any entry in `RegistrarLookups` has `purchaseable == "weird"` **or** `weirdResponse == "yes"` **or** `weirdResponse == "weird"` |
| `**registrar-purchaseable**` | Any entry has `purchaseable == "yes"`                                                                                           |


Notes:

- Domains with `RegistrarLookups == nil` / empty contribute **neither** reason.
- After every registrar update (and existing WHOIS / DNS / HTTPS updates), call `RefreshAlert`.
- New constants: `AlertReasonRegistrarWeird = "registrar-weird"`, `AlertReasonRegistrarPurchaseable = "registrar-purchaseable"`.
- Domain list `[x]` and Alert Details panel pick these up automatically via `AlertReasons`.
- CLI `checkdomains` stdout `reasons=` / counters include the new tokens.

### GoDaddy (v1 source)

Call GoDaddy’s single-domain availability check (preferred: `GET /v1/domains/available?domain={fqdn}` with `Authorization: sso-key {key}:{secret}`, or the current v3 check-availability + Bearer PAT if that is what credentials support — pick **one** in implementation and keep it injectable).

- Query the **apex** hostname (same `www.` strip convention as WHOIS / DNS).
- Use `checkType=FULL` / `optimizeFor=ACCURACY` (or equivalent) when the API offers a live-registry option; otherwise document the default.
- Map body field `available` (boolean) → purchaseable `yes`/`no` only when the response is well-formed (`weirdResponse=no`).
- Do **not** treat pricing / suggestions / agreements as required for v1.

Credentials via env (document in [ENVIRONMENT.md](../ENVIRONMENT.md)):


| Variable                           | Required         | Notes                                                                                                                          |
| ---------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `GODADDY_API_KEY`                  | for live GoDaddy | SSO key (or document PAT-only alternative)                                                                                     |
| `GODADDY_API_SECRET`               | with key         | SSO secret                                                                                                                     |
| *(optional later)* `GODADDY_PAT`   | alt auth         | Only if implementation uses Bearer PAT instead of sso-key                                                                      |
| `SKIP_BACKGROUND_REGISTRAR_LOOKUP` | no               | When `true`, startup + periodic registrar scans do not run. Manual `**u**` / domain-added / Ctrl+P / `checkdomains` still run. |


If credentials are missing when a GoDaddy lookup runs: log ERROR, persist `purchaseable=weird` / `weirdResponse=yes` / `error=missing GoDaddy credentials`, refresh alert, redraw panel. Do not panic.

## TLD → sources config

New file next to the app (same pattern as `search_terms.json` / `csv_refresh.json`).

**Canonical format** (use from day one — multiple TLDs, ordered source lists):

`**domain_lookups.json**`

```json
{
  "com.au": ["godaddy"],
  "com": ["godaddy", "namecheap"]
}
```


| Rule                              | Behaviour                                                                                                                                 |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Key                               | TLD suffix **without** leading dot, lowercase (`com.au`, `com`, later `net.au`, …)                                                        |
| Value                             | Ordered list of source ids to run for that TLD                                                                                            |
| Match                             | Longest suffix wins (`example.com.au` → `com.au`, not `au`; `example.com` → `com`)                                                        |
| Unknown TLD / no match            | Run **no** registrar lookups (WHOIS/DNS/HTTPS still run)                                                                                  |
| Unimplemented / unknown source id | **Decided:** log ERROR, **do not** call the network, **do not** write `RegistrarLookups[source]`, continue remaining sources in the list |
| Reload                            | Reload config at the start of each scan / kick / CLI run (no restart needed); invalid/missing file → no registrar lookups + one ERROR log |

**Ship `domain_lookups.json` exactly as above** (including `"com": ["godaddy", "namecheap"]`). Do **not** omit `namecheap` until a client exists.

### Unimplemented source (Namecheap v1)

When the dispatcher resolves source id `namecheap` (or any id with no client):

```text
ERROR registrar namecheap example.com: not implemented
```

Rules:

- Log once per hostname/source attempt (same cadence as other per-lookup ERROR lines).
- Skip that source only; still run `godaddy` (and any other implemented sources) for the same hostname.
- Do **not** persist a stub `RegistrarLookups["namecheap"]` entry and do **not** flip alert solely because Namecheap is unimplemented.
- **Due / throttle:** only **implemented** configured sources count for `NeedsRegistrar` freshness and “updated” counters. If a TLD lists only unimplemented sources, the hostname is never background-due (avoids ERROR spam every tick). Force paths (`u`, Ctrl+P, domain-added, `checkdomains` live) always walk the full source list — implemented sources look up; unimplemented sources ERROR + skip.

## Desired behaviour

### 1. Persist on `data.Domain`

```go
// RegistrarLookupRecord is the latest check for one registrar source.
type RegistrarLookupRecord struct {
    CheckedAt     time.Time `json:"checkedAt,omitempty"`
    Purchaseable  string    `json:"purchaseable"`  // yes | no | weird
    WeirdResponse string    `json:"weirdResponse"` // yes | no | weird
    Error         string    `json:"error,omitempty"`
}

type Domain struct {
    // ...
    // Keyed by source id: "godaddy", later "namecheap", …
    RegistrarLookups map[string]*RegistrarLookupRecord `json:"registrarLookups,omitempty"`
}
```

Example JSON:

```json
"registrarLookups": {
  "godaddy": {
    "checkedAt": "2026-09-11T21:00:00+10:00",
    "purchaseable": "no",
    "weirdResponse": "no"
  }
}
```

Weird / alert example:

```json
"godaddy": {
  "checkedAt": "2026-09-11T21:00:00+10:00",
  "purchaseable": "weird",
  "weirdResponse": "yes",
  "error": "missing GoDaddy credentials"
}
```

- Only the **latest** result per source is kept (overwrite).
- Written with normal MP saves (`Ctrl+S`); CLI live mode saves unless `-save=false`.
- In-memory update on each lookup completion; `RefreshAlert` after each write.

### 2. Freshness / NeedsRegistrar

Mirror HTTPS:

```go
const RegistrarMaxAge = WhoisMaxAge // 10 days

// NeedsRegistrar reports whether any configured source for this hostname is due.
// Never-checked source or checkedAt older than maxAge → due.
func (d Domain) NeedsRegistrar(at time.Time, maxAge time.Duration, sources []string) bool
```

- Background / periodic: skip hostname when **no** configured sources, or **all** configured sources are still fresh.
- If config lists `godaddy` + `namecheap` and only godaddy is fresh, still run the stale/missing sources.
- Force paths (`u`, Ctrl+P, domain-added, `checkdomains` live): ignore freshness.

### 3. Package layout


| Piece                                         | Role                                                                                                    |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `registrarcheck/` (façade + `godaddy` client) | `Lookup(hostname, source) (Info, error)` with injectable HTTP client                                    |
| `domainlookups/`                              | Load/parse `domain_lookups.json`; `SourcesForHostname(host) []string`                                   |
| `data.Domain`                                 | `UpdateRegistrarLookup` / `NeedsRegistrar` + alert reasons                                              |
| `registrarrefresh/`                           | Batch runner like `dnsrefresh` / `httpsrefresh` (log lines, throttle, force)                            |
| `jobs.NewRegistrarOnAdd`                      | Domain-added job                                                                                        |
| UI helpers                                    | `refreshDomainRegistrar`, `startBackgroundRegistrar`, wire into `rerunBackgroundChecks` / `checkDomain` |


Suggested `Info`:

```go
type Info struct {
    Source        string
    Hostname      string
    Purchaseable  TriState // yes | no | weird
    WeirdResponse TriState // yes | no | weird
    Message       string
}
```

### 4. Trigger wire-up

#### Domains panel `**u**` (`checkDomain`)

1. Existing WHOIS + DNS + HTTPS force refresh.
2. Force registrar refresh for configured sources on the selected domain (job or shared refresh helper).
3. Redraw Domain Information when each completes.

#### Domain-added

Append `jobs.NewRegistrarOnAdd(ui.refreshDomainRegistrar)` alongside WHOIS/DNS/HTTPS in `domainAddedJobs`.

#### Background startup + periodic + **Ctrl+P**

- `startBackgroundRegistrar(force bool)` parallel to whois/dns/https.
- `rerunBackgroundChecks` starts registrar too (force on Ctrl+P).
- Honor `SKIP_BACKGROUND_REGISTRAR_LOOKUP` when `force=false` only.
- Periodic ticker already gated by `SKIP_PERIODIC_DOMAIN_CHECKS`; when armed, include registrar in each tick.

Log style (match existing):


| Level | Example                                                              |
| ----- | -------------------------------------------------------------------- |
| INFO  | `INFO checking all (42) MP domains registrar`                        |
| DEBUG | `DEBUG checking Jane Doe MP domain example.com.au registrar godaddy` |
| INFO  | `INFO checked 12 MPs and updated 18 domains registrar`               |


#### CLI (`cmd/checkdomains`)

- Live: force registrar lookups for every domain with configured sources (same delay flag as other probes).
- `--dry-run`: no network; classify `Alert` from persisted `registrarLookups` when present.
- Errors log to stderr; do not abort the whole run on one domain failure.

### 5. Domain Information panel (top)

Amend `DrawWhoisPanel` (signature gains registrar map, or a small view-model).

**Order:**

1. `==[Alert Details]==` … (unchanged when reasons present — may now list `registrar-weird` / `registrar-purchaseable`)
2. **Registrar block** ← **new, top of lookup content**
3. WHOIS
4. HTTPS
5. DNS

When there is at least one `RegistrarLookups` entry, render **above** WHOIS:

```text
GoDaddy: purchaseable=yes  weird=no
```

or multi-source / weird:

```text
GoDaddy: purchaseable=weird  weird=yes
Namecheap: purchaseable=no  weird=no
```

Display rules:

- Source label: known pretty name (`godaddy` → `GoDaddy`).
- Print stored tri-state strings as-is (`yes` / `no` / `weird`).
- Panel label `weird=` maps to field `weirdResponse`.
- Optional: append checked time like DNS ( `(06-01-02 15:04)`).
- If `error` non-empty and `weirdResponse` is `yes` or `weird`, optional one indented `Error: …` line (truncate long bodies).
- If map is empty / nil: **omit** the registrar block (no “not checked yet” noise on unmatched TLDs).
- Empty panel message (`No domain lookup yet`) only when alert empty **and** WHOIS/HTTPS/DNS all nil **and** no registrar entries.

### 6. Logging (per lookup)


| Level | Example                                                                      |
| ----- | ---------------------------------------------------------------------------- |
| DEBUG | `DEBUG registrar godaddy checking Jane Doe example.com.au`                   |
| INFO  | `INFO registrar godaddy example.com.au purchaseable=no weirdResponse=no`     |
| ERROR | `ERROR registrar godaddy example.com.au: missing credentials` / HTTP / parse |
| ERROR | `ERROR registrar namecheap example.com: not implemented` (stub until client exists) |


### 7. Tests (before / with implementation)

Write tests first where practical (table-driven), then implement:


| Area             | Cases                                                                                                                                                                                         |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `domainlookups`  | longest TLD match (`com.au` vs `com`); multi-source list order; no match; empty/invalid file; shipped file includes `namecheap`                                                               |
| Dispatcher       | `namecheap` (unimplemented) → ERROR `not implemented`, no persist, continues to next source; unknown id same behaviour                                                                      |
| GoDaddy client   | available true/false → yes/no + weirdResponse no; bad JSON → purchaseable weird + weirdResponse yes; ambiguous 200 → both weird; 401 / missing creds → purchaseable weird + weirdResponse yes |
| `AlertReasons`   | registrar-weird / registrar-purchaseable; no lookups → neither                                                                                                                                |
| `NeedsRegistrar` | never / fresh / stale / partial sources due                                                                                                                                                   |
| `Domain` persist | map upsert; overwrite; omitempty                                                                                                                                                              |
| `DrawWhoisPanel` | registrar block after alert, before WHOIS; shows `weird`; multi-source; omit when nil                                                                                                         |
| Refresh / jobs   | background skip fresh; force paths always; domain-added kicks job; checkdomains `--dry-run` classifies                                                                                         |


Prefer injectable HTTP round-tripper; no live GoDaddy calls in unit tests.

## Out of scope (v1)

- Namecheap **HTTP client** / credentials — only the ERROR stub path above
- Purchase / cart / checkout flows
- Pricing display
- History of registrar checks
- Changing WHOIS/DNS/HTTPS layout beyond inserting the registrar block at the top

## Acceptance checklist

- [ ] Shipped `domain_lookups.json` includes `com.au` → godaddy and `com` → godaddy + namecheap
- [ ] Unimplemented `namecheap` logs `ERROR registrar namecheap <host>: not implemented`, no persist, does not block godaddy
- [ ] `domain_lookups.json` uses multi-TLD ordered source lists; longest-suffix match
- [ ] Registrar runs on: background, periodic, **Ctrl+P**, `**u`**, domain-added, `cmd/checkdomains`
- [ ] `purchaseable` and `weirdResponse` stored as `yes`  `no`  `weird`
- [ ] Hard/uncertain failures use `purchaseable=weird` (not forced `no`)
- [ ] `Alert` includes `registrar-weird` and `registrar-purchaseable`
- [ ] Domain Information shows registrar lines **at the top** of lookup content (after Alert Details)
- [ ] Missing credentials / HTTP failure are safe (log + persist weird states, no crash)
- [ ] `SKIP_BACKGROUND_REGISTRAR_LOOKUP` documented; force / manual / CLI unaffected
- [ ] Unit tests cover config, classification, alert, freshness, panel order, persist

## Suggested implementation order

1. Spec review (this doc) — **done when accepted**
2. Config loader + TLD match tests (`domain_lookups.json` multi-TLD format)
3. GoDaddy client + tri-state classification tests
4. `data.Domain` field + `NeedsRegistrar` + `AlertReasons` tests
5. Panel: registrar block at top + tests
6. `registrarrefresh` + UI background / Ctrl+P / `**u**` / domain-added jobs
7. `cmd/checkdomains` integration
8. `ENVIRONMENT.md` / README / shortcuts for GoDaddy env + skip flag + `domain_lookups.json`

## Related


| Doc                                                                                                                            | Relation                                                 |
| ------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------- |
| [SPEC_DOMAIN_ADDED_JOBS.md](SPEC_DOMAIN_ADDED_JOBS.md)                                                                         | `jobs.Kick` / `Job` pattern for domain-added             |
| [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md) / [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md) / [SPEC_HTTPS_CERT.md](SPEC_HTTPS_CERT.md) | Domain Information panel + check stack to mirror         |
| [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md) / [SPEC_CLI_EXPIRED_DOMAINS.md](SPEC_CLI_EXPIRED_DOMAINS.md)              | Background throttle / CLI alert classification           |
| [ENVIRONMENT.md](../ENVIRONMENT.md)                                                                                            | GoDaddy credentials + `SKIP_BACKGROUND_REGISTRAR_LOOKUP` |
| [KEYBOARD_SHORTCUTS.md](../KEYBOARD_SHORTCUTS.md)                                                                              | `**u**` / **Ctrl+P**                                     |
| [ADDING_A_REGISTRAR.md](../ADDING_A_REGISTRAR.md)                                                                              | How to implement Namecheap / other sources               |


