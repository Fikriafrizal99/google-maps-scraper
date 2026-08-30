package leadstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGetLead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "leads.csv")
	content := "place_id,title,address,phone,thumbnail\n" +
		"p-detail,Kost Detail,Jakarta Selatan,08123456789,https://img.example/detail.jpg\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(filepath.Join(dir, "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.ImportCSV(context.Background(), csvPath, "kost", "jakarta", "Jakarta Selatan"); err != nil {
		t.Fatal(err)
	}
	leads, err := store.List(context.Background(), Filter{Preset: "kost", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("expected 1 lead, got %d", len(leads))
	}

	lead, err := store.Get(context.Background(), leads[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if lead.Title != "Kost Detail" || lead.Thumbnail == "" {
		t.Fatalf("unexpected lead: %+v", lead)
	}
}
