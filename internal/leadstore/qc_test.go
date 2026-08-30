package leadstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateKostQCWrongCategoryStillValid(t *testing.T) {
	t.Parallel()
	cases := []Lead{
		{Preset: "kost", Title: "Kost Exclusive Wall Living", Category: "Motel", Address: "Jakarta Selatan", Phone: "0812"},
		{Preset: "kost", Title: "Rumah Kost Premium Radio Dalam", Category: "Homestay", Address: "Jakarta Selatan"},
		{Preset: "kost", Title: "Kost Putri Tebet HK", Category: "Guest house", Address: "Tebet", Phone: "0813"},
	}
	for _, lead := range cases {
		eval := EvaluateLeadQC(lead)
		if eval.Status != ReviewValid {
			t.Fatalf("expected %q (%s) to be valid, got %+v", lead.Title, lead.Category, eval)
		}
	}
}

func TestEvaluateKostQCAmbiguousNeedsReview(t *testing.T) {
	t.Parallel()
	lead := Lead{Preset: "kost", Title: "The Executive House Setiabudi", Category: "Guest house", Address: "Jakarta Selatan", Phone: "0812"}
	eval := EvaluateLeadQC(lead)
	if eval.Status != ReviewNeedsReview {
		t.Fatalf("expected ambiguous house to need review, got %+v", eval)
	}
}

func TestEvaluateKostQCObviousHotelExcluded(t *testing.T) {
	t.Parallel()
	lead := Lead{Preset: "kost", Title: "OYO Hotel Melati Jakarta", Category: "Hotel", Address: "Jakarta Selatan"}
	eval := EvaluateLeadQC(lead)
	if eval.Status != ReviewExclude {
		t.Fatalf("expected obvious hotel to be excluded, got %+v", eval)
	}
}

func TestEvaluateKostQCHotelWordDoesNotOverrideKost(t *testing.T) {
	t.Parallel()
	lead := Lead{Preset: "kost", Title: "Kost Hotel Style Residence", Category: "Motel", Address: "Jakarta Selatan", Phone: "0812"}
	eval := EvaluateLeadQC(lead)
	if eval.Status != ReviewValid {
		t.Fatalf("kost keyword must outweigh category/non-kost word, got %+v", eval)
	}
}

func TestRunAutoQCPreservesManualReview(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "leads.csv")
	content := "place_id,title,category,address,phone,review_rating,review_count\n" +
		"p1,Kost Exclusive A,Motel,Jakarta Selatan,0812,4.5,20\n" +
		"p2,OYO Hotel B,Hotel,Jakarta Selatan,,4.0,10\n"
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
	if len(leads) != 2 {
		t.Fatalf("expected 2 leads, got %d", len(leads))
	}

	var hotelID int64
	for _, lead := range leads {
		if lead.Title == "OYO Hotel B" {
			hotelID = lead.ID
		}
	}
	if err := store.UpdateReview(context.Background(), hotelID, ReviewValid, "manual override"); err != nil {
		t.Fatal(err)
	}

	summary, err := store.RunAutoQC(context.Background(), Filter{Preset: "kost", Limit: 10}, true)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ManualKept != 1 {
		t.Fatalf("expected one manual review preserved, got %+v", summary)
	}
	review, err := store.GetReview(context.Background(), hotelID)
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != ReviewValid || review.Source != ReviewSourceManual {
		t.Fatalf("manual review was overwritten: %+v", review)
	}
}
