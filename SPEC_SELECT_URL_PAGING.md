# Spec: Select a URL search paging (v2)

Extends [SPEC_SELECT_URL_MODAL.md](SPEC_SELECT_URL_MODAL.md). v1 covers the Select a URL modal UI; this v2 adds **paged search** so the user can load the next (~10) results without leaving the guess-domain flow.

## Goal

From the Select a URL modal, move to the **previous / next page** of **Bing** search results (about 10 links per page). While a page fetch runs, show and focus the existing **Searching** modal (`SEARCHING_MODAL`). On success, reopen Select a URL with that page’s links. Log failures (and other notable issues) to the console log panel. DuckDuckGo is out of scope for this paging work.

## Current behaviour (baseline)

| Aspect | Today |
| ------ | ----- |
| Search | `UrlSearcher.Search(term)` → DDG then Bing; first page only |
| UI flow | Searching modal → Select a URL with whatever links came back |
| State | `SearchState{ term, result }`; no page index |
| Status bar | `enter: Add Domain, c: Copy Link` |
| Engines | `duckduck.Go(term)` / `bing.Go(term)` — no offset / `first` / `s` parameter |

## Desired behaviour

### 1. Page model

- **Page size:** ~10 results per Bing response (do not invent a second client-side slice unless Bing returns more than one page in one response).
- **Page index:** 0-based integer on `SearchState` (e.g. `page int`). Ctrl+G starts at page `0`.
- **Replace, don’t append:** each successful page load replaces `searchState.result` with that page’s links (modal shows only the current page).
- **Display numbering:** list indices may stay `1…N` for the **current page** (v1 style), or show global numbers `(page*10)+i` — pick one and keep it consistent; prefer **per-page 1…N** for simplicity unless global numbers are easy.

### 2. Keybindings (Select a URL modal)

| Key | Action |
| --- | ------ |
| **→** (Right) | Request **next** page (`page + 1`) |
| **←** (Left) | Request **previous** page (`page - 1`) if `page > 0` |

- Scope: `LIST_URLS_MODAL` only (same registration style as Enter / `c`).
- **←** on page `0`: do **not** search; log a short console message (e.g. already on first page) and keep the current modal focused.
- **→** when the next page returns no links / hard failure: see failure behaviour below; do not leave the user stuck without feedback.

Update [KEYBOARD_SHORTCUTS.md](KEYBOARD_SHORTCUTS.md) Select a URL section.

### 3. Status bar

Extend the Select a URL status line to include paging hints, e.g.:

```text
enter: Add Domain, c: Copy Link, ←/→: Page
```

or, if width allows:

```text
enter: Add Domain, c: Copy Link, ←: Prev page, →: Next page
```

Keep a single status line if possible (modal is wide in v1). Page number belongs in the **modal title**, not the status bar (see below).

### 3b. Modal title

Set the Select a URL view title to include the **1-based** page number:

```text
Select a URL (Page 1)
Select a URL (Page 2)
…
```

- Derived from `searchState.page + 1` whenever `toggleListUrlsModal` opens the modal.
- Update `v.Title` (and the stored `panelViews` title if that drives redraws) so paging reopens show the correct page.

### 4. Pagination UI flow

When **←** / **→** triggers a real fetch:

1. Close **Select a URL** (`LIST_URLS_MODAL`).
2. Open **Searching** (`SEARCHING_MODAL`) and make it the focused / current modal (same pattern as initial Ctrl+G → `toggleSearchingModal` / `SearchAndDisplay`).
3. Run the paged search **off the gocui main loop** (goroutine), same as today’s `SearchAndDisplay`.
4. On completion, `g.Update(...)`:
   - Close Searching.
   - On success with ≥1 link: set `searchState` (term, page, result) and **reopen** Select a URL (`toggleListUrlsModal`), cursor on first item.
   - On failure / empty: log to console; reopen Select a URL with the **previous** page’s results if still held, **or** leave Searching closed and restore focus to the list/domain panel with a clear error — prefer **reopen Select a URL with prior results** when prior links are still in memory so the user can keep browsing.

Do not leave Searching open after the fetch finishes (success or failure), except transiently during the request.

### 5. Search API (Bing only)

Paging uses **Bing only**. Do not add DuckDuckGo offsets or DDG fallback for ← / → in v2.

- Suggested shape: `bing.Go(term string, page int) ([]Link, error)` and/or `UrlSearcher.SearchPage(term string, page int)` that **only** calls Bing, with page `0` = first results.
- **Bing offset:** `first = page*10 + 1` (i.e. `first=1`, `first=11`, `first=21`, …).
- Wire Ctrl+G’s shared “search then show modal” helper through this Bing paged API for page `0` as well, so ← / → stay on the same engine and result continuum. Leave existing DDG code in place but **unused for paging** (and fine to skip for this flow entirely in v2).
- No `SearchState.engine` field required while Bing is the sole paging backend.

### 6. Logging (console)

Use the existing console / `ui.log` paths. Log when useful; avoid spam.

| Event | Level |
| ----- | ----- |
| Page fetch starts | optional DEBUG/INFO (`Searching Bing page N for …`) |
| Page fetch success | INFO (`Found K links for … (page N)`) |
| Page fetch error | **ERROR** (network, parse, empty Bing page, etc.) |
| Empty next page | **ERROR** or INFO (`No links on page N`) — must be visible |
| ← on first page | INFO (`Already on first page`) |
| Could not reopen Searching / Select a URL | **ERROR** |

Errors must never be silent: if pagination fails, the user sees a console line explaining why.

### 7. State sketch

```text
SearchState:
  term   string
  page   int              // 0-based; Bing first = page*10+1
  result *[]searching.Link
```

Initial Ctrl+G: `page = 0` via Bing, then Searching → results → Select a URL.

## Interaction summary (Select a URL, after v2)


| Key | Action |
| --- | ------ |
| **↑** / **↓** | Previous / next item (v1) |
| **Enter** | Add hostname; close modal (v1) |
| **c** | Copy full URL (v1) |
| **←** | Previous Bing page (noop + log on page 0) |
| **→** | Next Bing page |
| **Ctrl+C** | Close modal without quitting |

While paginating: **Searching** is visible and focused until the fetch completes.

## Files likely touched

- `searching/bing.go` — `first` query param from page index
- `searching/url.go` — Bing-only paged entry point used by the UI helper
- `ui/search.go` — `SearchState.page`; shared “search page then show modal” helper used by Ctrl+G and ←/→
- `ui/toggleModal.go` — reopen Searching with focus during pagination; set title `Select a URL (Page N)`
- `ui/handlers.go` / `ui/urlList.go` — ← / → handlers
- `ui/panel/listUrlPanels.go` — status bar text
- `KEYBOARD_SHORTCUTS.md` — document ← / →

## Out of scope (v2)

- DuckDuckGo paging or DDG-then-Bing fallback for ← / →
- Infinite scroll inside one modal without Searching
- Caching all visited pages for instant back without re-fetch (nice-to-have; v2 may re-fetch on ←)
- Changing page size away from “one Bing response (~10)”
- Parallel multi-engine merge for a single page
- Mouse-driven paging

## Acceptance criteria

- [x] Status bar mentions ← / → (or Left/Right) for paging
- [x] Modal title is `Select a URL (Page N)` with 1-based `N` matching `searchState.page`
- [x] **→** closes Select a URL, opens/focuses Searching, fetches next ~10 **Bing** results, then reopens Select a URL on success
- [x] **←** does the same for the previous Bing page when `page > 0`
- [x] **←** on page 0 does not search; logs to console; modal stays usable
- [x] Search errors and empty pages are logged to the console
- [x] Failed page fetch does not leave the UI wedged on Searching with no message
- [x] `KEYBOARD_SHORTCUTS.md` documents the new bindings
- [x] Unit coverage for Bing `first` URL/offset construction and/or page-index helpers
- [x] No DDG calls on the pagination path