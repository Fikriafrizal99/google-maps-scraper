package main

import (
	"strings"
	"testing"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func TestCustomerSafeRowsDropsExcluded(t *testing.T) {
	t.Parallel()
	rows := []dashboardRow{
		{Lead: leadstore.Lead{ID: 1, Title: "Kost A"}, Review: leadstore.Review{Status: leadstore.ReviewValid}},
		{Lead: leadstore.Lead{ID: 2, Title: "Kost B"}, Review: leadstore.Review{Status: leadstore.ReviewExclude}},
		{Lead: leadstore.Lead{ID: 3, Title: "Kost C"}, Review: leadstore.Review{Status: leadstore.ReviewUnreviewed}},
	}
	got := customerSafeRows(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 customer rows, got %d", len(got))
	}
	for _, row := range got {
		if row.Review.Status == leadstore.ReviewExclude {
			t.Fatal("excluded lead leaked into customer export")
		}
	}
}

func TestCustomerExportHeaderHasNoInternalFields(t *testing.T) {
	t.Parallel()
	forbidden := map[string]bool{
		"place_id": true, "data_id": true, "source_key": true, "images": true,
		"review_status": true, "review_note": true, "reviewed_at": true,
		"first_seen": true, "last_checked": true, "enrichment_source": true,
	}
	for _, field := range customerExportHeader {
		if forbidden[field] {
			t.Fatalf("internal field %q must not be in customer export", field)
		}
	}
}

func TestCustomerExportUnknownBecomesBlank(t *testing.T) {
	t.Parallel()
	row := dashboardRow{
		Lead: leadstore.Lead{Title: "Kost Putri Tebet", Rating: 4.8, ReviewCount: 20},
		Enrichment: leadstore.KostEnrichment{
			Segment: leadstore.EnrichmentUnknown,
			Target:  "mahasiswa",
			VerificationStatus: leadstore.VerificationVerified,
		},
	}
	record := customerExportRecord(row)
	if len(record) != len(customerExportHeader) {
		t.Fatalf("record/header mismatch: %d vs %d", len(record), len(customerExportHeader))
	}
	joined := strings.Join(record, "|")
	if strings.Contains(joined, leadstore.EnrichmentUnknown) {
		t.Fatal("unknown marker should be blank in customer export")
	}
}
