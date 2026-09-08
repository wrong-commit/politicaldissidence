package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/db"
)

func main() {
	soonDays := flag.Int("soon-days", int(data.AlertSoonWindow/(24*time.Hour)), "days ahead that count as soon for WHOIS alert")
	delay := flag.Duration("delay", time.Second, "sleep between domain WHOIS+DNS+HTTPS checks")
	dryRun := flag.Bool("dry-run", false, "classify from persisted data only; no network, no save")
	save := flag.Bool("save", true, "persist WHOIS/DNS/HTTPS/alert after live run")
	verbose := flag.Bool("v", false, "debug logging per domain")
	flag.Parse()

	soonWindow := time.Duration(*soonDays) * 24 * time.Hour
	if soonWindow <= 0 {
		soonWindow = data.AlertSoonWindow
	}

	status := db.ReadMpsValidated()
	if status.Err != nil {
		fmt.Fprintf(os.Stderr, "ERROR checkdomains: %s\n", status.LogMessage())
		os.Exit(1)
	}
	mps := status.MPs

	total := 0
	for _, mp := range mps {
		total += len(mp.Domains)
	}
	fmt.Fprintf(os.Stderr, "INFO checkdomains: checking all (%d) MP domains (soon-days=%d)\n", total, *soonDays)

	now := time.Now()
	var (
		alertCount         int
		expiredCount       int
		soonCount          int
		updatedStaleCount  int
		emptyCount         int
		httpsExpiredCount  int
		httpsSoonCount     int
		httpsMissingCount  int
		http404Count       int
		http500Count       int
		errorCount         int
		mutated            bool
	)

	domainIndex := 0
	for i := range mps {
		mp := &mps[i]
		for j := range mp.Domains {
			domainIndex++
			dom := &mp.Domains[j]
			if !*dryRun {
				if _, err := dom.UpdateExpiryInfo(); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR checkdomains: domain %s for MP \"%s\" whois error: %v\n", dom.Hostname, mp.Name(), err)
					errorCount++
				}
				if _, err := dom.UpdateDns(); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR checkdomains: domain %s for MP \"%s\" dns error: %v\n", dom.Hostname, mp.Name(), err)
					errorCount++
				}
				if _, err := dom.UpdateHttps(); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR checkdomains: domain %s for MP \"%s\" https error: %v\n", dom.Hostname, mp.Name(), err)
					errorCount++
				}
				mutated = true
				if domainIndex < total && *delay > 0 {
					time.Sleep(*delay)
				}
			}
			dom.RefreshAlert(now, soonWindow)
			if !*dryRun {
				mutated = true
			}

			reasons := data.AlertReasons(*dom, now, soonWindow)
			if *verbose {
				fmt.Fprintf(os.Stderr, "DEBUG checkdomains: %s %s alert=%v\n", mp.Name(), dom.Hostname, dom.Alert)
			}
			if !dom.Alert {
				continue
			}
			alertCount++
			for _, r := range reasons {
				switch r {
				case data.AlertReasonExpired:
					expiredCount++
				case data.AlertReasonSoon:
					soonCount++
				case data.AlertReasonUpdatedStale:
					updatedStaleCount++
				case data.AlertReasonDNSEmpty:
					emptyCount++
				case data.AlertReasonHTTPSExpired:
					httpsExpiredCount++
				case data.AlertReasonHTTPSSoon:
					httpsSoonCount++
				case data.AlertReasonHTTPSMissing:
					httpsMissingCount++
				case data.AlertReasonHTTP404:
					http404Count++
				case data.AlertReasonHTTP500:
					http500Count++
				}
			}
			dnsOutcome := "?"
			if dom.DNS != nil {
				if dom.DNS.Outcome != "" {
					dnsOutcome = dom.DNS.Outcome
				} else if dom.DNS.Empty {
					dnsOutcome = "empty"
				} else {
					dnsOutcome = "ok"
				}
			}
			httpsStatus := "?"
			if dom.Https != nil && dom.Https.Status != "" {
				httpsStatus = dom.Https.Status
			}
			fmt.Printf("%s\tmp=%s\talert=true\texpiry=%s\treasons=%s\tdns=%s\thttps=%s\n",
				dom.Hostname, mp.Name(), dom.Expiry, strings.Join(reasons, ","), dnsOutcome, httpsStatus)
		}
	}

	if !*dryRun && *save && mutated {
		if err := db.WriteMps(mps); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR checkdomains: save: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "INFO checkdomains: saved mp_data.json\n")
	} else if *dryRun || !*save {
		fmt.Fprintf(os.Stderr, "INFO checkdomains: skip save\n")
	}

	if alertCount == 0 {
		fmt.Println("# no alerts")
	}
	fmt.Fprintf(os.Stderr, "INFO checkdomains: done alert=%d errors=%d\n", alertCount, errorCount)
	fmt.Printf("# checked=%d alert=%d expired=%d soon=%d updated-stale=%d dns-empty=%d https-expired=%d https-soon=%d https-missing=%d http-404=%d http-500=%d errors=%d\n",
		total, alertCount, expiredCount, soonCount, updatedStaleCount, emptyCount, httpsExpiredCount, httpsSoonCount, httpsMissingCount, http404Count, http500Count, errorCount)
}
