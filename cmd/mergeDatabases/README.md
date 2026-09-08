# mergeDatabases

Merge two MP JSON databases into a new file, deduplicating members by name.

See the full behaviour contract: [SPEC_MERGE_DATABASES.md](../../docs/specs/SPEC_MERGE_DATABASES.md).

## Usage

From the project root:

```powershell
go run ./cmd/mergeDatabases -a .\mp_data.json -b .\mps_from_csv.json
go run ./cmd/mergeDatabases -a .\mp_data.json -b .\mps_from_csv.json -o .\mps_merged.json
```

Build:

```powershell
go build -o mergeDatabases.exe ./cmd/mergeDatabases
.\mergeDatabases.exe -a .\mp_data.json -b .\mps_from_csv.json -o .\out.json
```

## Rules (summary)

- `-a` and `-b` are required validated MP JSON arrays.
- `-o` defaults to `mps_merged.json` and **must not already exist** (no overwrite).
- Same `FirstName` + `Surname` (case-insensitive) collapses to one member.
- On name match, biographical / party fields come from the **later** record (`-b` over `-a`, and later duplicates within a file).
- Domains are unioned by hostname; WHOIS, DNS, and HTTPS (HTTP status) checks are cleared on name-merged members.
