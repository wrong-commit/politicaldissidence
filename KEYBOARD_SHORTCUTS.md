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


---



## List panel (`list`)

Focus this panel with **Tab** (or click it).


| Key        | Action                                                                     |
| ---------- | -------------------------------------------------------------------------- |
| **↑**      | Previous MP                                                                |
| **↓**      | Next MP                                                                    |
| **Ctrl+F** | Cycle MP filter: `all` → `have domains` → `no domains` → `all`             |
| **Tab**    | Focus next panel (Domains)                                                 |
| **Ctrl+A** | Open Add Domain modal                                                      |
| **g**      | Guess domain (search for the selected MP, then open URL picker)            |


---



## Domains panel (`domains`)

Focus this panel with **Tab** (or click it). Shows domains for the currently selected MP.


| Key        | Action                                                       |
| ---------- | ------------------------------------------------------------ |
| **↑**      | Previous domain                                              |
| **↓**      | Next domain                                                  |
| **Tab**    | Focus next panel (List)                                      |
| **Ctrl+A** | Open Add Domain modal                                        |
| **g**      | Guess domain (same as on List)                               |
| **U**      | Check domain (WHOIS / expiry update for the selected domain) |


---



## Add Domain modal (`adddomain`)

Opened with **Ctrl+A** from List or Domains. Type a domain, then confirm.


| Key        | Action                                       |
| ---------- | -------------------------------------------- |
| **Enter**  | Confirm and add the domain to the current MP |
| **Ctrl+C** | Close modal without quitting                 |


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
Tab             Switch List ↔ Domains

List / Domains:
  ↑ ↓           Navigate
  Ctrl+A        Add domain
  g             Guess domain (search)

List only:
  Ctrl+F        Cycle filter

Domains only:
  U             Check domain (WHOIS)

Add Domain / URL picker:
  Enter         Confirm selection
  ↑ ↓ ← →      Move / page
  e / t         Engine / term
```

