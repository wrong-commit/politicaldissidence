package main

import (
	"flag"
	"fmt"
	"os"

	"politicaldissidence/csv"
	"politicaldissidence/data"
	"politicaldissidence/db"
)

func main() {
	senatorsPath := flag.String("senators", "", "path to senators CSV (e.g. allsenel.csv)")
	membersPath := flag.String("members", "", "path to House of Reps CSV (e.g. FamilynameRepsCSV.csv)")
	outPath := flag.String("o", "mps_from_csv.json", "output JSON path (does not overwrite mp_data.json by default)")
	flag.Parse()

	if *senatorsPath == "" && *membersPath == "" {
		fmt.Fprintf(os.Stderr, "ERROR csv2json: provide -senators and/or -members CSV path\n")
		flag.Usage()
		os.Exit(2)
	}

	var mps []data.MP

	if *senatorsPath != "" {
		senators, err := csv.ParseSenatorFile(*senatorsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR csv2json: senators %s: %v\n", *senatorsPath, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "INFO csv2json: parsed %d senators from %s\n", len(senators), *senatorsPath)
		mps = append(mps, senators...)
	}

	if *membersPath != "" {
		members, err := csv.ParseMemberFile(*membersPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR csv2json: members %s: %v\n", *membersPath, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "INFO csv2json: parsed %d members from %s\n", len(members), *membersPath)
		mps = append(mps, members...)
	}

	if err := db.WriteMpsTo(*outPath, mps); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR csv2json: write %s: %v\n", *outPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "INFO csv2json: wrote %d MPs to %s\n", len(mps), *outPath)
}
