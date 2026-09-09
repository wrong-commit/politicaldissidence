# Spec: Startup title screen (ASCII panel)

## Goal

Show a **title screen** for at least **1 second** while `InitApp` startup work runs. The screen is a normal **gocui panel** (same TUI stack as Help / Searching), not a pre-TUI `fmt.Print` splash. The panel’s body is ASCII art **embedded as a string constant in a Go source file** (`ascii.go`), **horizontally and vertically centered** in the terminal.

## Motivation

Startup (`NewUI` → `Init` → `Load` → config/tickers → `Loop`) currently paints nothing useful until `MainLoop` and the main layout exist. A short branded title panel covers that dead air and gives a clear “app is starting” surface without changing later main-UI behaviour.

## Prerequisites / current behaviour

| Aspect | Today |
| ------ | ----- |
| Entry | `main` → `ui.InitApp()` |
| Order | `NewUI` → `Init` (manager + keybindings) → `Load` → set `started`, arm tickers / kick background jobs → `Loop` (`MainLoop`) |
| Paint | Views only appear once `MainLoop` (or `gui.Update`) drives layout |
| Branding | No startup title / ASCII art |
| Asset | Draft art may exist as a scratch `.txt` in the repo; **not** loaded at runtime |

Relevant code: `ui/app.go` (`InitApp`), `ui/ui.go` (`NewUI`, `Init`, `Loop`), `ui/panels.go` (`openModal` / `createModal` centering, `Layout`), `ui/toggleModal.go` (Searching / Help modal pattern).

## Desired behaviour

### 1. Asset: embedded in `ascii.go`

- **No runtime filesystem read** of ASCII art (no `os.ReadFile`, no cwd/`mp_data.json`-style path).
- Put the art in a simple Go file named **`ascii.go`** (e.g. under `ui/` next to the title helpers), as a package-level raw string constant, e.g.:

```go
package ui

const titleASCII = `
...art lines...
`
```

- Contents: the Political Dissidence banner art (same content as the draft `ASCII..txt` / equivalent). Preserve intentional leading spaces; trailing newline optional.
- Empty constant: show a short fallback string (e.g. `Political Dissidence`) rather than a blank panel. No ERROR required for “missing file” because there is no file load.
- Do **not** use `go:embed` of a `.txt` for v1 — the Go source **is** the source of truth.

### 2. Title panel (TUI)

- New constant, e.g. `TITLE_PANEL` / view name `"title"` (or `TITLE_MODAL` if implemented as a modal).
- Registered like other modals in `modalViews` (or a dedicated one-shot view): frame title may be empty, `"Political Dissidence"`, or omitted — prefer a **minimal frame** so the ASCII is the visual focus.
- **Not** in `MainViews` / `tabViews`; display-only; **no** keybindings that change app state while it is open (ignore input or no-op, same spirit as Searching while a fetch runs).
- Opened via the existing modal helpers (`openModal` / `createModal`) or an equivalent `SetView` that is **centered on the terminal** the same way Help / Searching are.

### 3. Centering

Two layers of centering:

1. **Panel** — centered in the terminal (reuse `createModal` math: `width/2 ± w/2`, `height/2 ± h/2`).
2. **ASCII body inside the panel** — the art block is centered in the view’s content area:
   - Measure art **width** = max rune/cell width of any line (trim only if needed for consistency; prefer preserving intentional trailing spaces in the constant).
   - Measure art **height** = number of lines.
   - Panel size: large enough for the art plus a small margin (e.g. 2 cells padding), clamped to `gui.Size()` so it never exceeds the terminal. If the art is larger than the terminal, clip or scroll is acceptable for v1; prefer **clip with panel sized to terminal** and still center what fits.
   - Pad each displayed line with leading spaces so the block is horizontally centered in the view width.
   - Pad with blank lines above/below so the block is vertically centered in the view height.

Suggested helper (name free): `panel.DrawTitleASCII(art string, viewW, viewH int) string` — pure function, easy to unit test.

### 4. Timing relative to `InitApp`

| Requirement | Detail |
| ----------- | ------ |
| Minimum display | **1 second** wall clock from first successful paint / open of the title panel |
| Overlap | Title remains visible **while** synchronous startup work that today runs before `Loop` continues (`Load`, CSV config load, ticker arming, kicking background WHOIS/DNS/HTTPS, etc.) |
| Dismissal | Close the title panel only when **both** are true: (a) that startup work has finished, and (b) ≥ 1s has elapsed |
| After dismiss | Main layout is the normal `MainViews` UI; `started` / log flush behaviour remains as today (or equivalent); focus lands on `LIST_PANEL` as today |

**Paint constraint:** gocui does not show views until the main loop (or an update flush) runs. `InitApp` must be restructured so the title can actually appear **during** startup, for example:

1. `NewUI` + `Init`
2. Open title panel (embedded `titleASCII` written into the view)
3. Enter `MainLoop` early **or** otherwise ensure a layout pass paints the title
4. Run remaining startup (`Load`, config, tickers, background kicks) on a path that does not block the first paint (goroutine + `gui.Update`, or load before loop but flush/paint title first — pick one approach and keep it simple)
5. When startup done **and** 1s elapsed → `closeModal(TITLE_…)` (or delete the title view) and continue with the normal interactive UI

Exact concurrency shape is an implementation detail; acceptance is: user sees the centered ASCII panel for ≥ 1s overlapping startup, then the usual MP list UI.

Background jobs that today start at the end of `InitApp` may start **before** the title dismisses (as long as they do not steal focus or replace the title panel). Prefer starting them in the same relative place as today (after load / config), still under the title if the 1s timer has not finished.

### 5. Skip via env: `SKIP_TITLE_SCREEN`

**Disable via env:** if `SKIP_TITLE_SCREEN=true` (case-insensitive, same boolean rules as other skip flags in [ENVIRONMENT.md](../ENVIRONMENT.md)), do **not** open the title panel and do **not** wait the 1s minimum. Run the normal `InitApp` startup path straight into the main UI (`Load`, config, tickers, background kicks, `Loop` as appropriate).

| Variable | Values | Default | Behaviour |
| -------- | ------ | ------- | --------- |
| `SKIP_TITLE_SCREEN` | `true` / unset / other | title **shown** (unset) | When `true`, skip the title panel and the ≥1s hold. Startup work and the main UI are unchanged otherwise. |

- Log once when skipped, e.g. `INFO title screen skipped (SKIP_TITLE_SCREEN=true)` (optional but preferred for parity with other `SKIP_*` flags).
- Document in [ENVIRONMENT.md](../ENVIRONMENT.md).
- Useful for fast local iteration / CI without waiting on the splash.

### 6. Interaction

- While the title is up: **no** Tab / list navigation / modals; Esc / Ctrl+C may still quit if that matches global quit bindings — do not invent a “skip splash” key in v1 (use `SKIP_TITLE_SCREEN` instead).
- After dismiss: existing shortcuts unchanged. No entry in `KEYBOARD_SHORTCUTS.md` required unless a skip key is added later.

### 7. Logging

- Do not spam INFO on every successful title show unless useful for debugging; optional single DEBUG/INFO is fine.
- No file-missing ERROR path for the art (it is compiled in).
- When `SKIP_TITLE_SCREEN=true`, prefer a single INFO skip line (see §5).

## Suggested API / touch points

| Area | Change |
| ---- | ------ |
| `ui/ascii.go` (or similar) | Raw string constant with the banner art; `EnvSkipTitleScreen` + helper mirroring other `SKIP_*` checks |
| `ui/app.go` | Restructure `InitApp` for title + ≥1s + startup overlap; honour env skip |
| `ui/panels.go` / `ui/toggleModal.go` | `TITLE_*` constant, `modalViews` entry, open/close helpers |
| `ui/panel/` | Draw/center helper for ASCII body |
| `docs/ENVIRONMENT.md` | Document `SKIP_TITLE_SCREEN` |
| Tests | Center padding for known art + view size; empty-const fallback; env skip bypasses title |

## Out of scope

- Animated / multi-frame ASCII
- User-configurable splash duration or art path (beyond the boolean env skip)
- Skip-on-keypress
- Runtime load of a `.txt` art file / `go:embed` of a separate `.txt`
- Changing main layout proportions after the title closes
- Showing the title again on Ctrl+R reload

## Acceptance criteria

- [ ] On startup, a gocui panel shows the embedded `ascii.go` art centered in the terminal (panel centered; art centered in panel)
- [ ] Title is visible for **at least 1 second**
- [ ] Title overlaps `InitApp` startup work (`Load` and the other pre-`Loop` setup that remains part of startup)
- [ ] After dismiss, normal main panels and focus behave as today
- [ ] Art is compiled into the binary (no runtime `.txt` read for the title)
- [ ] `SKIP_TITLE_SCREEN=true` skips the title panel and the 1s wait; startup/main UI otherwise unchanged
- [ ] No new interactive shortcuts required for v1

## Test plan

- [ ] Launch app with art in `ascii.go`; confirm banner is centered and visible ≥ 1s before the MP list UI
- [ ] Art wider/taller than a small terminal: panel clamps; app still reaches main UI
- [ ] Empty `titleASCII`: fallback text still shows a usable title panel
- [ ] Slow `Load` (large `mp_data.json`): title stays until load finishes even if that is > 1s
- [ ] Fast `Load`: title still held for full 1s
- [ ] `SKIP_TITLE_SCREEN=true`: no title panel, no 1s delay; app reaches main UI after normal startup
- [ ] After title, Tab / list / log panel work as before; background WHOIS/DNS/HTTPS still start
