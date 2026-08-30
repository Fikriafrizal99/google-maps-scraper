package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func main() {
	dbPath := flag.String("db", filepath.FromSlash("data/leads.db"), "SQLite master database path")
	preset := flag.String("preset", "kost", "preset/niche to enrich")
	area := flag.String("area", "", "optional area filter")
	subarea := flag.String("subarea", "", "optional subarea filter")
	apply := flag.Bool("apply", false, "write auto enrichment to enrichment store")
	flag.Parse()

	store, err := leadstore.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	summary, err := store.RunAutoEnrichment(context.Background(), leadstore.Filter{
		Preset:  *preset,
		Area:    *area,
		Subarea: *subarea,
		Limit:   5000,
	}, *apply)
	if err != nil {
		log.Fatal(err)
	}

	mode := "PREVIEW"
	if *apply {
		mode = "APPLIED"
	}
	fmt.Printf("AUTO ENRICHMENT %s\n", mode)
	fmt.Printf("Total        : %d\n", summary.Total)
	fmt.Printf("Ada sinyal   : %d\n", summary.Detected)
	fmt.Printf("Tanpa sinyal : %d\n", summary.NoSignal)
	if *apply {
		fmt.Printf("Auto updated : %d\n", summary.Applied)
		fmt.Printf("Manual kept  : %d\n", summary.ManualKept)
	}
	if !*apply {
		fmt.Println("\nPreview only. Data belum diubah. Jalankan lagi dengan -apply jika hasilnya sudah masuk akal.")
	}
}
