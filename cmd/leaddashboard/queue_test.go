package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func TestFilterQueueRowsPendingSkipsVerifiedAndExclude(t *testing.T) {
	rows := []dashboardRow{
		{Lead: leadstore.Lead{ID: 1}, Enrichment: leadstore.KostEnrichment{VerificationStatus: leadstore.VerificationUnverified}, Review: leadstore.Review{Status: leadstore.ReviewUnreviewed}},
		{Lead: leadstore.Lead{ID: 2}, Enrichment: leadstore.KostEnrichment{VerificationStatus: leadstore.VerificationNeedsCheck}, Review: leadstore.Review{Status: leadstore.ReviewNeedsReview}},
		{Lead: leadstore.Lead{ID: 3}, Enrichment: leadstore.KostEnrichment{VerificationStatus: leadstore.VerificationVerified}, Review: leadstore.Review{Status: leadstore.ReviewValid}},
		{Lead: leadstore.Lead{ID: 4}, Enrichment: leadstore.KostEnrichment{VerificationStatus: leadstore.VerificationUnverified}, Review: leadstore.Review{Status: leadstore.ReviewExclude}},
	}

	got := filterQueueRows(rows, "", true)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("unexpected pending rows: %+v", got)
	}
}

func TestFilterQueueRowsExplicitVerification(t *testing.T) {
	rows := []dashboardRow{
		{Lead: leadstore.Lead{ID: 1}, Enrichment: leadstore.KostEnrichment{VerificationStatus: leadstore.VerificationUnverified}, Review: leadstore.Review{Status: leadstore.ReviewUnreviewed}},
		{Lead: leadstore.Lead{ID: 2}, Enrichment: leadstore.KostEnrichment{VerificationStatus: leadstore.VerificationVerified}, Review: leadstore.Review{Status: leadstore.ReviewValid}},
	}
	got := filterQueueRows(rows, leadstore.VerificationVerified, false)
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("unexpected verified rows: %+v", got)
	}
}

func TestQueueURLPreservesFilter(t *testing.T) {
	values := url.Values{
		"preset":  {"kost"},
		"subarea": {"Jakarta Selatan"},
		"segment": {"putri"},
	}
	got := queueURL(values, 77)
	for _, want := range []string{"/queue?", "id=77", "preset=kost", "segment=putri", "subarea=Jakarta+Selatan"} {
		if !strings.Contains(got, want) {
			t.Fatalf("queue URL %q missing %q", got, want)
		}
	}
}

func TestSafeQueueRedirect(t *testing.T) {
	if got := safeQueueRedirect("/queue?preset=kost&id=2"); got == "" {
		t.Fatal("expected internal queue redirect")
	}
	for _, raw := range []string{"https://evil.example/queue", "//evil.example/queue", "/lead/1"} {
		if got := safeQueueRedirect(raw); got != "" {
			t.Fatalf("expected %q to be rejected, got %q", raw, got)
		}
	}
}
