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

func TestBuildCustomerWorkbookV12UsesB2BProspectSheet(t *testing.T) {
	rows := []dashboardRow{{
		Lead: leadstore.Lead{
			Title:       "Jaya Motor Cianjur",
			Address:     "Jl. Raya Cugenang",
			Area:        "java-sumatra",
			Subarea:     "CUGENANG, KABUPATEN CIANJUR, JAWA BARAT, Indonesia",
			Phone:       "0812-1186-8446",
			Website:     "https://example.com",
			Rating:      4.8,
			ReviewCount: 37,
			Link:        "https://maps.google.com/example",
		},
		Enrichment: leadstore.KostEnrichment{
			Segment:            "kecil",
			Target:             "high",
			RentalType:         "bengkel motor",
			Facilities:         "servis motor, sparepart",
			Furnish:            "follow_up",
			VerificationStatus: "verified",
		},
	}}
	filters := url.Values{"segment": {"kecil"}, "target": {"high"}}
	content, err := buildCustomerWorkbookV12(rows, filters, time.Date(2026, 8, 30, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("build workbook: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	var summary, leads string
	for _, f := range zr.File {
		if f.Name != "xl/worksheets/sheet1.xml" && f.Name != "xl/worksheets/sheet2.xml" {
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
		if f.Name == "xl/worksheets/sheet1.xml" {
			summary = string(data)
		} else {
			leads = string(data)
		}
	}
	for _, want := range []string{"DATABASE PROSPEK B2B", "Total Prospek", "Coverage Telepon"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary sheet missing %q", want)
		}
	}
	for _, want := range []string{"DATABASE PROSPEK B2B", "Jaya Motor Cianjur", "Skala Usaha", "Prioritas Prospek", "Status Kontak", "Chat WA", "Buka Maps", `autoFilter ref="A4:S5"`, `mergeCell ref="A1:S1"`} {
		if !strings.Contains(leads, want) {
			t.Fatalf("leads sheet missing %q", want)
		}
	}
	if strings.Contains(leads, "Nama Kost") || strings.Contains(leads, "Kisaran Harga") {
		t.Fatalf("v1.2 B2B workbook must not expose kost-specific headers: %s", leads)
	}
}