# Spec: Custom search terms

Extends [SPEC_SELECT_URL_SEARCHER_V3.md](SPEC_SELECT_URL_SEARCHER_V3.md). v3 hardcodes two Go methods (`MP.SearchTerm1` / `SearchTerm2`) and toggles them with **`t`**. This spec replaces that fixed pair with a **config-driven list of query templates** rendered from `data.MP` fields, loaded when the guess-URL search flow opens, cycled from Select a URL with the **same ← / → fetch flow** used today for result paging, and remembered in session state.

## Goal

1. Ship a config file of search-query **templates** that interpolate fields from `data.MP`.
2. **Load** that config when opening the search / Select a URL flow.
3. Let the user **cycle** among rendered terms while Select a URL is open (term shown in the panel).
4. **Remember** the last chosen term index for the rest of the app session.
5. Drive term changes with **← / →**, using the same Searching → re-fetch page 0 → reopen Select a URL pattern that result paging uses today.

Result-page navigation must move to different keys so ← / → are free for terms (see §5).

## Motivation

Only two baked-in strings (`SearchTerm1` / `SearchTerm2`) limits experimentation (honorific vs preferred name, electorate-only, party + state, etc.) without a rebuild. A small JSON template list next to the app matches the `csv_refresh.json` pattern and keeps query design editable.

## Current behaviour (baseline)

| Aspect | Today |
| ------ | ----- |
| Terms | `MP.SearchTerm1()` / `SearchTerm2()` only |
| Switch | **`t`** toggles index 1 ↔ 2; re-fetch page 0 |
| ← / → | Previous / next **result page** (Bing / DDG paging) |
| Prefs | `SearchPrefs.termIndex` is `1` or `2`; survives `resetURLSearchState` |
| Panel | Status line + `Search: {term}` under it (`DrawListUrlPanel`) |
| Title | `Select a URL ({engine} · T{n})` |

## Desired behaviour

### 1. Config file (`search_terms.json`)

Path: **`search_terms.json`** next to the app working directory (same convention as `csv_refresh.json`). Do **not** hardcode query strings in UI code once config loads successfully.

#### Schema

```json
{
  "terms": [
    {
      "id": "honorific-electorate-party",
      "label": "Honorific + electorate + party",
      "template": "{{.NameWithHonorific}} member for {{.Electorate}} {{.Party}}"
    },
    {
      "id": "name-electorate-party",
      "label": "Name + electorate + party",
      "template": "{{.Name}} member for {{.Electorate}} {{.Party}}"
    }
  ]
}
```

| Field | Required | Notes |
| ----- | -------- | ----- |
| `terms` | yes | Non-empty array |
| `terms[].id` | no | Stable slug for logs; if omitted, use 1-based index as `T1`, `T2`, … |
| `terms[].label` | no | Short human name for console / optional UI; default = `id` or `T{n}` |
| `terms[].template` | yes | Non-empty Go `text/template` string over an MP view model (below) |

#### Template data (`data.MP` fields)

Expose a template root that mirrors `data.MP` plus the existing name helpers so configs can reproduce today’s built-ins:

| Placeholder | Source |
| ----------- | ------ |
| `{{.Honorific}}` | `MP.Honorific` |
| `{{.FirstName}}` | `MP.FirstName` |
| `{{.Surname}}` | `MP.Surname` |
| `{{.OtherName}}` | `MP.OtherName` |
| `{{.PreferredName}}` | `MP.PreferredName` |
| `{{.Electorate}}` | `MP.Electorate` |
| `{{.Party}}` | `MP.Party` |
| `{{.State}}` | `MP.State` |
| `{{.Level}}` | `MP.Level` |
| `{{.Name}}` | `MP.Name()` |
| `{{.NameWithHonorific}}` | `MP.NameWithHonorific()` |

Do **not** expose `Domains` in templates for v1 (not useful for search queries; keeps templates string-only).

Rendering rules:

- Trim outer whitespace on the final string.
- Empty field → empty substitution (no error).
- Invalid template syntax at **load** time → reject that entry / whole file (fail closed with ERROR).
- Execute against the **currently selected** MP when starting or changing a search.

Default file (ship in repo or document as recommended starter) should match today’s two methods so behaviour is unchanged until the user edits the file:

```json
{
  "terms": [
    {
      "id": "t1",
      "label": "T1",
      "template": "{{.NameWithHonorific}} member for {{.Electorate}} {{.Party}} "
    },
    {
      "id": "t2",
      "label": "T2",
      "template": "{{.Name}} member for {{.Electorate}} {{.Party}} "
    }
  ]
}
```

#### Validation

On load:

- JSON must parse.
- `terms` must be non-empty.
- Each `template` must be non-empty and compile with `text/template`.
- Duplicate `id` values → ERROR (or last-wins with WARN — prefer **ERROR** for clarity).

#### Fallback when config is missing / invalid

| Situation | Behaviour |
| --------- | --------- |
| File missing | Fall back to built-in two terms equivalent to `SearchTerm1` / `SearchTerm2`; log INFO once |
| Invalid JSON / empty `terms` / bad templates | Fall back to built-ins; log ERROR; search still works |
| Valid file with N≥1 terms | Use config only; do not merge with hardcodes |

Keep `SearchTerm1` / `SearchTerm2` on `MP` as the built-in fallback implementation (or move those strings into a package-level default config constant). UI should resolve terms through one helper, e.g. `ResolveSearchTerm(mp, index)`, not call the methods directly when config is active.

### 2. Load when opening the search screen

Reload **`search_terms.json`** at the start of the guess-URL flow so edits are picked up without restarting the process:

| Trigger | When to load |
| ------- | ------------ |
| **g** / Ctrl+G (guess domain) | Before resolving the query string for the selected MP |
| Term cycle ← / → from Select a URL | Optional: reuse last-loaded config for the session **or** reload each cycle; prefer **reload once per open of the search flow** (on g) and keep that list until Select a URL is dismissed — avoids mid-modal list length changes |

Recommended approach:

1. On **g**: `LoadFile("search_terms.json")` → store `[]SearchTermDef` (or rendered-ready defs) on UI / search session.
2. Clamp `SearchPrefs.termIndex` into `0 .. len(terms)-1` (or 1-based `1 .. N` — pick one and use it everywhere; prefer **0-based index in code**, **1-based `T{n}` in UI**).
3. Render `terms[i].template` with the selected MP → concrete `SearchState.term`.
4. Proceed with existing Searching → Select a URL path.

Do **not** require a full app restart to pick up config edits; do **not** need to reload on every ← / → if the list was loaded for this open search.

Log on successful load (DEBUG/INFO): `Loaded N search terms from search_terms.json`.

### 3. Show / “scroll” terms in the URL searcher panel

Select a URL already shows the active query on the second header line (`Search: …`). Keep that, and make the active config term obvious:

- **Body:** `Search: {rendered term}` (unchanged layout; content updates when the term changes).
- **Title:** include term position, e.g. `Select a URL (Bing · T2/5)` where `2` is 1-based index and `5` is `len(terms)`.
- Optional: if `label` is set and short, allow `Select a URL (Bing · T2: Name + electorate)` only when it fits; otherwise stick to `T{n}/{N}`.

“Scroll” means **cycle the active term** among the loaded list (not a separate scrollable list widget in v1). ↑ / ↓ remain for moving among **URL results**.

When the rendered term is longer than the modal width, existing gocui wrapping / truncation behaviour is acceptable for v1; do not add a horizontal scroll widget.

### 4. Remember the last search term in state

Extend session prefs (survive closing Select a URL / `resetURLSearchState`):

```text
SearchPrefs:
  engine     bing | duckduckgo
  termIndex  int   // 0-based index into last-loaded terms; default 0
```

Rules:

- After a successful term change (← / →), update `termIndex` and keep it for the next **g** on any MP (re-render that MP with the same template index).
- If the next load has fewer terms than `termIndex+1`, clamp to the last term (or `0`) and log INFO.
- `SearchState.term` remains the **concrete string** last fetched (for paging results / logging).
- Do **not** persist term index to disk across process restarts (same as engine in v3).

Remove the v3 assumption that `termIndex` is only `1` or `2`. Retire **`t`** as a binary toggle (see §5), or keep **`t`** as an alias for → (next term) if useful — prefer removing **`t`** from the status line once ← / → own term cycling, to avoid two ways to do the same thing.

### 5. ← / → choose search terms (same navigation pattern as result paging)

#### Interaction

From Select a URL:

| Key | New action |
| --- | ---------- |
| **→** | Next search term (`termIndex + 1`); **wrap** from last → first |
| **←** | Previous search term (`termIndex - 1`); **wrap** from first → last |

On each real term change:

1. Resolve new template → new query string for the current MP.
2. Close Select a URL.
3. Open / focus Searching.
4. Fetch **page 0** with the **current engine** and new term (same helper as today’s `refetchURLSearch` / `toggleURLSearchTerm`).
5. On success, reopen Select a URL; on failure, restore prior results when possible (v2/v3 failure behaviour).
6. Update title / `Search:` line / console (`Search term → T{n}/{N} <query>`).

Ignore ← / → for terms while Searching is already open (same “search already in progress” guard).

If only **one** term is configured, wrap leaves the index unchanged: **no-op + INFO** (`Only one search term configured`) — do not re-fetch.

#### Result paging must move

Today ← / → page SERP results. After this change they cycle **terms**. Relocate result paging to:

| Key | Action |
| --- | ------ |
| **`[`** | Previous result page (no-op on page 0 + INFO) |
| **`]`** | Next result page |

(Alternative if `[` / `]` are awkward on some layouts: **`n`** / **`p`**. Pick **`[`** / **`]`** unless implementation hits binding issues.)

Update status line, e.g.:

```text
↑/↓: Move, enter: Add Domain, c: Copy Link, ←/→: Term, [/]: Page, e: Engine
```

Update [KEYBOARD_SHORTCUTS.md](../KEYBOARD_SHORTCUTS.md) and Ctrl+H help text.

#### Why reuse the page-nav *flow*

Term changes already need a network round-trip (new query). Reusing close → Searching → fetch → reopen keeps one mental model and one code path (`refetchURLSearch`), differing only in whether `term` or `page` changes.

### 6. State sketch

```text
// Loaded for the open guess-URL continuum (from search_terms.json or built-in fallback)
SearchTermDef:
  id       string
  label    string
  template *template.Template  // compiled

// Session prefs — survive closing Select a URL
SearchPrefs:
  engine    bing | duckduckgo
  termIndex int                 // 0-based; default 0

SearchState:
  term   string                 // concrete rendered query last used
  page   int                    // 0-based result page
  result *[]searching.Link
```

`resetURLSearchState` clears `term` / `page` / `result` only — **not** `termIndex` or `engine`. Optionally clear the in-memory term defs on dismiss, or keep them until the next **g** reload.

### 7. Keybindings (Select a URL modal, after this spec)

| Key | Action |
| --- | ------ |
| **↑** / **↓** | Previous / next URL result |
| **Enter** | Add hostname; close modal |
| **c** | Copy full URL |
| **←** / **→** | Previous / next **search term** (wrap at ends); Searching → page 0 results |
| **`[`** / **`]`** | Previous / next **result page** |
| **e** | Toggle Bing ↔ DuckDuckGo; re-fetch page 0 |
| **Ctrl+C** | Close modal without quitting |

**`t`** removed (or deprecated alias for →). Document the removal in KEYBOARD_SHORTCUTS.

### 8. Logging (console)

| Event | Level |
| ----- | ----- |
| Config loaded | INFO/DEBUG (`Loaded N search terms from search_terms.json`) |
| Config missing / invalid | INFO / **ERROR** then fallback |
| Term cycled | INFO (`Search term → T{n}/{N} <query>`) |
| Only one term + ←/→ | INFO |
| Term index clamped after reload | INFO |
| Fetch start / success / failure | Same as v2/v3 |

### 9. Package / files likely touched

| Area | Work |
| ---- | ---- |
| New e.g. `searchterms/` or under `searching/` | Load / validate / render templates; default built-ins |
| `search_terms.json` | Example config in repo root |
| `ui/search.go` | Load on **g**; `searchTermForMP` from config; ←/→ term cycle; `[`/`]` paging |
| `ui/handlers.go` | Rebind ←/→ to terms; add `[`/`]`; remove or alias **`t`** |
| `ui/panel/listUrlPanels.go` | Status bar text (+ tests) |
| `ui/toggleModal.go` / title helper | `T{n}/{N}` in title |
| `data/mp.go` | Keep `SearchTerm1`/`SearchTerm2` as fallback sources only |
| Docs | This spec; KEYBOARD_SHORTCUTS; README Specs list |

### 10. Out of scope

- Free-form editable query box inside the TUI
- Persisting last term index to disk
- Per-MP remembered term index
- Google as a search **engine** (queries are engine-agnostic; Bing/DDG remain)
- Conditional templates / `if` complexity beyond stock `text/template` (allowed if already free; no requirement)
- Mouse-driven term picker
- Showing a multi-line list of all templates inside the modal

## Acceptance criteria

- [ ] `search_terms.json` defines one or more templates using documented `MP` placeholders; invalid / missing file falls back to today’s two built-in strings with a console message
- [ ] Opening the search flow (**g**) loads (or reloads) the config and renders the preferred term for the selected MP
- [ ] Select a URL shows the rendered term and a clear `T{n}/{N}` (or equivalent) indicator
- [ ] **←** / **→** cycle terms with **wrap at both ends**, show Searching, re-fetch page 0, reopen Select a URL (same pattern as former page nav); single-term list is no-op + INFO
- [ ] Last `termIndex` is kept in `SearchPrefs` across modal close and reused on the next **g**
- [ ] Result paging still works via **`[`** / **`]`** (or the chosen alternate keys)
- [ ] Status bar and KEYBOARD_SHORTCUTS document the new bindings; **`t`** binary toggle is gone or clearly aliased
- [ ] Unit tests for config parse/validate, template render for a sample MP, index clamp after reload, and wrap-at-ends (including single-term no-op)

## Relation to prior specs

| Spec | Still true? |
| ---- | ----------- |
| [SPEC_SELECT_URL_MODAL.md](SPEC_SELECT_URL_MODAL.md) | Yes (list / enter / copy) |
| [SPEC_SELECT_URL_PAGING.md](SPEC_SELECT_URL_PAGING.md) | Paging remains; **keys change** from ←/→ to `[`/`]` |
| [SPEC_SELECT_URL_SEARCHER_V3.md](SPEC_SELECT_URL_SEARCHER_V3.md) | Engine toggle **`e`** remains; fixed T1/T2 + **`t`** superseded by this spec |
