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

func TestBuildCustomerWorkbookV12UsesDataFirstLeadsSheet(t *testing.T) {
	rows := []dashboardRow{{
		Lead: leadstore.Lead{
			Title:       "Kost Putri Tebet HK",
			Address:     "Jl. Tebet Barat Dalam",
			Area:        "jakarta",
			Subarea:     "Jakarta Selatan",
			Phone:       "0812-1186-8446",
			Website:     "https://example.com",
			Rating:      4.8,
			ReviewCount: 37,
			Link:        "https://maps.google.com/example",
		},
		Enrichment: leadstore.KostEnrichment{
			Segment:            "putri",
			Facilities:         "AC, WiFi, kamar mandi dalam",
			VerificationStatus: "verified",
		},
	}}
	filters := url.Values{"segment": {"putri"}}
	content, err := buildCustomerWorkbookV12(rows, filters, time.Date(2026, 8, 30, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("build workbook: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	var leads string
	for _, f := range zr.File {
		if f.Name != "xl/worksheets/sheet2.xml" {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		leads = string(data)
	}
	for _, want := range []string{"DATABASE - ", "Kost Putri Tebet HK", "Chat WA", "Buka Maps", `autoFilter ref="A4:S5"`, `mergeCell ref="A1:S1"`} {
		if !strings.Contains(leads, want) {
			t.Fatalf("leads sheet missing %q", want)
		}
	}
	if strings.Contains(leads, "Lihat Foto") || strings.Contains(leads, `ref="T`) {
		t.Fatalf("v1.2 should not expose photo-link column: %s", leads)
	}
}
