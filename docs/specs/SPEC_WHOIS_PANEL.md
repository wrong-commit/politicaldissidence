# Spec: WHOIS information panel

## Goal

Replace the unused **Logo Panel** with a **WHOIS information** panel that shows the **latest** WHOIS lookup for the **currently selected domain** in Member Domains. The latest record is **persisted on each domain** in `mp_data.json` (one record per domain, overwritten on each lookup).

Primary population path: the **startup domain scan** (and other WHOIS paths: Ctrl+U, domain-added).

## Desired behaviour

### 1. WHOIS Panel (replaces Logo)

- Constant `WHOIS_PANEL` (view `"whois"`), title `"WHOIS information"`.
  - **Superseded for display title:** [SPEC_DNS_EMPTY.md](SPEC_DNS_EMPTY.md) renames the visible title to `"Domain Information"` (constant/view id unchanged in that v1).
- Same layout as former Logo Panel (right third, top ~60%).
- In `MainViews`; **not** in `tabViews` (display-only).

### 2. Persist latest WHOIS on the domain

Each `data.Domain` may carry:

```json
"whois": {
  "checkedAt": "...",
  "status": ["..."],
  "created": "...",
  "updated": "...",
  "expiry": "...",
  "registrar": "...",
  "nameServers": ["..."],
  "error": "..."
}
```

Rules:

- **Only the latest** record is kept (overwrite on each lookup attempt).
- Stored under the domain; written with normal MP saves (`db.WriteMps`, including end of startup scan).
- `checkedAt` is the wall clock when that lookup ran.
- On **success**: also update `Expiry` / `LastChecked` as today; fill WHOIS summary fields; clear `error`.
- On **failure**: leave `Expiry` / `LastChecked` unchanged; store `whois` with `checkedAt` + `error`.
- Domains never looked up omit `whois` (`omitempty`).

### 3. Panel content

First line is the check time when present:

```text
Checked: 26-09-08 15:04
example.com.au
MP: Jane Doe
...
```

Empty / no record: `No WHOIS lookup yet`.

### 4. Follow Member Domains selection

- Panel always shows WHOIS for the **selected row** in Member Domains.
- Cycling domains (`↑`/`↓` / `selectDomain`) and changing MP (`selectMp`) redraws the WHOIS panel for the new selection.
- Background lookups refresh the panel when done (selected domain’s stored record is what is drawn).

### 5. Out of scope

- WHOIS history (multiple records per domain)
- Focusing / editing the WHOIS panel
- Changing background throttle / env skip ([SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md))
