package main

import (
	"flag"
	"fmt"
	"os"

	"politicaldissidence/csvrefresh"
	"politicaldissidence/data"
	"politicaldissidence/db"
)

func main() {
	configPath := flag.String("config", csvrefresh.DefaultConfigPath, "path to csv_refresh.json")
	mpPath := flag.String("mp", "mp_data.json", "MP JSON to merge against (use empty string for none)")
	verbose := flag.Bool("v", false, "print debug lines (resolved download URL, etc.)")
	flag.Parse()

	cfg, err := csvrefresh.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR csvrefresh: config %v\n", err)
		os.Exit(2)
	}

	current, err := loadMPs(*mpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR csvrefresh: load MPs: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "INFO csvrefresh: config=%s entries=%d interval=%s currentMPs=%d\n",
		*configPath, len(cfg.Entries), cfg.Interval, len(current))
	for i, e := range cfg.Entries {
		fmt.Fprintf(os.Stderr, "INFO csvrefresh: entry[%d] format=%s filename=%s url=%s\n",
			i, e.NormalizedFormat, e.CSVFilename, e.CSVSourceURL)
	}

	log := csvrefresh.LogFn{
		OnInfo: func(msg string) {
			fmt.Fprintln(os.Stderr, msg)
		},
		OnDebug: func(msg string) {
			if *verbose {
				fmt.Fprintln(os.Stderr, msg)
			}
		},
		OnError: func(msg string) {
			fmt.Fprintln(os.Stderr, msg)
		},
	}

	var appliedN int
	res := csvrefresh.Run(csvrefresh.Deps{
		Log:    log,
		Config: cfg,
		Current: func() []data.MP {
			return current
		},
		Apply: func(merged []data.MP) {
			appliedN = len(merged)
			current = merged
		},
	})

	if res.Skipped {
		os.Exit(1)
	}
	if !res.Applied {
		// all entries failed (errors already logged)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "INFO csvrefresh: done parsed=%d added=%d merged=%d entryFails=%d inMemory=%d (not saved)\n",
		res.ParsedCount, res.AddedCount, res.MergedCount, res.EntryFails, appliedN)
}

func loadMPs(path string) ([]data.MP, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "INFO csvrefresh: no MP file at %s; merging against empty list\n", path)
			return nil, nil
		}
		return nil, err
	}
	// Prefer live DB helper when using the default filename.
	if path == "mp_data.json" {
		status := db.ReadMpsValidated()
		if !status.Valid() {
			return nil, fmt.Errorf("%s", status.LogMessage())
		}
		return status.MPs, nil
	}
	status := db.ValidateMPJSONFile(path)
	if !status.Valid() {
		return nil, fmt.Errorf("%s", status.LogMessage())
	}
	return status.MPs, nil
}
