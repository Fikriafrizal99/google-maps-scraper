package leadstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImportCSVAndList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "leads.csv")
	content := "place_id,data_id,title,category,address,phone,website,latitude,longitude,review_rating,review_count,link,thumbnail\n" +
		"p1,d1,Kost Tebet,Kost,Tebet Jakarta Selatan,08123456789,https://example.com,-6.2,106.8,4.7,123,https://maps.example/p1,https://img.example/p1.jpg\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(filepath.Join(dir, "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	count, err := store.ImportCSV(context.Background(), csvPath, "kost", "jakarta", "Jakarta Selatan")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 imported row, got %d", count)
	}

	leads, err := store.List(context.Background(), Filter{Preset: "kost", HasPhone: true, MinRating: 4, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("expected 1 lead, got %d", len(leads))
	}
	if leads[0].Title != "Kost Tebet" {
		t.Fatalf("unexpected title %q", leads[0].Title)
	}
	if leads[0].Thumbnail == "" {
		t.Fatal("expected thumbnail to be stored")
	}

	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 1 || stats.WithPhone != 1 || stats.WithWebsite != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestImportCSVUpsertsSamePresetLead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "leads.csv")
	write := func(phone string) {
		t.Helper()
		content := "place_id,title,address,phone,review_rating,review_count\n" +
			"p1,Kost A,Jakarta," + phone + ",4.5,10\n"
		if err := os.WriteFile(csvPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	store, err := Open(filepath.Join(dir, "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	write("0811")
	if _, err := store.ImportCSV(context.Background(), csvPath, "kost", "jakarta", ""); err != nil {
		t.Fatal(err)
	}
	write("0822")
	if _, err := store.ImportCSV(context.Background(), csvPath, "kost", "jakarta", ""); err != nil {
		t.Fatal(err)
	}

	leads, err := store.List(context.Background(), Filter{Preset: "kost", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("expected upsert to keep 1 lead, got %d", len(leads))
	}
	if leads[0].Phone != "0822" {
		t.Fatalf("expected refreshed phone, got %q", leads[0].Phone)
	}
}

func TestListMatchesParentAdministrativeScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "leads.csv")
	content := "place_id,title,address,phone\n" +
		"p1,Bengkel Maju,Cugenang Cianjur,08123456789\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(filepath.Join(dir, "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	villagePath := "SUKAMULYA, CUGENANG, KABUPATEN CIANJUR, JAWA BARAT, Indonesia"
	if _, err := store.ImportCSV(context.Background(), csvPath, "b2b-prospecting", "java-sumatra", villagePath); err != nil {
		t.Fatal(err)
	}

	for _, scope := range []string{
		"JAWA BARAT, Indonesia",
		"KABUPATEN CIANJUR, JAWA BARAT, Indonesia",
		"CUGENANG, KABUPATEN CIANJUR, JAWA BARAT, Indonesia",
		villagePath,
	} {
		leads, err := store.List(context.Background(), Filter{Area: "java-sumatra", Subarea: scope, Limit: 10})
		if err != nil {
			t.Fatalf("List(%q) error = %v", scope, err)
		}
		if len(leads) != 1 {
			t.Fatalf("List(%q) len = %d, want 1", scope, len(leads))
		}
	}
}
