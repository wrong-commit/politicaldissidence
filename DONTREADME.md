# Political Dissonance 

## TODO

- [ ] Automating MP detection
    - [ ] Get APH list of Senators and Reps
        - [x] Parse HTML [0] 
        - [x] Fetch CSV files 
    - [ ] Get list of territory and state Members and Senators
        - [ ] Find sites 
        - [ ] Parse HTML 
        - [ ] Fetch CSV files 
    - [ ] From CSV files extract (Honorific) (Full Name + Prefered Name) (Party) (Electorate) into database
        - [ ] Alert when new MP found
- [ ] Linking MP and domain
    - [ ] Display list of MPs requiring linkage
    - [ ] Generate google search terms for MP homepage
        - [ ] Automate using non-G search engine (?)
    - [ ] Allow CRUD to set domain to MP
    - [ ] WHOIS lookup to check domain
- [ ] FE ( golang TUI )
    - [ ] Display list of MP/domains that will soon be expiring
    - [ ] 
- [ ] Maintainence 
    - [ ] Detect when URL disappears
    - [ ] Alert when new MP found/removed
- [ ] Code
    - [ ] Figure out how and when to use `log` package, replace fmt.Println


[0] https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files


### gocui references

https://github.com/jroimartin/gocui
https://github.com/inconshreveable/ngrok/blob/master/src/ngrok/client/views/term/area.go 
