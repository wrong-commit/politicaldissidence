# Spec: WHOIS information panel

## Goal

Replace the unused **Logo Panel** (`LOGO_PANEL` / view name `logo`) with a **WHOIS information** panel that shows the **latest** WHOIS lookup details from the session. Data is **display-only** (in-memory); it must **not** be written to `mp_data.json`.

Primary population path: the **startup domain scan** (session-start background WHOIS / `startBackgroundWhois` → `refresh.Runner.TryRun` / `refresh.Run`).

## Current behaviour

| Aspect | Today |
| ------ | ----- |
| Right-hand panel | `LOGO_PANEL` — title `"Logo Panel"`, placeholder text (selected MP name + filter) |
| Layout | Same geometry as list/domain: `x1=2/3`, `y1=0`, `x2=1`, `y2=0.6` (`ui/panels.go`) |
| Tab focus | Not in `tabViews` (only `LIST_PANEL` / `DOMAIN_PANEL`); Logo is display-only |
| WHOIS result used | `whois.GetExpiry` → expiry string only; persisted on `data.Domain` (`Expiry`, `LastChecked`) |
| Startup scan | `UI.startBackgroundWhois` after load; updates domains + log panel; redraws domain panel |
| Rich WHOIS | Parsed fields beyond expiry (status, registrar, NS, created/updated) are discarded after `GetExpiry` |

Relevant code: `ui/panels.go`, `ui/whoisRefresh.go`, `refresh/refresh.go`, `whois/whoisLookup.go`, `data/domains.go`. Background job behaviour remains as in [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md).

## Desired behaviour

### 1. Replace Logo Panel with WHOIS Panel

- Rename / replace the constant and view:
  - Constant: e.g. `WHOIS_PANEL` (view name `"whois"` — or keep `"logo"` only if renaming views is painful; prefer a clear name).
  - Title: `"WHOIS information"` (or `"WHOIS Information"`).
- Keep the **same layout rectangle** as today’s Logo Panel (right third, top ~60%).
- Include it in `MainViews` in place of `LOGO_PANEL`.
- Do **not** add it to `tabViews` (v1 stays non-focusable, like Logo).
- Remove Logo-specific content (MP name / filter dump). Update stray comments that mention hiding `logo_panel` (e.g. `toggleListUrlsPanel`) if they still refer to Logo.

### 2. Session-only “latest WHOIS” state

Hold a single in-memory snapshot of the most recent lookup, e.g. on `UI` / `State`:

| Field | Purpose |
| ----- | ------- |
| Hostname | Domain that was looked up |
| MP name (optional) | Context from the scan (`FormatDebug` already has MP + hostname) |
| Looked-up-at | Wall clock when this snapshot was taken |
| Summary fields | See §3 |
| Error (optional) | Non-empty when the latest attempt failed |

Rules:

- **Not** part of `data.Domain` / `data.MP` JSON tags.
- **Not** passed to `db.WriteMps` / atomic save.
- Cleared on process exit (no disk). Reload / restart starts empty until the next lookup.
- “Latest” means **overwrite on every completed lookup attempt** that participates in the population path (success or failure), so the panel always reflects the most recent scan activity.

### 3. What to show

Render a short, readable multi-line summary (not the full raw WHOIS blob unless truncated for debug later). Prefer fields already available from `github.com/likexian/whois-parser` after parse:

```text
example.com.au
MP: Jane Doe
Checked: 26-09-08 15:04

Status: clientTransferProhibited
Created: 2019-01-02
Updated: 2025-01-02
Expiry: 2027-01-01

Registrar: Example Registrar Pty Ltd
Name servers:
  ns1.example.net
  ns2.example.net
```

Guidelines:

- Omit empty lines / blank fields rather than printing `""`.
- On failure, show hostname + error message (align with existing `Could not get WHOIS` / `Could not parse` wording where possible).
- Empty state before any lookup: a single short line, e.g. `No WHOIS lookup yet`.
- Domain panel continues to own the persisted expiry / last-checked list; this panel is the **detail view for the latest lookup**, not a duplicate of the domain list.

Exact line order can be tuned; keep it scannable in a ~1/3-width column.

### 4. Populate during startup domain scan

Wire snapshot + panel redraw into the same path that already runs WHOIS at session start:

1. Extend the WHOIS seam so a lookup can yield **display fields**, not only an expiry string. Options (pick one; keep tests injectable):
   - Add e.g. `whois.Lookup` / `GetInfo` that returns expiry + parsed summary (and keep `GetExpiry` as a thin wrapper), **or**
   - Have `UpdateExpiry` (or `refresh.Deps.UpdateExpiry`) accept a callback / return richer result used only by the UI.
2. On each domain the startup scan actually attempts (not 10-day skips): after the lookup completes, update the in-memory latest snapshot.
3. Redraw `WHOIS_PANEL` via `g.Update(...)` (same thread-safety pattern as `refreshDomainPanel` / log lines). Prefer redrawing after each lookup so the panel “ticks” during a long startup scan; at minimum redraw when the run finishes with the last snapshot.
4. Skipped (fresh) domains do **not** change the WHOIS panel.
5. If `SKIP_BACKGROUND_WHOIS_LOOKUP=true`, the panel stays at empty state unless another path updates it (see §5).

Persisted side effects of the startup scan (`Expiry`, `LastChecked`, end-of-run save) are **unchanged**. Only the new display snapshot is additive and non-persisted.

### 5. Other WHOIS paths (recommended, same session)

So “latest” stays meaningful after the user acts:

| Path | Update WHOIS panel? |
| ---- | ------------------- |
| Startup background scan | **Required** |
| Manual check (`U` / `checkDomain`) | Recommended |
| Domain-added WHOIS job | Recommended |

If deferred, document as follow-up; v1 acceptance only requires the startup scan.

### 6. Out of scope (v1)

- Persisting raw or parsed WHOIS into `mp_data.json`
- Focusing / editing the WHOIS panel (Tab cycle, cursor)
- Showing WHOIS for the **selected** domain when selection changes without a new lookup (panel tracks **latest lookup**, not selection)
- Full raw WHOIS dump / scrollable history of all lookups this session
- Changing background throttle, delay, or env skip behaviour ([SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md))

## Acceptance criteria

- [ ] Logo Panel is gone; right-hand main panel is titled WHOIS information (same layout)
- [ ] Panel shows empty state until a lookup runs
- [ ] During / after startup domain scan, panel shows fields from the most recent attempted WHOIS (success or failure)
- [ ] Skipped (fresh) domains do not overwrite the panel
- [ ] No new WHOIS detail fields appear in `mp_data.json` after a scan or save
- [ ] Existing domain expiry / `lastChecked` persistence and domain-panel display still work
- [ ] TUI remains usable while the startup scan updates the panel

## Implementation notes

- Touch points: `ui/panels.go` (`panelViews`, `MainViews`, `createPanelView` switch), optional `ui/panel/whoisPanel.go` draw helper, `ui/ui.go` / `State` for snapshot, `ui/whoisRefresh.go` + `refresh` / `whois` for capturing info during `Run`.
- Prefer a pure `DrawWhoisPanel(snapshot) string` for easy unit tests without gocui.
- Reuse date formatting style already used in the domain panel / console where practical.

## Unit test plan (lightweight)

| ID | Case | Expect |
| -- | ---- | ------ |
| P1 | Empty snapshot | Draw helper returns the empty-state string |
| P2 | Success snapshot | Draw includes hostname + expiry (and omits blank optional fields) |
| P3 | Failure snapshot | Draw includes hostname + error text |
| P4 | Refresh hook | When a lookup runs under Force/background, snapshot is replaced; skipped domain does not |
| P5 | Persistence | After refresh + save, JSON domain objects still only have existing fields (no whois blob / registrar / NS arrays) |

Live network WHOIS remains out of scope for unit tests (fixtures / injected fetcher, same as [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md) §1).
