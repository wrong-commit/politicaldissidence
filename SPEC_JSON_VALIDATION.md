# Spec: MP JSON validation on load / reload

## Goal

Whenever MP data is read from disk (startup load and Ctrl+R reload), validate that `mp_data.json` is entirely correct JSON for the app’s MP schema. Log a clear **valid** or **invalid** result to the console (log panel). On invalid JSON, emit an **ERROR** line that includes the **full file path** and **file size in kilobytes**.

## Prerequisites / current behavior

- Persistence lives in `db` (`mpJsonFilename` = `mp_data.json`).
- `db.ReadMps` reads the file and `json.Decoder.Decode`s into `[]data.MP`.
- `UI.Load` (startup via `InitApp`) and `UI.Reload` (Ctrl+R) both call `db.ReadMps`.
- Today, decode failures are only partially surfaced (`fmt.Printf` in `deserializeMps`; `Load` panics; `Reload` logs a misleading “Could not write…” message). There is no dedicated valid/invalid console summary and no path + size on failure.

## What “entirely correct” means

Validation succeeds only when **all** of the following hold:

1. **File readable** — the JSON file exists and can be read (same path `ReadMps` uses).
2. **Well-formed JSON** — the file bytes parse as JSON with no trailing garbage (prefer `json.Decoder` with `DisallowUnknownFields` **off** for forward-compat of optional fields like `lastChecked`, but require a complete decode of the root value).
3. **Root type** — root value is a **JSON array** of MP objects (not an object, null, or scalar).
4. **Schema decode** — each element unmarshals into `data.MP` / nested `data.Domain` without decode error (same types as normal load).

Optional / empty fields (`domains`, `expiry`, `lastChecked`, etc.) remain allowed as today. Semantic business rules (e.g. non-empty hostname) are **out of scope** for v1 — this spec is structural JSON + decode correctness, not domain-policy linting.

If validation fails for any reason above, the load/reload path must treat the file as **invalid** and log accordingly.

## When to run

| Trigger | Path | Notes |
| ------- | ---- | ----- |
| Startup | `UI.Load` (from `InitApp`) | After UI/console can accept logs (or buffer via existing `startupLog` until `started`, same as other early logs). |
| Reload | `UI.Reload` (Ctrl+R) | Same validation as startup, before replacing in-memory MP state. |

Do **not** re-validate on `Save` / `WriteMps` in v1 (writer already encodes from in-memory structs). Do **not** run as a separate background job.

Prefer a single shared helper (e.g. in `db` or a small validate function used by `ReadMps` / Load+Reload) so startup and reload share identical rules and log wording.

## Behavior on valid vs invalid

**Valid**

- Proceed with normal load/reload (populate `ui.state.all` / `visible`, redraw list, etc.).
- Log a non-error console line that the JSON is valid (see table below).

**Invalid**

- Log an **ERROR** console line with full absolute path and size in KB (see table).
- Also log that the JSON is invalid (can be the same ERROR line, or a short companion line — prefer one ERROR that covers both).
- Do **not** silently replace in-memory state with a partial/empty decode.
- Startup (`Load`): do not panic solely for invalid JSON if a console ERROR can be shown; keep prior empty/unloaded state usable enough to see the log. (If the file is missing entirely, still ERROR with path + size `0` or omit size only if `Stat` fails — prefer size when known.)
- Reload (`Reload`): leave previous in-memory MPs unchanged; return an error after logging.

## Console log messages

Use level prefixes consistent with the WHOIS refresh spec (`INFO` / `ERROR`). Prefer `ui.logPlain` (or equivalent) so ANSI coloring does not corrupt `INFO`/`ERROR` prefixes in the log panel.


| Level | When | Example |
| ----- | ---- | ------- |
| INFO | JSON validated successfully | `INFO mp_data.json valid` |
| ERROR | JSON invalid (any failure reason) | `ERROR mp_data.json invalid path=C:\...\mp_data.json size=128.4KB line=42 col=3: <reason>` |


**ERROR details (required):**

- **Full file path** — absolute path to the file that was validated (resolve via `filepath.Abs` or equivalent from the path used to open the file).
- **File size in kilobytes** — size of the file on disk at validation time, expressed in KB (decimal kilobytes: `bytes / 1000.0`, or binary KiB if documented — pick one and stick to it; recommend **bytes/1024** rounded to one decimal, labeled `KB`).
- **Line / column** — when the failure has a byte offset (`json.SyntaxError`, `UnmarshalTypeError`, or trailing garbage after a valid value), include 1-based `line` and `col` in the ERROR log.
- **Reason** — short decode/read error text (e.g. unexpected EOF, invalid character, wrong root type).

**Valid line:** may include path optionally; minimum is a clear valid/invalid outcome. Matching the table above is enough for v1.

Both startup and reload must emit these messages (valid → INFO; invalid → ERROR with path + size).

## Acceptance criteria

- [x] On startup (`Load`), MP JSON is validated before (or as part of) accepting loaded MPs
- [x] On reload (`Reload` / Ctrl+R), the same validation runs
- [x] Console shows an INFO (or clear non-error) line when JSON is valid
- [x] Console shows an ERROR line when JSON is invalid, including **full path** and **size in KB**
- [x] Invalid reload does not wipe previously loaded in-memory MPs
- [x] Startup and reload share the same validation + log wording

## Out of scope (v1)

- Schema lint of required string fields / hostname format
- Rejecting unknown JSON fields (`DisallowUnknownFields`)
- Validating on save / write
- Auto-repair or backup of corrupt JSON
- Changing the on-disk filename or format of `mp_data.json`

## Unit test plan

Prefer table-driven tests. Keep gocui out of unit tests — validate pure `db` (or extracted) helpers with temp files / `io.Reader` fixtures.

---

### 1. Validation core (`db` or shared helper)

| ID | Area | Cases |
|----|------|--------|
| V1 | Valid array | well-formed `[]` of MP-shaped objects → valid; no error |
| V2 | Empty array | `[]` → valid |
| V3 | Syntax error | truncated / garbage JSON → invalid; error reason non-empty |
| V4 | Wrong root | JSON object `{}` or scalar → invalid |
| V5 | Trailing garbage | valid array then extra tokens → invalid if decoder requires EOF |
| V6 | Nested domain | valid MP with `domains` including `lastChecked` → valid |
| V7 | Path + size | invalid path reports absolute path and size in KB matching fixture file length |
| V8 | Missing file | unreadable / missing file → invalid; path still reported; size `0` or explicit “unknown” only if Stat fails |

### 2. Load / reload wiring (light)

| ID | Area | Cases |
|----|------|--------|
| W1 | Load valid | validation INFO emitted; MPs populated |
| W2 | Load invalid | ERROR with path + KB; state not treated as successfully loaded |
| W3 | Reload valid | same INFO wording as load |
| W4 | Reload invalid | ERROR with path + KB; previous `all`/`visible` unchanged |

### Done when

1. §1 tests green for validate helper (path, size, valid/invalid).
2. Load and Reload both call validation and log per the console table.
3. Spec acceptance criteria verified manually: corrupt `mp_data.json`, restart app, and Ctrl+R after fixing/breaking the file.
