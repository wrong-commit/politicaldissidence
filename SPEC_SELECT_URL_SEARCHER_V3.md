# Spec: Select a URL searcher controls (v3)

Extends [SPEC_SELECT_URL_PAGING.md](SPEC_SELECT_URL_PAGING.md) (v2) and [SPEC_SELECT_URL_MODAL.md](SPEC_SELECT_URL_MODAL.md) (v1). v2 added Bing paging; this v3 adds **search-engine** and **search-term** toggles from the Select a URL panel, plus a fuller status bar.

## Goal

From the Select a URL modal, let the user:

1. **Toggle the search engine** (Bing ↔ DuckDuckGo) and keep that choice for later searches in the same app session.
2. **Toggle between two fixed search terms** (`SearchTerm1` / `SearchTerm2` on `data.MP`).
3. See **all relevant keybindings** in the status bar, including the missing **↑** / **↓** hints.

For v3, terms are only these two presets — no free-form query editor.

## Current behaviour (baseline)

| Aspect | Today |
| ------ | ----- |
| Engine | Ctrl+G / ← / → use Bing only via `UrlSearcher.SearchPage` |
| Term | Always `mp.SearchTerm()`; `SearchTerm2()` exists on `MP` but is unused by the UI |
| Persistence | `SearchState` holds `term`, `page`, `result`; closing the modal clears them via `resetURLSearchState` |
| Status bar | `enter: Add Domain, c: Copy Link, ←/→: Page` — no ↑/↓, engine, or term hints |
| Engines in code | `bing.Go(term, page)` (paged); `duckduck.Go(term)` (first page only) |

## Desired behaviour

### 1. Search engine toggle

- **Engines:** Bing and DuckDuckGo (the two backends already in `searching/`).
- **Keybinding (Select a URL):** **`e`** — cycle engine (Bing → DuckDuckGo → Bing).
- **On toggle:** treat like a fresh search for the **current term** at **page 0**:
  1. Close Select a URL.
  2. Open / focus Searching.
  3. Fetch with the new engine.
  4. On success, reopen Select a URL with the new results; on failure, restore prior results when possible (same pattern as v2 paging failure).
- **Retain preference:** store the last-used engine on UI session state (not cleared by `resetURLSearchState` when dismissing Select a URL). Subsequent **Ctrl+G** and ← / → use that engine until the user toggles again (or the process exits).
- **Default:** Bing (matches v2 behaviour) until the user presses **`e`**.
- **Paging with DuckDuckGo:** if DDG has no page offset yet, either:
  - add a minimal DDG page parameter / offset in the same change, **or**
  - keep ← / → Bing-only and, while DDG is selected, log that paging is unavailable / no-op on ← / →.

  Prefer wiring `UrlSearcher.SearchPage(term, page, engine)` (or equivalent) so Ctrl+G, ← / →, **`e`**, and term toggle all share one fetch path. Leave the old DDG-then-Bing `Search()` fallback for non-UI callers if still useful.

### 2. Search term toggle (`SearchTerm1` / `SearchTerm2`)

#### Data (`data/mp.go`)

- Rename **`SearchTerm()` → `SearchTerm1()`** (update all call sites).
- Keep **`SearchTerm2()`** as the alternate query string.
- For v3 the UI only chooses between these two methods; do not add more terms or a custom string field yet.
- Exact string formulas may stay as today (or be tightened later); the important part is two distinct named options the UI can switch between.

#### UI

- **Keybinding (Select a URL):** **`t`** — toggle active term between term 1 and term 2.
- **On toggle:** re-fetch with the **new term**, **same engine**, **page 0** (Searching modal flow as above). Do not keep the old page index for a different query.
- **Retain preference:** which term index (1 vs 2) is active should persist for subsequent Ctrl+G in the same session (same idea as engine), applied to whatever MP is selected when search starts.
- **Default:** term 1 (`SearchTerm1`).
- **State:** e.g. `termIndex int` (`1` or `2`) on session search prefs; `SearchState.term` remains the concrete string last fetched (for paging / logging).

When starting Ctrl+G for an MP:

```text
term = mp.SearchTerm1()  if preferred term index == 1
term = mp.SearchTerm2()  if preferred term index == 2
engine = last preferred engine (default Bing)
page = 0
```

### 3. Status bar

Update the Select a URL status line so it documents navigation and the new toggles. Include the missing **↑** / **↓**.

Suggested single-line wording (trim if the terminal is narrow):

```text
↑/↓: Move, enter: Add Domain, c: Copy Link, ←/→: Page, e: Engine, t: Term
```

Optional short current-state suffix if it fits without wrapping badly, e.g. `(Bing · T1)` / `(DDG · T2)`. Prefer putting rich “current engine / term” detail in the **modal title** or a console INFO log on toggle if the status line is too crowded.

Rules from v1 still apply: status line is not selectable; ↑ from the first item stays on the first item.

### 3b. Modal title (optional but useful)

Extend the title beyond page number when cheap, e.g.:

```text
Select a URL (Page 1 · Bing · T1)
```

Minimum for v3 acceptance: page number remains correct (v2). Engine / term in the title is nice-to-have if status bar already shows them.

### 4. State sketch

```text
// Session prefs — survive closing Select a URL / resetURLSearchState
SearchPrefs (or fields on UI):
  engine    bing | duckduckgo     // default bing
  termIndex 1 | 2                 // default 1

SearchState (per open search / paging continuum):
  term   string                   // concrete query last used
  page   int                      // 0-based
  result *[]searching.Link
  // optionally mirror engine for logging; source of truth for next fetch is SearchPrefs
```

`resetURLSearchState` clears `term` / `page` / `result` only — **not** engine or termIndex.

### 5. Keybindings (Select a URL modal)

| Key | Action |
| --- | ------ |
| **↑** / **↓** | Previous / next item (v1) |
| **Enter** | Add hostname; close modal (v1) |
| **c** | Copy full URL (v1) |
| **←** / **→** | Previous / next page (v2; engine-aware per §1) |
| **e** | Toggle search engine; re-fetch page 0 |
| **t** | Toggle SearchTerm1 ↔ SearchTerm2; re-fetch page 0 |
| **Ctrl+C** | Close modal without quitting |

Scope: register **`e`** and **`t`** on `LIST_URLS_MODAL` like Enter / `c` / ← / →.

Ignore **`e`** / **`t`** while Searching is already open (same “search already in progress” guard as paging).

Update [KEYBOARD_SHORTCUTS.md](KEYBOARD_SHORTCUTS.md) Select a URL section.

### 6. Logging (console)

| Event | Level |
| ----- | ----- |
| Engine toggled | INFO (`Search engine → Bing` / `→ DuckDuckGo`) |
| Term toggled | INFO (`Search term → T1` / `→ T2`, optionally include the term string) |
| Fetch starts (any path) | optional INFO (`Searching {engine} page N for …`) |
| Fetch success / error / empty | same expectations as v2 |
| ← / → while engine cannot page | INFO (explain no-op) |

### 7. Interaction summary (Select a URL, after v3)

| Key | Action |
| --- | ------ |
| **↑** / **↓** | Previous / next item |
| **Enter** | Add hostname; close modal |
| **c** | Copy full URL |
| **←** / **→** | Page (when supported for current engine) |
| **e** | Toggle Bing ↔ DuckDuckGo; Searching → page 0 results |
| **t** | Toggle term 1 ↔ term 2; Searching → page 0 results |
| **Ctrl+C** | Close modal without quitting |

Ctrl+G uses the last preferred engine and term index.

## Files likely touched

- `data/mp.go` — rename `SearchTerm` → `SearchTerm1`; keep `SearchTerm2`
- `searching/url.go` — engine-aware `SearchPage` (or sibling API); optional DDG paging
- `searching/duckduckgo.go` — page/offset support if ← / → must work under DDG
- `ui/search.go` — prefs; wire Ctrl+G / paging / toggles through shared fetch helper
- `ui/handlers.go` — **`e`** / **`t`** handlers
- `ui/panel/listUrlPanels.go` — status bar text (+ tests)
- `ui/toggleModal.go` — optional title including engine / term
- `KEYBOARD_SHORTCUTS.md` — document **e**, **t**, and status-bar wording

## Out of scope (v3)

- Free-form / editable custom search query
- More than two term presets
- Persisting engine / term choice to disk across process restarts
- Parallel multi-engine merge for one page
- Changing Bing page size
- Mouse-driven toggles

## Acceptance criteria

- [ ] From Select a URL, **`e`** switches Bing ↔ DuckDuckGo, shows Searching, then reopens with page-0 results for the current term
- [ ] Last-used engine is reused for the next Ctrl+G (and for ← / → when that engine supports paging) after closing the modal
- [ ] `MP.SearchTerm1()` and `MP.SearchTerm2()` exist; UI **`t`** toggles between them and re-fetches page 0
- [ ] Last-used term index is reused for the next Ctrl+G
- [ ] Status bar includes **↑/↓** plus existing and new hints (engine / term)
- [ ] Toggles are blocked or no-op with a clear message if a search is already in progress
- [ ] Failures follow v2: console log + reopen prior results when available; do not wedge on Searching
- [ ] `KEYBOARD_SHORTCUTS.md` documents **e**, **t**, and the updated status line
- [ ] Call sites updated after `SearchTerm` → `SearchTerm1` rename
