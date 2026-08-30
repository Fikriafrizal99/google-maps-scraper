package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func TestBuildCustomerWorkbookContainsSummaryAndFilteredLead(t *testing.T) {
	rows := []dashboardRow{
		{
			Lead: leadstore.Lead{
				Title:       "Kost Putri Tebet HK",
				Address:     "Jl. Test Tebet",
				Area:        "jakarta",
				Subarea:     "Jakarta Selatan",
				Phone:       "0812-1186-8446",
				Website:     "https://example.com",
				Rating:      5,
				ReviewCount: 37,
				Link:        "https://maps.google.com/example",
				Thumbnail:   "https://example.com/photo.jpg",
			},
			Enrichment: leadstore.KostEnrichment{
				Segment:            "putri",
				Facilities:         "AC, WiFi, kamar mandi dalam",
				VerificationStatus: leadstore.VerificationVerified,
			},
		},
	}

	filters := url.Values{
		"preset":              {"kost"},
		"area":                {"jakarta"},
		"subarea":             {"Jakarta Selatan"},
		"segment":             {"putri"},
		"verification_status": {"verified"},
	}
	content, err := buildCustomerWorkbook(rows, filters, time.Date(2026, 8, 30, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("build workbook: %v", err)
	}
	if len(content) < 1000 {
		t.Fatalf("workbook unexpectedly small: %d bytes", len(content))
	}

	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open workbook zip: %v", err)
	}
	workbook := zipEntryText(t, zr, "xl/workbook.xml")
	if !strings.Contains(workbook, `name="SUMMARY"`) || !strings.Contains(workbook, `name="LEADS"`) {
		t.Fatalf("expected SUMMARY and LEADS sheets, got %s", workbook)
	}

	summary := zipEntryText(t, zr, "xl/worksheets/sheet1.xml")
	for _, want := range []string{"DATABASE LEADS KOST", "Jakarta Selatan", "Putri", "Terverifikasi"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q", want)
		}
	}

	leads := zipEntryText(t, zr, "xl/worksheets/sheet2.xml")
	for _, want := range []string{"Kost Putri Tebet HK", "AC, WiFi, kamar mandi dalam", "Buka Maps", "Chat WA", `autoFilter ref="A4:T5"`} {
		if !strings.Contains(leads, want) {
			t.Fatalf("leads sheet missing %q", want)
		}
	}
	if !strings.Contains(zipEntryText(t, zr, "xl/worksheets/_rels/sheet2.xml.rels"), "https://wa.me/6281211868446") {
		t.Fatal("expected WhatsApp hyperlink relationship")
	}
}

func TestSafeHTTPURLRejectsNonHTTP(t *testing.T) {
	if got := safeHTTPURL("javascript:alert(1)"); got != "" {
		t.Fatalf("expected unsafe URL to be rejected, got %q", got)
	}
	if got := safeHTTPURL("https://example.com/path?a=1&b=2"); got == "" {
		t.Fatal("expected https URL to be accepted")
	}
}

func zipEntryText(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		r, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", name, err)
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read zip entry %s: %v", name, err)
		}
		return string(data)
	}
	t.Fatalf("zip entry not found: %s", name)
	return ""
}
