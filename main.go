package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

func main() {
	agentFlag := flag.Bool("agent", false, "per-agent count table")
	dayFlag := flag.Bool("day", false, "per-day count table")
	monthFlag := flag.Bool("month", false, "per-month count table")
	jsonFlag := flag.Bool("json", false, "emit JSON instead of tables")
	sinceFlag := flag.String("since", "", "only records on/after DATE (YYYY-MM-DD)")
	untilFlag := flag.String("until", "", "only records on/before DATE (YYYY-MM-DD)")
	noFetch := flag.Bool("no-fetch", false, "offline pricing only (never touch network)")
	refresh := flag.Bool("refresh", false, "manually refresh prices now (force re-fetch)")
	flag.Parse()

	allFlag := !*agentFlag && !*dayFlag && !*monthFlag

	var since, until time.Time
	if *sinceFlag != "" {
		t, err := time.Parse("2006-01-02", *sinceFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --since date: %v\n", err)
			os.Exit(1)
		}
		since = t
	}
	if *untilFlag != "" {
		t, err := time.Parse("2006-01-02", *untilFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --until date: %v\n", err)
			os.Exit(1)
		}
		until = t.Add(24*time.Hour - time.Nanosecond) // end of day inclusive
	}

	provider, err := NewProvider(*refresh, *noFetch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pricing init: %v\n", err)
		os.Exit(1)
	}

	type job struct {
		parser Parser
		path   string
	}
	var jobs []job
	for _, p := range All() {
		paths, err := p.Discover()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s discover: %v\n", p.Name(), err)
			continue
		}
		for _, fp := range paths {
			jobs = append(jobs, job{p, fp})
		}
	}

	// Parse files in parallel; each job writes to its own slot so the merge stays deterministic.
	parsed := make([][]Record, len(jobs))
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, j job) {
			defer wg.Done()
			defer func() { <-sem }()
			records, err := j.parser.Parse(j.path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s parse %s: %v\n", j.parser.Name(), j.path, err)
				return
			}
			kept := records[:0]
			for _, r := range records {
				if !since.IsZero() && r.Timestamp.Before(since) {
					continue
				}
				if !until.IsZero() && r.Timestamp.After(until) {
					continue
				}
				kept = append(kept, r)
			}
			parsed[i] = kept
		}(i, j)
	}
	wg.Wait()

	var allRecords []Record
	for _, records := range parsed {
		allRecords = append(allRecords, records...)
	}

	res := Compute(allRecords, provider)

	if *jsonFlag {
		if err := JSON(os.Stdout, res); err != nil {
			fmt.Fprintf(os.Stderr, "json: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if allFlag || *agentFlag {
		fmt.Println("# By Agent")
		AgentTable(os.Stdout, res)
		fmt.Println()
	}
	if allFlag || *dayFlag {
		fmt.Println("# By Day")
		DayTable(os.Stdout, res)
		fmt.Println()
	}
	if allFlag || *monthFlag {
		fmt.Println("# By Month")
		MonthTable(os.Stdout, res)
		fmt.Println()
	}
	Footer(os.Stdout, res)
}
