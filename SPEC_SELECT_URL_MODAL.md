# Spec: Select a URL modal improvements

## Goal

Make the **Select a URL** modal (`LIST_URLS_MODAL` / view name `search`) easier to scan and act on after a **Ctrl+G** search: larger canvas, inline key hints, host + path per result, visual cue for domains already on the MP, and a one-key copy for googling the full URL.

## Current behaviour


| Aspect     | Today                                                                                                      |
| ---------- | ---------------------------------------------------------------------------------------------------------- |
| Entry      | `SearchAndDisplay` → `toggleListUrlsModal` after search returns links                                      |
| Render     | `panel.DrawListUrlPanel` → host-only lines via `domainFromUrl` + `renderList`                              |
| Layout     | Modal size ≈ content (`openModal(w, h)`); floor width 20, height ≥ 1                                       |
| List shape | One line per result: `0. example.com` (0-based index)                                                      |
| Selection  | Highlight + cursor y = buffer line; **↑** / **↓** move one line                                            |
| Confirm    | **Enter** parses domain from `strings.Split(line, " ")[2]`, then `addDomain` and closes                    |
| Caps       | Search engines typically return on the order of ~10 links; panel shows whatever is in `searchState.result` |


Relevant code: `ui/panel/listUrlPanels.go`, `ui/toggleModal.go` (`toggleListUrlsModal`), `ui/handlers.go` (`listUrlView` handlers), `KEYBOARD_SHORTCUTS.md` (Select a URL section).

## Desired behaviour



### 1. Larger modal

Open the Select a URL modal with a **large fixed (or near-full) width and height**, not a shrink-wrap to host-only lines.

- Prefer a substantial fraction of the terminal (e.g. ~70–90% of `gui.Size()`, or fixed mins such as width ≥ 60 and height enough for status + ~10 multi-line items). Exact numbers can be tuned; the point is room for status text and two-line items without a tiny floating box.
- Still clamp so the modal stays on-screen (`createModal` centering).
- Content may be shorter than the modal; unused space is fine. If content is taller than the modal, allow scrolling within the view (existing gocui cursor/origin behaviour) rather than growing past the terminal.



### 2. Status bar (top of panel)

First line(s) of the modal buffer are a **status / shortcut hint**, not a selectable result:

```text
enter: Add Domain, c: Copy Link
```

- Always visible at the top of the list buffer (or as a dedicated header line before items).
- Not part of the selectable item list: cursor must never land on the status line; **↑** from the first item stays on the first item.
- Wording may match help / `KEYBOARD_SHORTCUTS.md`; keep it short so it fits the wider modal.



### 3. Multi-line list items

Each search result is rendered as a **three-line item**:

```text
1. foobar.com.au
		/website/sub/directory/opath

```

Rules:

- **Line 1:** index + hostname (display host; strip scheme/`user@`/port the same way `domainFromUrl` does today, or equivalent). Prefer **1-based** indices in the UI (`1.` …) unless keeping 0-based is required for Enter parsing — if indices change, update the Enter handler accordingly.
- **Line 2:** path (and query/fragment if useful) from the original URL, indented (tabs or spaces as in the example). If the URL has no path (or only `/`), show `/` or an empty indented line consistently.
- **Line 3:** blank spacer after the path.
- Full original URL remains available in memory for copy / add (do not discard path when building the list).
- Selection is **per item**, not per buffer line: highlight should cover the whole item (both lines), and **↑** / **↓** move by one **item** (skip the path line and the status bar).

**Enter** must resolve the selected **item index** (from cursor y + layout math, or a side index map), then add the **hostname** for that link (same as today’s add-domain semantics), not re-parse a single buffer line with `Split(...)[2]`.

### 4. Already-added domains (gray)

If the result’s hostname is **already present** on the current MP’s `Domains` list, render that item with a **gray foreground**.

- Compare against `(*ui.state.visible)[ui.state.currentIndex].Domains` (or the same MP the search was run for), case-insensitive host match consistent with how domains are stored.
- Gray applies to the item text (host and path lines). Selection highlight should still be readable (e.g. keep cyan selection background; gray only when not conflicting badly with `SelFgColor`).
- Adding a domain that is already listed may still be allowed (current `addDomain` does not dedupe); gray is a **visual** cue only in v1. Optional later: block Enter or log a warning — out of scope unless easy.

Implementation note: `panel.decorate` in `listDomainPanel.go` is currently a no-op stub; ANSI/`\x1b` colors or gocui attributes are acceptable if they work under termbox. Prefer one shared helper for “muted” text.

### 5. Keybinding: `c` — copy link


| Key   | Action                                                             |
| ----- | ------------------------------------------------------------------ |
| **c** | Copy the **full URL** of the selected item to the system clipboard |


- Scope: `LIST_URLS_MODAL` only (same `listUrlView` registration pattern as Enter / arrows).
- Copy the original link URL (`searching.Link[0]`), not just the hostname — so the user can paste into a browser for googling / verification.
- On success: log a short INFO (e.g. copied URL or “copied to clipboard”); do not close the modal.
- On failure: log an error; leave selection and modal open.
- Clipboard: use a small cross-platform approach suitable for this Go module (e.g. OS clipboard helper / existing CLI). Windows is a primary target for this project.

Update [KEYBOARD_SHORTCUTS.md](KEYBOARD_SHORTCUTS.md) Select a URL section to include **c**.

## Data / API shape (suggested)

Keep `[]searching.Link` as the source of truth. `DrawListUrlPanel` (or a replacement) should:

1. Accept links **and** the current MP’s existing hostnames (or a `map[string]bool` / set) for gray styling.
2. Return buffer text, preferred modal width/height (or let the modal layer pick large defaults and ignore shrink-wrap width).
3. Expose enough structure for handlers to map `cursorY` → item index (e.g. status uses `S` lines; each item uses `L` lines → index = `(cy - S) / L`).

Do not strip path before handlers need it; host extraction stays for display + `addDomain`, full URL for copy.

## Interaction summary (after change)


| Key           | Action                                                          |
| ------------- | --------------------------------------------------------------- |
| **↑** / **↓** | Previous / next **item** (skip status + path lines)             |
| **Enter**     | Add selected item’s **hostname** to the current MP; close modal |
| **c**         | Copy selected item’s **full URL** to clipboard; stay open       |
| **Ctrl+C**    | Close modal without quitting (unchanged)                        |




## Files likely touched

- `ui/panel/listUrlPanels.go` — multi-line render, status line, gray for existing hosts, sizing hints
- `ui/toggleModal.go` — larger `openModal` dimensions; pass existing domains into draw
- `ui/handlers.go` — item-aware ↑/↓/Enter; new **c** handler
- `KEYBOARD_SHORTCUTS.md` — document **c** and status-bar wording
- Optional: small clipboard helper package or function under `ui/`



## Out of scope (v1)

- Selecting / adding multiple URLs in one confirm
- Deduping domains on Enter
- Changing search engines or result count
- Mouse click-to-select items
- Showing link descriptions (`Link[1]`) in the list
- Search result paging (← / →) — see [SPEC_SELECT_URL_PAGING.md](SPEC_SELECT_URL_PAGING.md)



## Acceptance criteria

- [x] Select a URL modal opens with a large width and height (clearly larger than today’s host-only shrink-wrap)
- [x] Top of the panel shows a status hint equivalent to `enter: Add Domain, c: Copy Link`
- [x] Each result shows hostname on one line and indented path on the next
- [x] ↑ / ↓ move between items; cursor does not stick on the status or path-only lines as separate “rows”
- [x] Hosts already on the current MP render in gray foreground
- [x] **Enter** still adds the selected hostname and closes the modal
- [x] **c** copies the full selected URL to the clipboard and logs success/failure without closing
- [x] `KEYBOARD_SHORTCUTS.md` documents the new binding