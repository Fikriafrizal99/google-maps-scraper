package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/internal/collectorconfig"
	"github.com/gosom/google-maps-scraper/internal/collectorpost"
	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func main() {
	var (
		presetName = flag.String("preset", "", "preset name from config/presets, e.g. kost")
		areaName   = flag.String("area", "", "area name from config/areas, e.g. jakarta")
		subarea    = flag.String("subarea", "", "optional static subarea name, e.g. Jakarta Selatan")
		location   = flag.String("location", "", "optional resolved administrative location from province to village")
		configDir  = flag.String("config-dir", "config", "collector config directory")
		engine     = flag.String("engine", filepath.FromSlash("bin/google_maps_scraper"), "path to google_maps_scraper binary")
		output     = flag.String("output", "collector-results.csv", "filtered/deduplicated CSV output")
		dbPath     = flag.String("db", filepath.FromSlash("data/leads.db"), "SQLite master lead database")
		noDB       = flag.Bool("no-db", false, "skip importing results into the master database")
		keepRaw    = flag.Bool("keep-raw", false, "keep temporary raw CSV and query file")
	)
	flag.Parse()

	if *presetName == "" || *areaName == "" {
		fatalf("-preset and -area are required")
	}

	presetPath := filepath.Join(*configDir, "presets", *presetName+".json")
	areaPath := filepath.Join(*configDir, "areas", *areaName+".json")

	preset, err := collectorconfig.LoadPreset(presetPath)
	if err != nil {
		fatalf("load preset: %v", err)
	}
	area, err := collectorconfig.LoadArea(areaPath)
	if err != nil {
		fatalf("load area: %v", err)
	}

	resolvedLocation := strings.TrimSpace(*location)
	var queries []string
	if resolvedLocation != "" {
		queries, err = collectorconfig.BuildQueriesForLocation(preset, resolvedLocation)
	} else {
		queries, err = collectorconfig.BuildQueries(preset, area, *subarea)
	}
	if err != nil {
		fatalf("build queries: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "gmaps-collector-*")
	if err != nil {
		fatalf("create temp dir: %v", err)
	}
	if !*keepRaw {
		defer os.RemoveAll(tmpDir)
	}

	queryFile := filepath.Join(tmpDir, "queries.txt")
	rawFile := filepath.Join(tmpDir, "raw.csv")
	if err := writeQueries(queryFile, queries); err != nil {
		fatalf("write queries: %v", err)
	}

	args := []string{"-input", queryFile, "-results", rawFile}
	args = append(args, flag.Args()...)

	fmt.Printf("Collector preset=%s area=%s queries=%d\n", preset.Name, area.DisplayName, len(queries))
	if resolvedLocation != "" {
		fmt.Printf("Location: %s\n", resolvedLocation)
	} else if *subarea != "" {
		fmt.Printf("Subarea: %s\n", *subarea)
	}
	printProgress("starting", len(queries), 0, 0, 0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmd := exec.CommandContext(ctx, *engine, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		printProgress("failed", len(queries), 0, 0, 0)
		fatalf("scraper failed to start: %v", err)
	}

	printProgress("scraping", len(queries), 0, 0, 0)
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var runErr error
scrapeLoop:
	for {
		select {
		case runErr = <-waitCh:
			break scrapeLoop
		case <-ticker.C:
			rawRows := countCSVDataRows(rawFile)
			printProgress("scraping", len(queries), rawRows, 0, 0)
		}
	}

	rawRows := countCSVDataRows(rawFile)
	if ctx.Err() != nil {
		printProgress("cancelled", len(queries), rawRows, 0, 0)
		fmt.Fprintln(os.Stderr, "collector: cancelled")
		return
	}
	if runErr != nil {
		printProgress("failed", len(queries), rawRows, 0, 0)
		fatalf("scraper failed: %v", runErr)
	}

	printProgress("processing", len(queries), rawRows, 0, 0)
	if err := collectorpost.ProcessCSV(rawFile, *output, preset); err != nil {
		printProgress("failed", len(queries), rawRows, 0, 0)
		fatalf("post-process results: %v", err)
	}
	finalRows := countCSVDataRows(*output)
	fmt.Printf("CSV: %s\n", *output)

	importedRows := 0
	if !*noDB {
		printProgress("importing", len(queries), rawRows, finalRows, 0)
		dir := filepath.Dir(*dbPath)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				printProgress("failed", len(queries), rawRows, finalRows, 0)
				fatalf("create database directory: %v", err)
			}
		}
		store, err := leadstore.Open(*dbPath)
		if err != nil {
			printProgress("failed", len(queries), rawRows, finalRows, 0)
			fatalf("open master database: %v", err)
		}
		storedSubarea := strings.TrimSpace(*subarea)
		if resolvedLocation != "" {
			storedSubarea = resolvedLocation
		}
		count, importErr := store.ImportCSV(context.Background(), *output, preset.Name, area.Name, storedSubarea)
		closeErr := store.Close()
		if importErr != nil {
			printProgress("failed", len(queries), rawRows, finalRows, 0)
			fatalf("import master database: %v", importErr)
		}
		if closeErr != nil {
			printProgress("failed", len(queries), rawRows, finalRows, count)
			fatalf("close master database: %v", closeErr)
		}
		importedRows = count
		fmt.Printf("Master DB: %s (%d rows processed)\n", *dbPath, count)
	}

	printProgress("completed", len(queries), rawRows, finalRows, importedRows)
	if *keepRaw {
		fmt.Printf("Raw files kept in: %s\n", tmpDir)
	}
}

func printProgress(stage string, queries, rawRows, finalRows, importedRows int) {
	fmt.Printf(
		"KOST_PROGRESS stage=%s queries=%d raw_rows=%d final_rows=%d imported_rows=%d\n",
		stage, queries, rawRows, finalRows, importedRows,
	)
}

func countCSVDataRows(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		records++
	}
	if records <= 1 {
		return 0
	}
	return records - 1
}

func writeQueries(path string, queries []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, query := range queries {
		if _, err := writer.WriteString(strings.TrimSpace(query) + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "collector: "+format+"\n", args...)
	os.Exit(1)
}
