# csvrefresh CLI

Dry-run the same CSV refresh pipeline as TUI **Ctrl+L** (fetch listing page → find CSV link → download → parse → merge in memory). Does **not** write `mp_data.json`.

```powershell
go run ./cmd/csvrefresh
go run ./cmd/csvrefresh -v
go run ./cmd/csvrefresh -config .\csv_refresh.json -mp .\mp_data.json -v
go run ./cmd/csvrefresh -mp "" -v
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-config` | `csv_refresh.json` | Config path |
| `-mp` | `mp_data.json` | Existing DB to merge against (`""` = empty) |
| `-v` | false | Print DEBUG lines (resolved download URL) |

Exit `1` on fetch/parse failure, `2` on bad config.
