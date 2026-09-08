package main

import (
	"flag"
	"fmt"
	"os"

	"politicaldissidence/data"
	"politicaldissidence/db"
)

func main() {
	pathA := flag.String("a", "", "path to first MP JSON database")
	pathB := flag.String("b", "", "path to second MP JSON database (newer party details win on name match)")
	outPath := flag.String("o", "mps_merged.json", "output JSON path (must not already exist)")
	flag.Parse()

	if *pathA == "" || *pathB == "" {
		fmt.Fprintf(os.Stderr, "ERROR mergeDatabases: provide -a and -b JSON paths\n")
		flag.Usage()
		os.Exit(2)
	}

	if err := ensureOutMissing(*outPath); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR mergeDatabases: %v\n", err)
		os.Exit(1)
	}

	statusA := db.ValidateMPJSONFile(*pathA)
	if !statusA.Valid() {
		fmt.Fprintf(os.Stderr, "ERROR mergeDatabases: -a %s\n", statusA.LogMessage())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "INFO mergeDatabases: loaded %d from %s\n", len(statusA.MPs), *pathA)

	statusB := db.ValidateMPJSONFile(*pathB)
	if !statusB.Valid() {
		fmt.Fprintf(os.Stderr, "ERROR mergeDatabases: -b %s\n", statusB.LogMessage())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "INFO mergeDatabases: loaded %d from %s\n", len(statusB.MPs), *pathB)

	before := len(statusA.MPs) + len(statusB.MPs)
	result := data.MergeMPs(statusA.MPs, statusB.MPs)
	for _, name := range result.MergedNames {
		fmt.Fprintf(os.Stderr, "INFO mergeDatabases: merged name=%q (party details from later record; domains cleared)\n", name)
	}
	fmt.Fprintf(os.Stderr, "INFO mergeDatabases: collapsed %d records into %d unique names (%d name merges)\n",
		before, len(result.MPs), len(result.MergedNames))

	if err := db.WriteMpsTo(*outPath, result.MPs); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR mergeDatabases: write %s: %v\n", *outPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "INFO mergeDatabases: wrote %d MPs to %s\n", len(result.MPs), *outPath)
}

// ensureOutMissing returns an error if path already exists so we never overwrite a DB.
func ensureOutMissing(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("output %s already exists (refusing to overwrite)", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat output %s: %w", path, err)
	}
	return nil
}
