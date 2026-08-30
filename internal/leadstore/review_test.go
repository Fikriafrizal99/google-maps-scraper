package leadstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewPersistsAcrossLeadRefresh(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "leads.csv")
	write := func(phone string) {
		t.Helper()
		content := "place_id,title,address,phone,review_rating\n" +
			"p1,Kost A,Jakarta," + phone + ",4.5\n"
		if err := os.WriteFile(csvPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	store, err := Open(filepath.Join(dir, "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.EnsureReviewSchema(ctx); err != nil {
		t.Fatal(err)
	}

	write("0811")
	if _, err := store.ImportCSV(ctx, csvPath, "kost", "jakarta", ""); err != nil {
		t.Fatal(err)
	}
	leads, err := store.List(ctx, Filter{Preset: "kost", Limit: 10})
	if err != nil || len(leads) != 1 {
		t.Fatalf("expected one lead, got %d err=%v", len(leads), err)
	}
	id := leads[0].ID
	if err := store.UpdateReview(ctx, id, ReviewValid, "nomor aktif"); err != nil {
		t.Fatal(err)
	}

	write("0822")
	if _, err := store.ImportCSV(ctx, csvPath, "kost", "jakarta", ""); err != nil {
		t.Fatal(err)
	}
	review, err := store.GetReview(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != ReviewValid || review.Note != "nomor aktif" {
		t.Fatalf("review was not preserved: %+v", review)
	}
}

func TestReviewMapDefaultsToUnreviewed(t *testing.T) {
	t.Parallel()
	store, err := Open(filepath.Join(t.TempDir(), "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	reviews, err := store.ReviewMap(context.Background(), []int64{10, 11})
	if err != nil {
		t.Fatal(err)
	}
	if reviews[10].Status != ReviewUnreviewed || reviews[11].Status != ReviewUnreviewed {
		t.Fatalf("unexpected defaults: %+v", reviews)
	}
}

func TestNormalizeReviewStatusRejectsUnknown(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeReviewStatus("approved"); err == nil {
		t.Fatal("expected unknown status to fail")
	}
}
