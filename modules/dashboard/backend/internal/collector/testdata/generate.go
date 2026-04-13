//go:build ignore

// generate.go writes three overlapping paginated AdGuard querylog fixtures.
// Run with `go run generate.go` from this directory to (re)create:
//
//	querylog_page1.json   t=1000..801 (200 entries)
//	querylog_page2.json   t=830..631  (200 entries; overlap with page 1: t=830..801 = 30)
//	querylog_page3.json   t=660..511  (150 entries; overlap with page 2: t=660..631 = 30)
//
// Total unique timestamps across all three pages: 490.
//
// Base time is 2026-04-13T14:00:00Z; each integer tick = 1 second after base.
// Fewer than PageSize=200 rows in page 3 lets the ingest loop terminate cleanly.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type question struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type answer struct {
	Value string `json:"value"`
	TTL   uint32 `json:"ttl"`
}

type entry struct {
	Time      string   `json:"time"`
	QuestionQ question `json:"question"`
	Answer    []answer `json:"answer"`
	Client    string   `json:"client"`
	Upstream  string   `json:"upstream"`
	ElapsedMs float64  `json:"elapsedMs"`
	Reason    string   `json:"reason"`
}

type page struct {
	Oldest string  `json:"oldest"`
	Data   []entry `json:"data"`
}

func main() {
	base := time.Date(2026, time.April, 13, 14, 0, 0, 0, time.UTC)

	build := func(newestTick, oldestTick int, filename string) {
		if newestTick < oldestTick {
			panic("newestTick must be >= oldestTick (entries sorted newest-first)")
		}
		count := newestTick - oldestTick + 1
		data := make([]entry, 0, count)
		for tick := newestTick; tick >= oldestTick; tick-- {
			ts := base.Add(time.Duration(tick) * time.Second)
			data = append(data, entry{
				Time:      ts.Format(time.RFC3339Nano),
				QuestionQ: question{Name: fmt.Sprintf("host-%d.example", tick), Type: "A"},
				Answer:    []answer{},
				Client:    "192.168.1.42",
				Upstream:  "tls://dns.quad9.net:853",
				ElapsedMs: 1.23,
				Reason:    "NotFiltered",
			})
		}
		oldestTs := base.Add(time.Duration(oldestTick) * time.Second)
		p := page{
			Oldest: oldestTs.Format(time.RFC3339Nano),
			Data:   data,
		}
		buf, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filename, buf, 0o644); err != nil {
			panic(err)
		}
		fmt.Printf("wrote %s (%d entries)\n", filename, count)
	}

	// Page 1: newest page (no older_than filter in real AdGuard)
	build(1000, 801, "querylog_page1.json")
	// Page 2: overlaps page 1 at t=830..801 (30 entries)
	build(830, 631, "querylog_page2.json")
	// Page 3: overlaps page 2 at t=660..631 (30 entries); 150 entries lets
	// the paginator stop on len(rows) < PageSize (=200).
	build(660, 511, "querylog_page3.json")
}
