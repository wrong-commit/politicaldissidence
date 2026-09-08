# Spec: atomic MP JSON save

## Goal

Persist in-memory MPs to `mp_data.json` without risking a corrupt or half-written live database. A failed or interrupted save must leave the previous on-disk file intact (or, after a successful replace, a fully valid new file).

## Entry points

All saves go through `db.WriteMps` → `db.write`:

| Trigger | Caller |
| ------- | ------ |
| Ctrl+S | `UI.Save` |
| Background WHOIS refresh | `ui/whoisRefresh.go` `Save` dependency |
| CLI / `main` helpers | direct `db.WriteMps` |

Live filename: `mp_data.json` (`mpJsonFilename`). Temp files live in the **same directory** as the destination.

## Save process (overview)

```
encode MPs → unique tmp file → fsync/close → validate tmp → delete live → rename tmp → done
```

1. **Encode** — `json.Encoder` with indent (`" "` prefix, `"  "` indent) into a memory buffer. Encoding failures abort; nothing is written to disk.
2. **Write temp** — `os.CreateTemp(dir, "mp_data.*.tmp")` creates a uniquely named file (random suffix). This avoids collisions when two writers save at the same time (e.g. Ctrl+S vs background WHOIS). Bytes are written, then `Sync` + `Close`.
3. **Revalidate** — run the same structural validation used on load/reload (`ValidateMPJSONFile` on the temp path). Rules match [SPEC_JSON_VALIDATION.md](SPEC_JSON_VALIDATION.md): readable file, well-formed JSON, root array, decode into `[]data.MP`, no trailing garbage.
4. **Replace** — `replaceFile(tmp, mp_data.json)`:
   - Delete the live `mp_data.json` if it exists.
   - Rename the temp file onto `mp_data.json`.
   - Same-directory rename keeps the swap as atomic as the OS allows. On Windows, rename cannot overwrite an existing file, so delete-then-rename is required.
5. **Cleanup on failure** — if any step before a successful rename fails, the temp file is removed and the previous live DB (if any) is left unchanged.

## Temp file naming and gitignore

- Pattern: `mp_data.<random>.tmp` (via `CreateTemp` pattern `mp_data.*.tmp`).
- Ignored by git: `mp_data.*.tmp` in `.gitignore`.
- Successful save leaves **no** temp file behind; only crash/interrupt mid-write may leave orphans (safe to delete manually).

## Failure behavior

| Failure | Live `mp_data.json` | Temp file |
| ------- | ------------------- | --------- |
| Encode error | unchanged | never created |
| Temp create/write/sync error | unchanged | removed |
| Validation fails | unchanged | removed |
| Delete/rename error | may be missing if delete succeeded but rename failed | still present (caller sees error; may need manual recovery) |

UI callers (`UI.Save`) already log a generic write failure when `WriteMps` returns an error.

## Relationship to load validation

- **Load / Reload** validate the **live** file before accepting it into memory ([SPEC_JSON_VALIDATION.md](SPEC_JSON_VALIDATION.md)).
- **Save** validates the **temp** file before it becomes the live file.
- Both share `ValidateMPJSONFile` / `decodeMpsStrict` so “what we write” and “what we read” use the same correctness rules.

## Out of scope (v1)

- Cross-process or in-process save locking (Ctrl+S vs background WHOIS may race; last successful replace wins)
- Keeping a `.bak` of the previous live file
- Stronger Windows replace (`MoveFileEx` with replace-existing) to avoid the brief delete/rename gap
- Changing the live filename or JSON schema

## Acceptance criteria

- [x] Saves write through a uniquely named temp file, not in-place overwrite of `mp_data.json`
- [x] Temp file is revalidated before replace
- [x] Failed validation leaves the previous live file intact
- [x] Successful replace removes the temp name (live file is `mp_data.json`)
- [x] Temp pattern is gitignored
- [x] Unit coverage for happy-path reload, shorter overwrite without trailing garbage, and replace helper
