package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCountCSVDataRows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "rows.csv")
	content := "title,address\nKost A,Jakarta\n\"Kost B\",\"Alamat baris 1\nAlamat baris 2\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if got := countCSVDataRows(path); got != 2 {
		t.Fatalf("countCSVDataRows() = %d, want 2", got)
	}
}

func TestCountCSVDataRowsMissingFile(t *testing.T) {
	t.Parallel()
	if got := countCSVDataRows(filepath.Join(t.TempDir(), "missing.csv")); got != 0 {
		t.Fatalf("missing file count = %d, want 0", got)
	}
}

func TestShouldDeclareStallWithExistingRows(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 11, 20, 20, 0, 0, time.Local)
	if shouldDeclareStall(start, start.Add(2*time.Minute+59*time.Second), 3*time.Minute, 85) {
		t.Fatal("declared stall too early")
	}
	if !shouldDeclareStall(start, start.Add(3*time.Minute), 3*time.Minute, 85) {
		t.Fatal("expected stall after 3 minutes without new rows")
	}
}

func TestShouldDeclareStallGivesEmptyResultExtraGrace(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 11, 20, 20, 0, 0, time.Local)
	if shouldDeclareStall(start, start.Add(5*time.Minute), 3*time.Minute, 0) {
		t.Fatal("zero-row scrape should get double stall grace")
	}
	if !shouldDeclareStall(start, start.Add(6*time.Minute), 3*time.Minute, 0) {
		t.Fatal("expected zero-row scrape to stall after double grace")
	}
}

func TestIdleSecondsSince(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 11, 20, 20, 0, 0, time.Local)
	if got := idleSecondsSince(start, start.Add(95*time.Second)); got != 95 {
		t.Fatalf("idleSecondsSince() = %d, want 95", got)
	}
}
