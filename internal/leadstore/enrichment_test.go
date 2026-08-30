package leadstore

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEvaluateKostEnrichmentSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lead Lead
		want string
	}{
		{"putri", Lead{ID: 1, Title: "Kost Putri Tebet HK"}, "putri"},
		{"putra", Lead{ID: 2, Title: "Rumah Kost Putra Exclusive"}, "putra"},
		{"pasutri", Lead{ID: 3, Title: "Kost Pasutri Mampang"}, "pasutri"},
		{"campur", Lead{ID: 4, Title: "Kos Campur Bulanan"}, "campur"},
		{"putra putri", Lead{ID: 5, Title: "Kost Putra Putri Setiabudi"}, "campur"},
		{"unknown", Lead{ID: 6, Title: "Kost Melati"}, EnrichmentUnknown},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := EvaluateKostEnrichment(tt.lead)
			if e.Segment != tt.want {
				t.Fatalf("segment = %q, want %q", e.Segment, tt.want)
			}
		})
	}
}

func TestEvaluateKostEnrichmentDoesNotGuess(t *testing.T) {
	t.Parallel()
	e := EvaluateKostEnrichment(Lead{
		ID:       1,
		Title:    "Kost Melati Residence",
		Category: "Guest house",
		Address:  "Tebet, Jakarta Selatan",
	})
	if e.Segment != EnrichmentUnknown || e.Target != EnrichmentUnknown || e.RentalType != EnrichmentUnknown {
		t.Fatalf("ambiguous lead should remain unknown: %+v", e)
	}
	if e.PriceRange != EnrichmentUnknown || e.Landmark != EnrichmentUnknown {
		t.Fatalf("price/landmark must not be invented: %+v", e)
	}
}

func TestEvaluateKostEnrichmentFacilitiesAndRental(t *testing.T) {
	t.Parallel()
	e := EvaluateKostEnrichment(Lead{
		ID:    1,
		Title: "Kost Putri Bulanan AC WiFi Kamar Mandi Dalam Pet Friendly",
	})
	if e.Segment != "putri" {
		t.Fatalf("unexpected segment: %q", e.Segment)
	}
	if e.RentalType != "bulanan" {
		t.Fatalf("unexpected rental type: %q", e.RentalType)
	}
	if e.Facilities == EnrichmentUnknown {
		t.Fatal("expected facilities to be detected")
	}
	if e.Rules == EnrichmentUnknown {
		t.Fatal("expected pet friendly rule to be detected")
	}
}

func TestManualEnrichmentBeatsAuto(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	manual := defaultKostEnrichment(10)
	manual.Segment = "putri"
	manual.Target = "karyawan"
	manual.VerificationStatus = VerificationVerified
	if err := store.UpdateEnrichment(ctx, manual); err != nil {
		t.Fatal(err)
	}

	auto := defaultKostEnrichment(10)
	auto.Segment = "putra"
	if err := store.updateAutoEnrichment(ctx, auto); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetEnrichment(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != EnrichmentManual || got.Segment != "putri" || got.Target != "karyawan" {
		t.Fatalf("manual enrichment was overwritten: %+v", got)
	}
}

func TestEnrichmentMapDefaultsToUnknown(t *testing.T) {
	t.Parallel()
	store, err := Open(filepath.Join(t.TempDir(), "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	items, err := store.EnrichmentMap(context.Background(), []int64{10, 11})
	if err != nil {
		t.Fatal(err)
	}
	if items[10].Segment != EnrichmentUnknown || items[11].VerificationStatus != VerificationUnverified {
		t.Fatalf("unexpected defaults: %+v", items)
	}
}
