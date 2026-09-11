# Adding a domain registrar source

How to plug in a new registrar availability client (e.g. Namecheap) next to GoDaddy.

Behaviour contract and TLD routing live in [SPEC_REGISTRAR_LOOKUP.md](specs/SPEC_REGISTRAR_LOOKUP.md). This doc is the **implementation checklist**.

## What you are adding

Each registrar is a **source id** string (lowercase, e.g. `godaddy`, `namecheap`).

| Layer | Role |
|-------|------|
| `domain_lookups.json` | Which sources run for which TLD |
| `registrarcheck` | HTTP client + classify → `purchaseable` / `weirdResponse` |
| `data.Domain.RegistrarLookups[source]` | Persist latest result |
| `registrarrefresh` / UI / `cmd/checkdomains` | Already walk configured sources — usually **no** change once `Lookup` works |

Until a source is implemented, listing it in config still works: the dispatcher logs  
`ERROR registrar <source> <host>: not implemented` and skips persist for that source.

## Tri-state contract (required)

Every successful classification must set both fields to one of `yes` | `no` | `weird`:

| Scenario | `purchaseable` | `weirdResponse` |
|----------|----------------|-----------------|
| Clean available | `yes` | `no` |
| Clean unavailable | `no` | `no` |
| Hard failure (HTTP error, auth, timeout, bad JSON, missing field) | `weird` | `yes` |
| Ambiguous 200 (wrong types, contradictory fields) | `weird` | `weird` |

Rules:

- Never set `purchaseable=yes` unless `weirdResponse=no`.
- Always set `checkedAt` when persisting (handled by `Domain.UpdateRegistrarSource`).
- Unimplemented sources return `registrarcheck.ErrNotImplemented` and **must not** write a map entry.

Alerts (already wired):

- `registrar-weird` — any source with `purchaseable=weird` or `weirdResponse` in `{yes,weird}`
- `registrar-purchaseable` — any source with `purchaseable=yes`

## Step-by-step

### 1. Choose a source id

- Lowercase, stable JSON key: `namecheap`, `porkbun`, …
- Add a constant next to `SourceGoDaddy` / `SourceNamecheap` in `registrarcheck/registrarcheck.go`.
- Add a pretty label in `SourceLabel` (Domain Information panel).

### 2. Enable it in `IsImplemented` + `LookupWith`

In `registrarcheck/registrarcheck.go`:

1. Return `true` from `IsImplemented` for the new id.
2. Add a `case` in `LookupWith` that calls your `lookupYourRegistrar(...)`.

Until both are done, force paths still ERROR+skip for that id.

### 3. Implement the client

Add a file such as `registrarcheck/namecheap.go` mirroring `godaddy.go`:

1. Normalize hostname with the shared `clean` helper (strip one leading `www.`, lowercase).
2. Read credentials from env (document them in [ENVIRONMENT.md](ENVIRONMENT.md)).
3. Prefer an injectable `*http.Client` (and credential func) for unit tests — same pattern as GoDaddy’s `LookupWith`.
4. Call the registrar’s **availability** API only (no purchase/cart).
5. Map the response through the tri-state matrix above into `registrarcheck.Info`.
6. On missing credentials: `Purchaseable=weird`, `WeirdResponse=yes`, `Message=…`, return an error (persist still happens via `UpdateRegistrarSource`).

Suggested `Info` fill:

```go
info := Info{
    Source:        SourceNamecheap,
    Hostname:      hostname,
    Purchaseable:  TriYes, // or TriNo / TriWeird
    WeirdResponse: TriNo,  // or TriYes / TriWeird
    Message:       "",     // error / weird detail when not clean
}
```

### 4. Unit tests (no live network)

In `registrarcheck/*_test.go`, table-drive at least:

- available → `yes` / `no`
- non-2xx / empty body / bad JSON → `weird` / `yes`
- ambiguous body → `weird` / `weird`
- missing credentials → `weird` / `yes`
- apex cleaning (`www.example.com` → `example.com`)

Use a fake `http.RoundTripper` like the GoDaddy tests.

### 5. Wire config

Edit **`domain_lookups.json`** (and any docs/examples):

```json
{
  "com.au": ["godaddy"],
  "com": ["godaddy", "namecheap"]
}
```

- Longest TLD suffix wins (`com.au` before `com`).
- Order in the array is run order.
- You can ship a source id before the client exists (ERROR stub); once implemented, remove the stub behaviour by completing steps 2–4.

Reload is automatic each scan / `u` / CLI run — no code change for hot reload.

### 6. Env + docs

1. Document new env vars in [ENVIRONMENT.md](ENVIRONMENT.md).
2. Mention them briefly in the root [README.md](../README.md) near `checkdomains` / registrar notes if operators need them.
3. Optional: note the source in [SPEC_REGISTRAR_LOOKUP.md](specs/SPEC_REGISTRAR_LOOKUP.md) “Related” / acceptance notes.

### 7. What you usually do **not** need to touch

These already iterate configured sources:

| Package / path | Why |
|----------------|-----|
| `data.Domain.UpdateRegistrarSource` | Persists any source key |
| `data.AlertReasons` | Reads all map entries |
| `registrarrefresh.Run` | Loads config, loops sources |
| `ui` background / Ctrl+P / `u` / domain-added | Calls `registrarrefresh` |
| `cmd/checkdomains` | Same loop; `--dry-run` uses persisted map |
| `ui/panel.DrawWhoisPanel` | Uses `SourceLabel` + stored tri-state |

Only change those if the new registrar needs a different freshness rule, panel layout, or alert reason.

## Checklist

- [ ] Source id constant + `SourceLabel`
- [ ] `IsImplemented` → true
- [ ] `LookupWith` dispatches to new client
- [ ] Client classifies to tri-state correctly
- [ ] Injectable HTTP client + unit tests
- [ ] Env vars in `ENVIRONMENT.md`
- [ ] `domain_lookups.json` lists the source on the right TLDs
- [ ] Manual: `u` on a matching domain; Domain Information shows `Name: purchaseable=…  weird=…`
- [ ] Manual / CI: `go test ./registrarcheck/ ./registrarrefresh/ ./data/`

## Reference layout

```text
domain_lookups.json          # TLD → [source, …]
registrarcheck/
  registrarcheck.go          # TriState, IsImplemented, LookupWith, SourceLabel
  godaddy.go                 # example client
  godaddy_test.go
  <new>.go                   # your client
data/domains.go              # RegistrarLookups + alerts
registrarrefresh/            # batch runner (shared)
docs/specs/SPEC_REGISTRAR_LOOKUP.md
```

## Namecheap (current stub)

`namecheap` is already a known id (`SourceNamecheap`) and appears under `"com"` in the shipped `domain_lookups.json`. Completing Namecheap is exactly steps 2–6 above: implement `lookupNamecheap`, flip `IsImplemented`, add tests and `NAMECHEAP_*` (or whatever) env docs.
