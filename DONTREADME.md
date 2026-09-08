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
    -   [x] duckduckgo
    -   [ ] google
    - [ ] bypass rate limit 
    - [x] Allow CRUD to set domain to MP
    - [x] WHOIS lookup to check domain
        - [x] Perform WHOIS and get expiry date
    - [ ] Fix UI bugginess, cursor gets out of whack 
- [ ] Maintainence 
    - [ ] Detect when URL disappears
    - [ ] Alert when current list changes found/removed
- [ ] Code
    - [ ] Figure out how and when to use `log` package, replace fmt.Println
    - [ ] Cleanup CLI code base buginess / shit ness  


[0] https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files


### gocui references

https://github.com/jroimartin/gocui
https://github.com/inconshreveable/ngrok/blob/master/src/ngrok/client/views/term/area.go 
https://github.com/esimov/diagram

