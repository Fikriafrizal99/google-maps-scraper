package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gosom/google-maps-scraper/internal/collectorconfig"
	"github.com/gosom/google-maps-scraper/internal/collectorpost"
	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func main() {
	var (
		presetName = flag.String("preset", "", "preset name from config/presets, e.g. kost")
		areaName   = flag.String("area", "", "area name from config/areas, e.g. jakarta")
		subarea    = flag.String("subarea", "", "optional subarea name, e.g. Jakarta Selatan")
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
	queries, err := collectorconfig.BuildQueries(preset, area, *subarea)
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
	if *subarea != "" {
		fmt.Printf("Subarea: %s\n", *subarea)
	}

	cmd := exec.Command(*engine, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fatalf("scraper failed: %v", err)
	}

	if err := collectorpost.ProcessCSV(rawFile, *output, preset); err != nil {
		fatalf("post-process results: %v", err)
	}

	fmt.Printf("CSV: %s\n", *output)

	if !*noDB {
		dir := filepath.Dir(*dbPath)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fatalf("create database directory: %v", err)
			}
		}
		store, err := leadstore.Open(*dbPath)
		if err != nil {
			fatalf("open master database: %v", err)
		}
		count, importErr := store.ImportCSV(context.Background(), *output, preset.Name, area.Name, *subarea)
		closeErr := store.Close()
		if importErr != nil {
			fatalf("import master database: %v", importErr)
		}
		if closeErr != nil {
			fatalf("close master database: %v", closeErr)
		}
		fmt.Printf("Master DB: %s (%d rows processed)\n", *dbPath, count)
	}

	if *keepRaw {
		fmt.Printf("Raw files kept in: %s\n", tmpDir)
	}
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
