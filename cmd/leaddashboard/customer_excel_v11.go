package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type customerWorkbookMetrics struct {
	Total          int
	WithPhone      int
	WithWebsite    int
	KnownSegment   int
	WithFacilities int
	WithPrice      int
	Verified       int
	AvgRating      float64
	LatestChecked  time.Time
	HasFreshness   bool
}

func (a *app) handleCustomerExportXLSXV11(w http.ResponseWriter, r *http.Request) {
	leads, err := a.store.List(r.Context(), a.filterFromRequest(r, 5000))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, err := a.dashboardRows(
		r.Context(), leads,
		r.URL.Query().Get("review_status"),
		r.URL.Query().Get("segment"),
		r.URL.Query().Get("target"),
		r.URL.Query().Get("verification_status"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows = customerSafeRows(rows)

	now := time.Now()
	content, err := buildCustomerWorkbookV11(rows, r.URL.Query(), now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := customerWorkbookFilename(rows, r.URL.Query(), now)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

func buildCustomerWorkbookV11(rows []dashboardRow, filters url.Values, generatedAt time.Time) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	summaryXML := buildSummarySheetXMLV11(rows, filters, generatedAt)
	leadsXML, links := buildLeadsSheetXMLV11(rows, filters, generatedAt)

	files := []struct {
		Name string
		Body string
	}{
		{"[Content_Types].xml", workbookContentTypesXML},
		{"_rels/.rels", workbookRootRelsXML},
		{"xl/workbook.xml", workbookXML},
		{"xl/_rels/workbook.xml.rels", workbookRelsXML},
		{"xl/styles.xml", workbookStylesXML},
		{"xl/worksheets/sheet1.xml", summaryXML},
		{"xl/worksheets/sheet2.xml", leadsXML},
	}
	if len(links) > 0 {
		files = append(files, struct {
			Name string
			Body string
		}{
			"xl/worksheets/_rels/sheet2.xml.rels", buildHyperlinkRelsXML(links),
		})
	}

	for _, file := range files {
		writer, err := zw.Create(file.Name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("create xlsx part %s: %w", file.Name, err)
		}
		if _, err := writer.Write([]byte(file.Body)); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write xlsx part %s: %w", file.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize xlsx: %w", err)
	}
	return out.Bytes(), nil
}

func buildSummarySheetXMLV11(rows []dashboardRow, filters url.Values, generatedAt time.Time) string {
	metrics := summarizeCustomerRows(rows)
	filterText := describeCustomerFilters(filters)
	niche := customerNicheScope(rows, filters)
	area := customerAreaScope(rows, filters)
	freshness := "Belum tersedia"
	if metrics.HasFreshness {
		freshness = formatCustomerDateTime(metrics.LatestChecked)
	}
	packageTitle := customerPackageTitle(rows, filters)

	rowsXML := []workbookRow{
		{Index: 1, Height: 32, Cells: []workbookCell{{Col: 1, Row: 1, Value: packageTitle, Style: 1}}},
		{Index: 2, Height: 22, Cells: []workbookCell{{Col: 1, Row: 2, Value: "Customer Delivery • Database lead terkurasi siap digunakan", Style: 2}}},
		{Index: 4, Height: 22, Cells: []workbookCell{{Col: 1, Row: 4, Value: "RINGKASAN PAKET", Style: 3}, {Col: 5, Row: 4, Value: "COVERAGE & KUALITAS", Style: 3}}},
		{Index: 5, Cells: []workbookCell{{Col: 1, Row: 5, Value: "Niche", Style: 4}, {Col: 2, Row: 5, Value: niche, Style: 5}, {Col: 5, Row: 5, Value: "Total Lead", Style: 6}, {Col: 6, Row: 5, Value: strconv.Itoa(metrics.Total), Style: 7, Numeric: true}, {Col: 7, Row: 5, Value: "Rata-rata Rating", Style: 6}, {Col: 8, Row: 5, Value: fmt.Sprintf("%.1f", metrics.AvgRating), Style: 8, Numeric: true}}},
		{Index: 6, Cells: []workbookCell{{Col: 1, Row: 6, Value: "Wilayah", Style: 4}, {Col: 2, Row: 6, Value: area, Style: 5}, {Col: 5, Row: 6, Value: "Coverage HP", Style: 6}, {Col: 6, Row: 6, Value: formatCoverage(metrics.WithPhone, metrics.Total), Style: 7}, {Col: 7, Row: 6, Value: "Coverage Segment", Style: 6}, {Col: 8, Row: 6, Value: formatCoverage(metrics.KnownSegment, metrics.Total), Style: 7}}},
		{Index: 7, Cells: []workbookCell{{Col: 1, Row: 7, Value: "Data Terakhir Dicek", Style: 4}, {Col: 2, Row: 7, Value: freshness, Style: 5}, {Col: 5, Row: 7, Value: "Coverage Fasilitas", Style: 6}, {Col: 6, Row: 7, Value: formatCoverage(metrics.WithFacilities, metrics.Total), Style: 7}, {Col: 7, Row: 7, Value: "Terverifikasi", Style: 6}, {Col: 8, Row: 7, Value: formatCoverage(metrics.Verified, metrics.Total), Style: 7}}},
		{Index: 8, Cells: []workbookCell{{Col: 1, Row: 8, Value: "Dibuat", Style: 4}, {Col: 2, Row: 8, Value: formatCustomerDateTime(generatedAt), Style: 5}, {Col: 5, Row: 8, Value: "Coverage Website", Style: 6}, {Col: 6, Row: 8, Value: formatCoverage(metrics.WithWebsite, metrics.Total), Style: 7}, {Col: 7, Row: 8, Value: "Coverage Harga", Style: 6}, {Col: 8, Row: 8, Value: formatCoverage(metrics.WithPrice, metrics.Total), Style: 7}}},
		{Index: 10, Height: 22, Cells: []workbookCell{{Col: 1, Row: 10, Value: "FILTER YANG DIGUNAKAN", Style: 3}}},
		{Index: 11, Height: 34, Cells: []workbookCell{{Col: 1, Row: 11, Value: filterText, Style: 9}}},
		{Index: 13, Height: 22, Cells: []workbookCell{{Col: 1, Row: 13, Value: "CATATAN PENGGUNAAN", Style: 3}}},
		{Index: 14, Height: 46, Cells: []workbookCell{{Col: 1, Row: 14, Value: "Data disusun dari listing bisnis publik dan hasil enrichment internal. Nomor kontak, harga, fasilitas, kebijakan, dan ketersediaan dapat berubah. Lakukan verifikasi kembali sebelum keputusan atau outreach komersial.", Style: 9}}},
		{Index: 18, Cells: []workbookCell{{Col: 1, Row: 18, Value: "Sheet LEADS berisi data customer-safe yang lolos filter. Kolom yang 100% kosong disembunyikan otomatis agar file lebih ringkas.", Style: 10}}},
	}

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0" showGridLines="0"/></sheetViews>`)
	b.WriteString(`<cols><col min="1" max="1" width="22" customWidth="1"/><col min="2" max="4" width="19" customWidth="1"/><col min="5" max="5" width="20" customWidth="1"/><col min="6" max="6" width="21" customWidth="1"/><col min="7" max="7" width="20" customWidth="1"/><col min="8" max="8" width="21" customWidth="1"/></cols>`)
	b.WriteString(`<sheetData>`)
	for _, row := range rowsXML {
		b.WriteString(writeWorkbookRow(row))
	}
	b.WriteString(`</sheetData>`)
	b.WriteString(`<mergeCells count="9"><mergeCell ref="A1:H1"/><mergeCell ref="A2:H2"/><mergeCell ref="A4:D4"/><mergeCell ref="E4:H4"/><mergeCell ref="A10:H10"/><mergeCell ref="A11:H11"/><mergeCell ref="A13:H13"/><mergeCell ref="A14:H15"/><mergeCell ref="A18:H18"/></mergeCells>`)
	b.WriteString(`<pageMargins left="0.3" right="0.3" top="0.5" bottom="0.5" header="0.2" footer="0.2"/></worksheet>`)
	return b.String()
}

func buildLeadsSheetXMLV11(rows []dashboardRow, filters url.Values, generatedAt time.Time) (string, []workbookHyperlink) {
	headers := []string{"No", "Nama Kost", "Segment", "Target", "Alamat", "Wilayah", "WhatsApp", "Website", "Rating", "Jumlah Review", "Kisaran Harga", "Fasilitas", "Tipe Sewa", "Furnish", "Aturan", "Landmark", "Selling Point", "Status Verifikasi", "Google Maps", "Foto"}
	links := make([]workbookHyperlink, 0, len(rows)*4)
	var sheetRows []workbookRow
	metrics := summarizeCustomerRows(rows)
	freshness := "freshness belum tersedia"
	if metrics.HasFreshness {
		freshness = "freshness " + formatCustomerDateTime(metrics.LatestChecked)
	}
	sheetRows = append(sheetRows,
		workbookRow{Index: 1, Height: 28, Cells: []workbookCell{{Col: 1, Row: 1, Value: "LEADS • " + customerPackageTitle(rows, filters), Style: 1}}},
		workbookRow{Index: 2, Height: 20, Cells: []workbookCell{{Col: 1, Row: 2, Value: fmt.Sprintf("%d lead • %s • Dibuat %s", len(rows), freshness, formatCustomerDateTime(generatedAt)), Style: 10}}},
	)
	headerCells := make([]workbookCell, len(headers))
	for i, header := range headers {
		headerCells[i] = workbookCell{Col: i + 1, Row: 4, Value: header, Style: 11}
	}
	sheetRows = append(sheetRows, workbookRow{Index: 4, Height: 28, Cells: headerCells})

	for i, row := range rows {
		excelRow := i + 5
		e := row.Enrichment
		wilayah := strings.TrimSpace(row.Subarea)
		if wilayah == "" {
			wilayah = strings.TrimSpace(row.Area)
		}
		waTarget := ""
		if phone := waNumber(row.Phone); phone != "" {
			waTarget = "https://wa.me/" + phone
		}
		websiteTarget := safeHTTPURL(row.Website)
		mapsTarget := safeHTTPURL(row.Link)
		photoTarget := ""
		if images := leadImages(row.Images, row.Thumbnail, 1); len(images) > 0 {
			photoTarget = safeHTTPURL(images[0])
		}
		verificationStyle := 14
		verificationText := verificationLabel(e.VerificationStatus)
		switch e.VerificationStatus {
		case "verified":
			verificationStyle = 15
		case "needs_check":
			verificationStyle = 16
		}
		cells := []workbookCell{
			{Col: 1, Row: excelRow, Value: strconv.Itoa(i + 1), Style: 12, Numeric: true},
			{Col: 2, Row: excelRow, Value: strings.TrimSpace(row.Title), Style: 13},
			{Col: 3, Row: excelRow, Value: segmentLabel(e.Segment), Style: 12},
			{Col: 4, Row: excelRow, Value: customerValue(e.Target), Style: 12},
			{Col: 5, Row: excelRow, Value: strings.TrimSpace(row.Address), Style: 13},
			{Col: 6, Row: excelRow, Value: wilayah, Style: 12},
			{Col: 7, Row: excelRow, Value: linkLabel(waTarget, row.Phone, "Chat WA"), Style: linkStyle(waTarget), Hyperlink: waTarget},
			{Col: 8, Row: excelRow, Value: linkLabel(websiteTarget, row.Website, "Website"), Style: linkStyle(websiteTarget), Hyperlink: websiteTarget},
			{Col: 9, Row: excelRow, Value: fmt.Sprintf("%.1f", row.Rating), Style: 17, Numeric: true},
			{Col: 10, Row: excelRow, Value: strconv.Itoa(row.ReviewCount), Style: 12, Numeric: true},
			{Col: 11, Row: excelRow, Value: customerValue(e.PriceRange), Style: 13},
			{Col: 12, Row: excelRow, Value: customerValue(e.Facilities), Style: 13},
			{Col: 13, Row: excelRow, Value: customerValue(e.RentalType), Style: 12},
			{Col: 14, Row: excelRow, Value: customerValue(e.Furnish), Style: 12},
			{Col: 15, Row: excelRow, Value: customerValue(e.Rules), Style: 13},
			{Col: 16, Row: excelRow, Value: customerValue(e.Landmark), Style: 13},
			{Col: 17, Row: excelRow, Value: customerValue(e.SellingPoint), Style: 13},
			{Col: 18, Row: excelRow, Value: verificationText, Style: verificationStyle},
			{Col: 19, Row: excelRow, Value: linkLabel(mapsTarget, row.Link, "Buka Maps"), Style: linkStyle(mapsTarget), Hyperlink: mapsTarget},
			{Col: 20, Row: excelRow, Value: linkLabel(photoTarget, "", "Lihat Foto"), Style: linkStyle(photoTarget), Hyperlink: photoTarget},
		}
		for _, c := range cells {
			if c.Hyperlink != "" {
				links = append(links, workbookHyperlink{Ref: cellRef(c.Col, c.Row), Target: c.Hyperlink})
			}
		}
		sheetRows = append(sheetRows, workbookRow{Index: excelRow, Height: 34, Cells: cells})
	}

	lastRow := 4
	if len(rows) > 0 {
		lastRow = len(rows) + 4
	}
	hiddenCols := customerEmptyLeadColumns(rows)
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0" showGridLines="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft" activeCell="A5" sqref="A5"/></sheetView></sheetViews>`)
	b.WriteString(`<cols>`)
	widths := []float64{6, 28, 12, 14, 46, 18, 16, 15, 10, 13, 18, 34, 16, 16, 28, 24, 28, 18, 14, 14}
	for i, width := range widths {
		hidden := ""
		if hiddenCols[i+1] {
			hidden = ` hidden="1"`
		}
		b.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%.1f" customWidth="1"%s/>`, i+1, i+1, width, hidden))
	}
	b.WriteString(`</cols><sheetData>`)
	for _, row := range sheetRows {
		b.WriteString(writeWorkbookRow(row))
	}
	b.WriteString(`</sheetData>`)
	b.WriteString(`<autoFilter ref="A4:T` + strconv.Itoa(lastRow) + `"/>`)
	b.WriteString(`<mergeCells count="2"><mergeCell ref="A1:T1"/><mergeCell ref="A2:T2"/></mergeCells>`)
	if len(links) > 0 {
		b.WriteString(`<hyperlinks>`)
		for i, link := range links {
			b.WriteString(fmt.Sprintf(`<hyperlink ref="%s" r:id="rId%d"/>`, link.Ref, i+1))
		}
		b.WriteString(`</hyperlinks>`)
	}
	b.WriteString(`<pageMargins left="0.25" right="0.25" top="0.5" bottom="0.5" header="0.2" footer="0.2"/></worksheet>`)
	return b.String(), links
}

func summarizeCustomerRows(rows []dashboardRow) customerWorkbookMetrics {
	metrics := customerWorkbookMetrics{Total: len(rows)}
	ratingTotal := 0.0
	ratingCount := 0
	for _, row := range rows {
		if strings.TrimSpace(row.Phone) != "" {
			metrics.WithPhone++
		}
		if strings.TrimSpace(row.Website) != "" {
			metrics.WithWebsite++
		}
		if customerValue(row.Enrichment.Segment) != "" {
			metrics.KnownSegment++
		}
		if customerValue(row.Enrichment.Facilities) != "" {
			metrics.WithFacilities++
		}
		if customerValue(row.Enrichment.PriceRange) != "" {
			metrics.WithPrice++
		}
		if strings.EqualFold(strings.TrimSpace(row.Enrichment.VerificationStatus), "verified") {
			metrics.Verified++
		}
		if row.Rating > 0 {
			ratingTotal += row.Rating
			ratingCount++
		}
		if checked, ok := parseCustomerLeadTime(row.LastChecked); ok && (!metrics.HasFreshness || checked.After(metrics.LatestChecked)) {
			metrics.LatestChecked = checked
			metrics.HasFreshness = true
		}
	}
	if ratingCount > 0 {
		metrics.AvgRating = ratingTotal / float64(ratingCount)
	}
	return metrics
}

func customerEmptyLeadColumns(rows []dashboardRow) map[int]bool {
	present := map[int]bool{}
	for _, row := range rows {
		e := row.Enrichment
		if customerValue(e.Segment) != "" {
			present[3] = true
		}
		if customerValue(e.Target) != "" {
			present[4] = true
		}
		if strings.TrimSpace(row.Phone) != "" {
			present[7] = true
		}
		if safeHTTPURL(row.Website) != "" || strings.TrimSpace(row.Website) != "" {
			present[8] = true
		}
		if customerValue(e.PriceRange) != "" {
			present[11] = true
		}
		if customerValue(e.Facilities) != "" {
			present[12] = true
		}
		if customerValue(e.RentalType) != "" {
			present[13] = true
		}
		if customerValue(e.Furnish) != "" {
			present[14] = true
		}
		if customerValue(e.Rules) != "" {
			present[15] = true
		}
		if customerValue(e.Landmark) != "" {
			present[16] = true
		}
		if customerValue(e.SellingPoint) != "" {
			present[17] = true
		}
		if safeHTTPURL(row.Link) != "" {
			present[19] = true
		}
		if images := leadImages(row.Images, row.Thumbnail, 1); len(images) > 0 && safeHTTPURL(images[0]) != "" {
			present[20] = true
		}
	}
	optional := []int{3, 4, 7, 8, 11, 12, 13, 14, 15, 16, 17, 19, 20}
	hidden := make(map[int]bool, len(optional))
	for _, col := range optional {
		if !present[col] {
			hidden[col] = true
		}
	}
	return hidden
}

func customerPackageTitle(rows []dashboardRow, filters url.Values) string {
	parts := []string{"Database"}
	niche := customerNicheScope(rows, filters)
	if niche == "Semua niche" {
		parts = append(parts, "Leads")
	} else {
		parts = append(parts, niche)
	}
	if segment := strings.TrimSpace(filters.Get("segment")); segment != "" && !strings.EqualFold(segment, "unknown") {
		parts = append(parts, segmentLabel(segment))
	}
	if target := strings.TrimSpace(filters.Get("target")); target != "" && !strings.EqualFold(target, "unknown") {
		parts = append(parts, titleWords(target))
	}
	if area := customerAreaScope(rows, filters); area != "Semua area" {
		parts = append(parts, area)
	}
	return strings.Join(parts, " ")
}

func customerNicheScope(rows []dashboardRow, filters url.Values) string {
	value := strings.TrimSpace(filters.Get("preset"))
	if value == "" {
		value = commonDashboardRowValue(rows, func(row dashboardRow) string { return row.Preset })
	}
	if value == "" {
		return "Semua niche"
	}
	return titleWords(value)
}

func customerAreaScope(rows []dashboardRow, filters url.Values) string {
	value := strings.TrimSpace(filters.Get("subarea"))
	if value == "" {
		value = strings.TrimSpace(filters.Get("area"))
	}
	if value == "" {
		value = commonDashboardRowValue(rows, func(row dashboardRow) string { return row.Subarea })
	}
	if value == "" {
		value = commonDashboardRowValue(rows, func(row dashboardRow) string { return row.Area })
	}
	if value == "" {
		return "Semua area"
	}
	return titleWords(value)
}

func commonDashboardRowValue(rows []dashboardRow, get func(dashboardRow) string) string {
	common := ""
	for _, row := range rows {
		value := strings.TrimSpace(get(row))
		if value == "" {
			continue
		}
		if common == "" {
			common = value
			continue
		}
		if !strings.EqualFold(common, value) {
			return ""
		}
	}
	return common
}

func titleWords(value string) string {
	value = strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(value))
	words := strings.Fields(value)
	for i, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func formatCoverage(count, total int) string {
	percent := 0
	if total > 0 {
		percent = int(float64(count)/float64(total)*100 + 0.5)
	}
	return fmt.Sprintf("%d/%d • %d%%", count, total, percent)
}

func parseCustomerLeadTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, customerWIBLocation()); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func formatCustomerDateTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	value = value.In(customerWIBLocation())
	months := [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	return fmt.Sprintf("%02d %s %04d %02d:%02d WIB", value.Day(), months[value.Month()-1], value.Year(), value.Hour(), value.Minute())
}

func customerWIBLocation() *time.Location {
	return time.FixedZone("WIB", 7*60*60)
}

func customerWorkbookFilename(rows []dashboardRow, filters url.Values, generatedAt time.Time) string {
	slug := strings.ToLower(customerPackageTitle(rows, filters))
	var b strings.Builder
	lastDash := false
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "customer-leads"
	}
	return fmt.Sprintf("%s-%s.xlsx", name, generatedAt.In(customerWIBLocation()).Format("20060102-150405"))
}
