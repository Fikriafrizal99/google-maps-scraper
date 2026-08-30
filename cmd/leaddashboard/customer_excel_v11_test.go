package main

import (
	"archive/zip"
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func TestBuildCustomerWorkbookV11PolishesPackage(t *testing.T) {
	rows := []dashboardRow{
		{
			Lead: leadstore.Lead{
				Preset:      "kost",
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
				LastChecked: "2026-08-30T03:26:21Z",
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
		"subarea":             {"Jakarta Selatan"},
		"segment":             {"putri"},
		"verification_status": {"verified"},
	}
	generatedAt := time.Date(2026, 8, 30, 5, 0, 0, 0, time.UTC)
	content, err := buildCustomerWorkbookV11(rows, filters, generatedAt)
	if err != nil {
		t.Fatalf("build v1.1 workbook: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open workbook zip: %v", err)
	}
	summary := zipEntryText(t, zr, "xl/worksheets/sheet1.xml")
	for _, want := range []string{
		"Database Kost Putri Jakarta Selatan",
		"Coverage HP",
		"Coverage Segment",
		"Coverage Fasilitas",
		"Data Terakhir Dicek",
		"30 Agu 2026 10:26 WIB",
		"1/1 • 100%",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q", want)
		}
	}

	leads := zipEntryText(t, zr, "xl/worksheets/sheet2.xml")
	if !strings.Contains(leads, `<col min="4" max="4" width="14.0" customWidth="1" hidden="1"/>`) {
		t.Fatal("expected empty Target column to be hidden")
	}
	if !strings.Contains(leads, `<col min="11" max="11" width="18.0" customWidth="1" hidden="1"/>`) {
		t.Fatal("expected empty Kisaran Harga column to be hidden")
	}
	if strings.Contains(leads, `<col min="12" max="12" width="34.0" customWidth="1" hidden="1"/>`) {
		t.Fatal("facilities column has data and must stay visible")
	}
	if !strings.Contains(leads, `autoFilter ref="A4:T5"`) {
		t.Fatal("expected customer lead autofilter")
	}
}

func TestCustomerPackageTitleFallsBackToDataScope(t *testing.T) {
	rows := []dashboardRow{{Lead: leadstore.Lead{Preset: "kost", Area: "jakarta", Subarea: "Jakarta Selatan"}}}
	if got := customerPackageTitle(rows, url.Values{}); got != "Database Kost Jakarta Selatan" {
		t.Fatalf("unexpected package title %q", got)
	}
}

func TestCustomerWorkbookFilenameUsesPackageTitle(t *testing.T) {
	rows := []dashboardRow{{Lead: leadstore.Lead{Preset: "kost", Subarea: "Jakarta Selatan"}}}
	filters := url.Values{"segment": {"putri"}}
	generatedAt := time.Date(2026, 8, 30, 5, 0, 0, 0, time.UTC)
	got := customerWorkbookFilename(rows, filters, generatedAt)
	want := "database-kost-putri-jakarta-selatan-20260830-120000.xlsx"
	if got != want {
		t.Fatalf("unexpected filename %q, want %q", got, want)
	}
}
