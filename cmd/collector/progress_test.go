package main

import (
	"os"
	"path/filepath"
	"testing"
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
