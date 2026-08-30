package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

func buildCatalogCoverPage(rows []dashboardRow, filters url.Values, generatedAt time.Time) catalogPDFPage {
	var p catalogPDFPage
	metrics := summarizeCustomerRows(rows)
	freshness := "Belum tersedia"
	if metrics.HasFreshness {
		freshness = formatCustomerDateTime(metrics.LatestChecked)
	}

	p.fillRect(0, 0, catalogPDFWidth, 205, pdfDark)
	p.text(42, 50, 11, false, pdfWhite, "CUSTOMER LEAD CATALOG")
	p.wrappedText(42, 78, 505, 26, 31, 2, true, pdfWhite, customerPackageTitle(rows, filters))
	p.text(42, 150, 11, false, pdfWhite, "Foto langsung di PDF - Excel tersedia untuk filter dan pengolahan data")

	cards := []struct{ label, value string }{
		{"Total Lead", strconv.Itoa(metrics.Total)},
		{"Coverage HP", formatCoverage(metrics.WithPhone, metrics.Total)},
		{"Terverifikasi", formatCoverage(metrics.Verified, metrics.Total)},
		{"Rata-rata Rating", fmt.Sprintf("%.1f", metrics.AvgRating)},
	}
	cardW := 121.5
	for i, card := range cards {
		x := 42 + float64(i)*(cardW+8)
		p.fillRect(x, 235, cardW, 72, pdfLight)
		p.text(x+12, 249, 9, false, pdfMuted, card.label)
		p.text(x+12, 271, 16, true, pdfDark, card.value)
	}

	p.text(42, 344, 11, true, pdfDark, "RINGKASAN PAKET")
	p.line(42, 365, 553, 365, pdfBorder, 0.8)
	p.infoPair(42, 382, 240, "Niche", customerNicheScope(rows, filters))
	p.infoPair(310, 382, 240, "Wilayah", customerAreaScope(rows, filters))
	p.infoPair(42, 428, 240, "Data Terakhir Dicek", freshness)
	p.infoPair(310, 428, 240, "Dibuat", formatCustomerDateTime(generatedAt))

	p.text(42, 493, 11, true, pdfDark, "FILTER")
	p.fillRect(42, 515, 511, 72, pdfLight)
	p.wrappedText(56, 530, 482, 10, 15, 3, false, pdfDark, describeCustomerFilters(filters))

	p.text(42, 625, 11, true, pdfDark, "CATATAN")
	note := "Data berasal dari listing bisnis publik dan enrichment internal. Nomor kontak, harga, fasilitas, kebijakan, serta ketersediaan dapat berubah. Verifikasi kembali sebelum outreach atau keputusan komersial."
	p.wrappedText(42, 648, 510, 10, 16, 5, false, pdfMuted, note)
	p.text(42, 786, 9, false, pdfMuted, "PDF = katalog visual | Excel = database yang dapat difilter")
	return p
}

func buildCatalogLeadPage(row dashboardRow, index, total int, packageTitle string, generatedAt time.Time, images []catalogImage, expectedImages int) catalogPDFPage {
	var p catalogPDFPage
	e := row.Enrichment

	p.fillRect(0, 0, catalogPDFWidth, 54, pdfDark)
	p.text(36, 17, 9, false, pdfWhite, packageTitle)
	p.textRight(559, 17, 9, false, pdfWhite, fmt.Sprintf("Lead %d / %d", index, total))

	titleHeight := p.wrappedText(36, 78, 420, 19, 23, 2, true, pdfDark, row.Title)
	badgeTop := 82 + titleHeight
	p.badge(36, badgeTop, segmentLabel(e.Segment), pdfLight, pdfDark)
	verifyFill := pdfLight
	if strings.EqualFold(e.VerificationStatus, "verified") {
		verifyFill = pdfGreenLight
	} else if strings.EqualFold(e.VerificationStatus, "needs_check") {
		verifyFill = pdfYellowLight
	}
	p.badge(122, badgeTop, verificationLabel(e.VerificationStatus), verifyFill, pdfDark)
	if row.Category != "" {
		p.text(36, badgeTop+30, 9, false, pdfMuted, row.Category)
	}

	photoCount := expectedImages
	if photoCount < 1 {
		photoCount = 1
	}
	if photoCount > 3 {
		photoCount = 3
	}
	photoTop, photoGap, photoH := 170.0, 8.0, 192.0
	photoW := (523 - float64(photoCount-1)*photoGap) / float64(photoCount)
	for i := 0; i < photoCount; i++ {
		x := 36 + float64(i)*(photoW+photoGap)
		if i < len(images) {
			p.imageCover(images[i], x, photoTop, photoW, photoH)
		} else {
			p.fillRect(x, photoTop, photoW, photoH, pdfLight)
			p.rectStroke(x, photoTop, photoW, photoH, pdfBorder, 0.7)
			p.textCenter(x+photoW/2, photoTop+88, 9, false, pdfMuted, "Foto tidak tersedia")
		}
	}

	top := 390.0
	p.infoPair(36, top, 247, "Harga", blankDash(customerValue(e.PriceRange)))
	p.infoPair(312, top, 247, "Tipe Sewa", blankDash(customerValue(e.RentalType)))
	p.infoPair(36, top+44, 247, "Rating", fmt.Sprintf("%.1f (%d review)", row.Rating, row.ReviewCount))
	p.infoPair(312, top+44, 247, "Target", blankDash(customerValue(e.Target)))
	p.text(36, top+99, 9, true, pdfMuted, "ALAMAT")
	p.wrappedText(36, top+115, 523, 10, 14, 3, false, pdfDark, blankDash(row.Address))
	p.text(36, top+166, 9, true, pdfMuted, "FASILITAS")
	p.wrappedText(36, top+182, 523, 10, 14, 2, false, pdfDark, blankDash(customerValue(e.Facilities)))
	p.text(36, top+222, 9, true, pdfMuted, "INFORMASI TAMBAHAN")
	p.wrappedText(36, top+238, 523, 9.5, 13, 3, false, pdfDark, catalogExtraText(e))

	buttons := []struct{ label, url string }{
		{"Chat WhatsApp", catalogWhatsAppURL(row.Phone)},
		{"Buka Website", safeHTTPURL(row.Website)},
		{"Buka Google Maps", safeHTTPURL(row.Link)},
	}
	for i, button := range buttons {
		x := 36 + float64(i)*170
		if button.url == "" {
			p.fillRect(x, 707, 160, 34, pdfLight)
			p.textCenter(x+80, 718, 9, false, pdfMuted, button.label+" -")
			continue
		}
		p.fillRect(x, 707, 160, 34, pdfGreen)
		p.textCenter(x+80, 718, 9, true, pdfWhite, button.label)
		p.links = append(p.links, catalogPDFLink{X: x, Top: 707, W: 160, H: 34, URL: button.url})
	}

	freshness := "Data terakhir dicek: -"
	if checked, ok := parseCustomerLeadTime(row.LastChecked); ok {
		freshness = "Data terakhir dicek: " + formatCustomerDateTime(checked)
	}
	p.text(36, 776, 8.5, false, pdfMuted, freshness)
	p.textRight(559, 776, 8.5, false, pdfMuted, "Dibuat "+formatCustomerDateTime(generatedAt))
	p.line(36, 801, 559, 801, pdfBorder, 0.6)
	p.text(36, 811, 7.8, false, pdfMuted, "Listing dapat berubah. Verifikasi kembali sebelum menghubungi pemilik/pengelola.")
	return p
}

func catalogExtraText(e leadstore.KostEnrichment) string {
	parts := make([]string, 0, 4)
	for _, item := range []struct{ label, value string }{
		{"Furnish", e.Furnish}, {"Aturan", e.Rules}, {"Landmark", e.Landmark}, {"Selling point", e.SellingPoint},
	} {
		if value := customerValue(item.value); value != "" {
			parts = append(parts, item.label+": "+value)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " | ")
}

func blankDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
