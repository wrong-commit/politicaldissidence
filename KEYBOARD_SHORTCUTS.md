# Keyboard Shortcuts

All bindings are defined in `ui/handlers.go` (plus `Ctrl+H` registered in `ApplyKeyBindings`). Focus matters: many shortcuts only work when a specific panel is active. Use **Tab** to move between the List and Domains panels.

In-app help: press **Ctrl+H** to open a modal listing shortcuts for the current panel plus globals. Press **Ctrl+H** or **Ctrl+C** to close it.

---

## Global (anywhere)


| Key        | Action                                                                                                              |
| ---------- | ------------------------------------------------------------------------------------------------------------------- |
| **Ctrl+H** | Toggle help modal (shortcuts for the focused panel + globals)                                                       |
| **Ctrl+C** | Quit the app, or close the open modal if one is showing. Does not save to avoid corrupting the DB while developing. |
| **Ctrl+S** | Save MPs and domains to disk                                                                                        |
| **Ctrl+R** | Reload MPs and domains from disk                                                                                    |
| **Ctrl+P** | Force-recheck all domains (WHOIS + DNS + HTTPS), ignoring lastChecked / checkedAt                                   |
| **PgUp**   | Scroll Domain Information panel up one page                                                                         |
| **PgDn**   | Scroll Domain Information panel down one page                                                                       |


---



## List panel (`list`)

Focus this panel with **Tab** (or click it).


| Key        | Action                                                                     |
| ---------- | -------------------------------------------------------------------------- |
| **↑**      | Previous MP                                                                |
| **↓**      | Next MP                                                                    |
| **f**      | Cycle MP filter: `all` → `have domains` → `no domains` → `all`             |
| **Tab**    | Focus next panel (Domains)                                                 |
| **g**      | Guess domain (search for the selected MP, then open URL picker)            |


---



## Domains panel (`domains`)

Focus this panel with **Tab** (or click it). Shows domains for the currently selected MP.


| Key        | Action                                                       |
| ---------- | ------------------------------------------------------------ |
| **↑**      | Previous domain                                              |
| **↓**      | Next domain                                                  |
| **Tab**    | Focus next panel (List)                                      |
| **g**      | Guess domain (same as on List)                               |
| **u**      | Check domain (WHOIS + DNS + HTTPS for the selected domain) |


---



## Domain Information panel (`whois`)

Display-only (not focusable). Shows WHOIS + DNS for the domain selected in Member Domains. Scroll with **PgUp** / **PgDn** from anywhere (see Global).


---



## Select a URL modal (`search`)

Opened after **g** finishes searching. Lists candidate URLs/domains for the selected MP.

Status line at the top of the modal: `↑/↓: Move, enter: Add Domain, c: Copy Link, ←/→: Page, e: Engine, t: Term`

Title includes page, engine, and term index, e.g. `Select a URL (Page 1 · Bing · T1)`.


| Key        | Action                                                              |
| ---------- | ------------------------------------------------------------------- |
| **↑**      | Previous URL                                                        |
| **↓**      | Next URL                                                            |
| **Enter**  | Add the selected URL’s domain to the current MP and close the modal |
| **c**      | Copy the selected full URL to the clipboard (modal stays open)      |
| **←**      | Previous result page (no-op on page 1)                              |
| **→**      | Next result page (shows Searching while fetching)                   |
| **e**      | Toggle search engine Bing ↔ DuckDuckGo; re-fetch page 0             |
| **t**      | Toggle search term T1 ↔ T2; re-fetch page 0                         |
| **Ctrl+C** | Close modal without quitting                                        |


---



## Searching modal (`searching`)

Shown briefly while a background domain search runs after **g**. Not meant for interaction; it closes when the search finishes (or on **Ctrl+C**).

---



## Mouse

Left-click / mouse release on clickable panels focuses that panel. Clicking a row in the List panel also selects that MP.

---



## Quick reference

```
Ctrl+H          Help
Ctrl+C          Quit / close modal
Ctrl+S          Save
Ctrl+R          Reload
Ctrl+P          Force recheck all domains
PgUp / PgDn     Scroll Domain Information
Tab             Switch List ↔ Domains

List / Domains:
  ↑ ↓           Navigate
  g             Guess domain (search)

List only:
  f             Cycle filter

Domains only:
  u             Check domain (WHOIS)

URL picker:
  Enter         Confirm selection
  ↑ ↓ ← →      Move / page
  e / t         Engine / term
```

