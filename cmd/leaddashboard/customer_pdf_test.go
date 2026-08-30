package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func TestBuildCustomerPDFCreatesVisualCatalog(t *testing.T) {
	rows := []dashboardRow{{
		Lead: leadstore.Lead{
			Title:       "Kost Putri Tebet HK",
			Category:    "Boarding house",
			Address:     "Jl. Tebet Barat Dalam, Jakarta Selatan",
			Phone:       "0812-1186-8446",
			Website:     "https://example.com",
			Link:        "https://maps.google.com/example",
			Thumbnail:   "https://example.com/photo.jpg",
			Rating:      4.8,
			ReviewCount: 37,
			LastChecked: "2026-08-30T03:26:00Z",
		},
		Enrichment: leadstore.KostEnrichment{
			Segment:            "putri",
			Target:             "karyawan",
			RentalType:         "bulanan",
			PriceRange:         "Rp1,8 juta/bulan",
			Facilities:         "AC, WiFi, kamar mandi dalam",
			VerificationStatus: "verified",
		},
	}}
	filters := url.Values{
		"preset":              {"kost"},
		"subarea":             {"Jakarta Selatan"},
		"segment":             {"putri"},
		"verification_status": {"verified"},
	}
	fetcher := func(context.Context, string) (catalogImage, error) {
		img := image.NewRGBA(image.Rect(0, 0, 120, 80))
		for y := 0; y < 80; y++ {
			for x := 0; x < 120; x++ {
				img.Set(x, y, color.RGBA{R: 210, G: 225, B: 215, A: 255})
			}
		}
		var out bytes.Buffer
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 70}); err != nil {
			t.Fatalf("encode test jpeg: %v", err)
		}
		return catalogImage{JPEG: out.Bytes(), Width: 120, Height: 80}, nil
	}

	pdf, err := buildCustomerPDF(context.Background(), rows, filters, time.Date(2026, 8, 30, 12, 50, 0, 0, time.Local), fetcher)
	if err != nil {
		t.Fatalf("build customer pdf: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatal("expected PDF header")
	}
	text := string(pdf)
	for _, want := range []string{
		"Database Kost Putri Jakarta Selatan",
		"Kost Putri Tebet HK",
		"Chat WhatsApp",
		"Buka Google Maps",
		"/Subtype /Image",
		"/Subtype /Link",
		"/Count 2",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("pdf missing %q", want)
		}
	}
}

func TestCatalogImageLimitAdaptsToPackageSize(t *testing.T) {
	cases := []struct {
		total int
		want  int
	}{
		{1, 3},
		{60, 3},
		{61, 2},
		{150, 2},
		{151, 1},
		{280, 1},
	}
	for _, tc := range cases {
		if got := catalogImageLimit(tc.total); got != tc.want {
			t.Fatalf("catalogImageLimit(%d)=%d want %d", tc.total, got, tc.want)
		}
	}
}
