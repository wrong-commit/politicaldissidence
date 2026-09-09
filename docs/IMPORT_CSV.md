# Import MPs from CSV

Turn APH (or similar) address-label CSVs into an MP JSON database via `cmd/csv2json`.

## 1. Download CSVs

Federal senator and House of Representatives address-label CSVs are published here:

[https://www.aph.gov.au/Senators_and_Members/Contacting_Senators_and_Members](https://www.aph.gov.au/Senators_and_Members/Contacting_Senators_and_Members)

Related guidelines / CSV notes:

[https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files](https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files)

Save the files somewhere under the project root (any path is fine; pass it to the flags below).

Typical filenames:


| Chamber       | Example file                                         |
| ------------- | ---------------------------------------------------- |
| Senators      | `allsenel.csv` or `allsenph.csv`                     |
| House of Reps | `FamilynameRepsCSV.csv` or `All members by name.csv` |


Header names differ between formats and different years. The parser maps headers via a `ColumnMap` (see [Adding a new CSV format](#adding-a-new-csv-format)).

## 2. Run csv2json

From the project root, provide `-senators` and/or `-members`, **or** `-custom` (not both). Output defaults to `mps_from_csv.json` (it does **not** overwrite `mp_data.json` unless you pass `-o`).

```powershell
go run ./cmd/csv2json -senators .\allsenel.csv -members .\FamilynameRepsCSV.csv
go run ./cmd/csv2json -senators .\allsenph.csv -o .\senators.json
go run ./cmd/csv2json -members .\FamilynameRepsCSV.csv -o .\members.json
go run ./cmd/csv2json -custom .\my.csv -o .\custom_mps.json
```

Or build once and run the binary:

```powershell
go build -o csv2json.exe ./cmd/csv2json
.\csv2json.exe -senators .\allsenel.csv -members .\FamilynameRepsCSV.csv
.\csv2json.exe -senators .\allsenph.csv -o .\senators.json
.\csv2json.exe -members .\FamilynameRepsCSV.csv -o .\members.json
.\csv2json.exe -custom .\my.csv -o .\custom_mps.json
```

Flags:


| Flag        | Meaning                                                               |
| ----------- | --------------------------------------------------------------------- |
| `-senators` | Path to senators CSV (uses `SenatorColumns`, level = Federal Senator) |
| `-members`  | Path to HoR members CSV (uses `MemberColumns`, level = Federal Rep)   |
| `-custom`   | Path to custom CSV (uses `CustomColumns`; exclusive with the above)   |
| `-o`        | Output JSON path (default `mps_from_csv.json`)                        |


Provide `-senators` and/or `-members`, or `-custom` alone. With senators+members, rows are concatenated into one file.

On success, stderr reports how many MPs were parsed and written.

## 3. What gets written

Each CSV row becomes an MP with honorific, names, party, state, electorate, and a chamber `level`. Domains are empty until you link them in the TUI (or elsewhere).

Built-in column maps live in `[csv/read.go](../csv/read.go)`:

- `SenatorColumns` — APH senators address-label headers (`Title`, `Electorate Suburb`, …)
- `MemberColumns` — APH House of Reps headers (`Honorific`, `Electorate`, …)
- `CustomColumns` — template with all fields as `""`; fill in headers for ad-hoc imports via `-custom`

## Adding a new CSV format

Easiest path: edit `CustomColumns` in `csv/read.go` with the **exact CSV header names**, then run with `-custom`:

```go
var CustomColumns = ColumnMap{
	Honorific:     "Title",
	FirstName:     "First Name",
	Surname:       "Surname",
	OtherName:     "Other Name",
	PreferredName: "Preferred Name",
	Party:         "Party",
	State:         "State",
	Electorate:    "District",
}
```

```powershell
go run ./cmd/csv2json -custom .\my.csv -o .\custom_mps.json
```

For a permanent built-in format, add another `ColumnMap` + `ParseXFile` / `ParseXReader` wrappers (and optionally a new `csv2json` flag), same pattern as senators/members.

Every mapped header must appear in the CSV’s first row; otherwise import fails with an “Expected column …” error.