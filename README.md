# Political Dissidence 

## Build and run

Requires [Go](https://go.dev/dl/) (module targets Go 1.17+).

From the project root:

```powershell
go build
.\politicaldissidence.exe
```

Or in one step without writing a binary:

```powershell
go run .
```

Batch watchlist (WHOIS + DNS, print / persist `alert` domains):

```powershell
go run ./cmd/checkdomains
go run ./cmd/checkdomains -dry-run
```

Or build and run the executable:

```powershell
go build -o checkdomains.exe ./cmd/checkdomains
.\checkdomains.exe
.\checkdomains.exe -dry-run
.\checkdomains.exe -v
.\checkdomains.exe -soon-days 90 -delay 1s
.\checkdomains.exe -save=false
```

Import APH CSVs into MP JSON (writes `mps_from_csv.json` by default, not `mp_data.json`):

```powershell
go run ./cmd/csv2json -senators .\allsenph.csv -members .\FamilynameRepsCSV.csv
go run ./cmd/csv2json -members .\FamilynameRepsCSV.csv -o .\mps_from_csv.json
```

Or build and run the executable:

```powershell
go build -o csv2json.exe ./cmd/csv2json
.\csv2json.exe -senators .\allsenph.csv -members .\FamilynameRepsCSV.csv
.\csv2json.exe -senators .\allsenph.csv -o .\senators.json
.\csv2json.exe -members .\FamilynameRepsCSV.csv -o .\members.json
```

On macOS / Linux (or Git Bash), the `make` script builds then runs:

```sh
./make
```

That script runs `go build` and, on success, `./politicaldissidence`.

## Keyboard shortcuts

See [KEYBOARD_SHORTCUTS.md](docs/KEYBOARD_SHORTCUTS.md).

## Specs

- [Background WHOIS refresh](docs/specs/SPEC_BACKGROUND_WHOIS.md)
- [WHOIS information panel](docs/specs/SPEC_WHOIS_PANEL.md)
- [DNS emptiness check](docs/specs/SPEC_DNS_EMPTY.md)
- [CLI batch domain watchlist](docs/specs/SPEC_CLI_EXPIRED_DOMAINS.md)
- [Domain-added background jobs](docs/specs/SPEC_DOMAIN_ADDED_JOBS.md)
- [MP JSON validation on load / reload](docs/specs/SPEC_JSON_VALIDATION.md)
- [Atomic MP JSON save](docs/specs/SPEC_JSON_SAVE.md)
- [Select a URL modal improvements](docs/specs/SPEC_SELECT_URL_MODAL.md)
- [Select a URL search paging (v2)](docs/specs/SPEC_SELECT_URL_PAGING.md)
- [Select a URL searcher controls (v3)](docs/specs/SPEC_SELECT_URL_SEARCHER_V3.md)

## TODO

- [ ] Automating MP detection
    - [ ] Get list of Senators and Reps
        - [x] Parse HTML [0] 
        - [x] Fetch CSV files 
    - [ ] Get list of territory and state Members and Senators
        - [ ] Find sites 
        - [ ] Parse HTML 
        - [ ] Fetch CSV files 
    - [x] From CSV files extract (Honorific) (Full Name + Prefered Name) (Party) (Electorate) into database
    - [ ] Alert when new MP found
- [ ] CLI 
    - [x] Linking MP and domain
    - [x] Display list of MPs requiring linkage
    - [ ] Search engines
        - [x] duckduckgo
        - [ ] google
        - [x] bing
        - [ ] bypass google/ddg rate limit 
        - [x] pagination
        - [x] automatic fallthrough to other domains when search fails
    - [x] Allow CRUD to set domain to MP
    - [x] WHOIS lookup to check domain
        - [x] Perform WHOIS and get expiry date
        - [x] Background WHOIS refresh (see [SPEC_BACKGROUND_WHOIS.md](docs/specs/SPEC_BACKGROUND_WHOIS.md))
        - [x] Domain-added jobs kickoff (see [SPEC_DOMAIN_ADDED_JOBS.md](docs/specs/SPEC_DOMAIN_ADDED_JOBS.md))
        - [x] Persist domain `lastChecked` on WHOIS
        - [x] Show last-checked in domain panel
        - [x] Show full WHOIS response
        - [x] DNS emptiness check (see [SPEC_DNS_EMPTY.md](docs/specs/SPEC_DNS_EMPTY.md))
        - [x] Add script for running WHOIS checks against all MP domains and outputting expired domains (see [SPEC_CLI_EXPIRED_DOMAINS.md](docs/specs/SPEC_CLI_EXPIRED_DOMAINS.md))
    - [x] Validate `mp_data.json` on load / reload (see [SPEC_JSON_VALIDATION.md](docs/specs/SPEC_JSON_VALIDATION.md))
    - [x] Fix UI bugginess, cursor gets out of whack 
    - [x] Add multiple engines/search terms to url lookup
    - [x] DNS Record Checks as well as WHOIS
        - [x] [SPEC_DNS_EMPTY.md](docs/specs/SPEC_DNS_EMPTY.md)
    - [ ] Import files easily
        - [ ] easy config for modifying for different files
        - [x] HTTPS Certificate Checks (see [SPEC_HTTPS_CERT.md](docs/specs/SPEC_HTTPS_CERT.md))
        - [x] Add HTTP Status Checks
- [ ] Maintainence 
    - [ ] Detect when URL disappears
    - [ ] Alert when current list changes found/removed
    - [x] Check all domains on startup
    - [x] WHOIS information panel (see [SPEC_WHOIS_PANEL.md](docs/specs/SPEC_WHOIS_PANEL.md))
    - [x] JSON Validation on startup/reload
- [ ] Code
    - [ ] Figure out how and when to use `log` package, replace fmt.Println
    - [x] Cleanup CLI code base buginess / shit ness 


[0] https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files


### gocui references

https://github.com/jroimartin/gocui
https://github.com/inconshreveable/ngrok/blob/master/src/ngrok/client/views/term/area.go 
https://github.com/esimov/diagram

