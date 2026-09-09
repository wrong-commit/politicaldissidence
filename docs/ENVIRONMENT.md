# Environment variables

Runtime flags read by the TUI (and related packages). For the boolean `"true"` skip flags, values are case-insensitive; unset or any value other than `true` leaves the feature **enabled**.

| Variable | Values | Default | Behaviour |
| -------- | ------ | ------- | --------- |
| `MP_DATA_PATH` | path string / unset | `mp_data.json` | Overrides the MP JSON database path for TUI load/reload/save, `cmd/checkdomains`, and the default `-mp` for `cmd/csvrefresh`. Relative paths are resolved from the process working directory. Empty / whitespace-only is treated as unset. |
| `SKIP_BACKGROUND_WHOIS_LOOKUP` | `true` / unset / other | enabled (unset) | When `true`, the automatic background WHOIS scan does **not** run (startup and periodic). Manual **`u`** / domain-added WHOIS still run. Logs: `INFO background WHOIS skipped (SKIP_BACKGROUND_WHOIS_LOOKUP=true)`. |
| `SKIP_BACKGROUND_DNS_LOOKUP` | `true` / unset / other | enabled (unset) | When `true`, the automatic background DNS scan does **not** run (startup and periodic). Manual **`u`** / domain-added DNS still run. Logs: `INFO background DNS skipped (SKIP_BACKGROUND_DNS_LOOKUP=true)`. |
| `SKIP_BACKGROUND_HTTPS_LOOKUP` | `true` / unset / other | enabled (unset) | When `true`, the automatic background HTTPS scan does **not** run (startup and periodic). Manual **`u`** / domain-added HTTPS still run. Logs: `INFO background HTTPS skipped (SKIP_BACKGROUND_HTTPS_LOOKUP=true)`. |
| `SKIP_PERIODIC_DOMAIN_CHECKS` | `true` / unset / other | enabled (unset) | When `true`, the **30-minute** ticker that re-runs WHOIS / DNS / HTTPS is not armed. Startup one-shot scans still run (unless their own `SKIP_BACKGROUND_*` flags are set). Manual **`u`** unaffected. Logs once at arm time: `INFO periodic domain checks ticker skipped (SKIP_PERIODIC_DOMAIN_CHECKS=true)`. |
| `SKIP_BACKGROUND_CSV_REFRESH` | `true` / unset / other | enabled (unset) | When `true`, the hourly CSV refresh ticker is not armed. **Ctrl+L** still runs when `csv_refresh.json` is valid. Logs: `INFO background CSV refresh ticker skipped (SKIP_BACKGROUND_CSV_REFRESH)`. |
| `SKIP_TITLE_SCREEN` | `true` / unset / other | title shown (unset) | When `true`, the startup ASCII title panel is not shown and the ≥1s hold is skipped. Load / tickers / background jobs are unchanged. Logs: `INFO title screen skipped (SKIP_TITLE_SCREEN=true)`. See [SPEC_TITLE_SCREEN.md](specs/SPEC_TITLE_SCREEN.md). |

## Periodic domain checks (detail)

After load, the TUI runs one background WHOIS, DNS, and HTTPS pass (subject to the per-check `SKIP_BACKGROUND_*` flags).

If `SKIP_PERIODIC_DOMAIN_CHECKS` is not `true`, a ticker also fires every **30 minutes**. Each tick starts the same three background jobs with `force=false`:

- Walks **all** loaded MPs / domains (not the filtered list).
- **Skips** domains still within their freshness window (`WhoisMaxAge` / `DnsMaxAge` / `HttpsMaxAge` — currently **10 days** since last successful check).
- Looks up only domains that were never checked or last checked **≥ max age** ago (same rules as the startup scan).
- Single-flight per check type: if a previous run is still going, the new tick logs that a refresh is already running and does not start a second concurrent pass for that type.
- Updates stay in memory until the user saves (**Ctrl+S**).

## Examples (PowerShell)

```powershell
# Use a different MP database file (load, reload, Ctrl+S, checkdomains)
$env:MP_DATA_PATH = ".\mp_debug_examples.json"
.\politicaldissidence.exe

# Skip only the 30-minute recheck loop; still scan once at startup
$env:SKIP_PERIODIC_DOMAIN_CHECKS = "true"
.\politicaldissidence.exe

# Skip all automatic WHOIS (startup + periodic); DNS/HTTPS still run
$env:SKIP_BACKGROUND_WHOIS_LOOKUP = "true"
.\politicaldissidence.exe

# Quiet network on startup for all domain checks + no periodic ticker
$env:SKIP_BACKGROUND_WHOIS_LOOKUP = "true"
$env:SKIP_BACKGROUND_DNS_LOOKUP = "true"
$env:SKIP_BACKGROUND_HTTPS_LOOKUP = "true"
$env:SKIP_PERIODIC_DOMAIN_CHECKS = "true"
.\politicaldissidence.exe

# Skip the startup title splash (no 1s hold)
$env:SKIP_TITLE_SCREEN = "true"
.\politicaldissidence.exe
```

## Related config (not env)

CSV refresh interval and source URL live in `csv_refresh.json` (see [SPEC_BACKGROUND_CSV_REFRESH.md](specs/SPEC_BACKGROUND_CSV_REFRESH.md)), not environment variables. Search term templates live in `search_terms.json`.
