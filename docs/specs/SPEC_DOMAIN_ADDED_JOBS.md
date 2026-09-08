# Spec: domain-added background jobs

When a domain is added to an MP, the UI kicks a list of background jobs through a shared helper instead of hardcoding `go refreshDomainWhois(...)`.

## Package

`jobs` provides:

| Piece | Role |
|-------|------|
| `Context` | MP index, domain index, hostname, optional log func |
| `Job` | `Name()` + `Run(Context) error` |
| `Kick` | Starts each job in its own goroutine; logs errors via `Context.Log` |
| `Func` | Adapts `func(Context) error` to `Job` |
| `NewWhoisOnAdd` | Wraps existing forced WHOIS + panel redraw |
| `NewCalcDemo` | Demo job: `calc.exe` via injectable `Starter` |

## Wire-up

- `UI.domainAddedJobs` is filled in `NewUI` (WHOIS + calc demo).
- `addDomain` calls `jobs.Kick(...)` with that slice.

To add another job: implement `Job` (or use `jobs.Func`) and append it to `domainAddedJobs`.

## Out of scope

- Session-start WHOIS (`startBackgroundWhois` / `Runner.TryRun`)
- Cancellation, queues, retries
- Non-Windows calc fallbacks
