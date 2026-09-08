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

On macOS / Linux (or Git Bash), the `make` script builds then runs:

```sh
./make
```

That script runs `go build` and, on success, `./politicaldissidence`.

## Keyboard shortcuts

See [KEYBOARD_SHORTCUTS.md](KEYBOARD_SHORTCUTS.md).

## Specs

- [Background WHOIS refresh](SPEC_BACKGROUND_WHOIS.md)
- [WHOIS information panel](SPEC_WHOIS_PANEL.md)
- [Domain-added background jobs](SPEC_DOMAIN_ADDED_JOBS.md)
- [MP JSON validation on load / reload](SPEC_JSON_VALIDATION.md)
- [Atomic MP JSON save](SPEC_JSON_SAVE.md)
- [Select a URL modal improvements](SPEC_SELECT_URL_MODAL.md)
- [Select a URL search paging (v2)](SPEC_SELECT_URL_PAGING.md)
- [Select a URL searcher controls (v3)](SPEC_SELECT_URL_SEARCHER_V3.md)

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
    - [x] Allow CRUD to set domain to MP
    - [x] WHOIS lookup to check domain
        - [x] Perform WHOIS and get expiry date
        - [x] Background WHOIS refresh (see [SPEC_BACKGROUND_WHOIS.md](SPEC_BACKGROUND_WHOIS.md))
        - [x] Domain-added jobs kickoff (see [SPEC_DOMAIN_ADDED_JOBS.md](SPEC_DOMAIN_ADDED_JOBS.md))
        - [x] Persist domain `lastChecked` on WHOIS
        - [x] Show last-checked in domain panel
        - [ ] Show full WHOIS response
    - [x] Validate `mp_data.json` on load / reload (see [SPEC_JSON_VALIDATION.md](SPEC_JSON_VALIDATION.md))
    - [x] Fix UI bugginess, cursor gets out of whack 
    - [x] Add multiple engines/search terms to url lookup
- [ ] Maintainence 
    - [ ] Detect when URL disappears
    - [ ] Alert when current list changes found/removed
    - [x] Check all domains on startup
    - [x] WHOIS information panel (see [SPEC_WHOIS_PANEL.md](SPEC_WHOIS_PANEL.md))
    - [x] JSON Validation on startup/reload
- [ ] Code
    - [ ] Figure out how and when to use `log` package, replace fmt.Println
    - [x] Cleanup CLI code base buginess / shit ness 


[0] https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files


### gocui references

https://github.com/jroimartin/gocui
https://github.com/inconshreveable/ngrok/blob/master/src/ngrok/client/views/term/area.go 
https://github.com/esimov/diagram

