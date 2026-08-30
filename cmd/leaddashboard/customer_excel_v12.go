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
)

func (a *app) handleCustomerExportXLSXV12(w http.ResponseWriter, r *http.Request) {
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
	content, err := buildCustomerWorkbookV12(rows, r.URL.Query(), now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := "business-prospects-" + now.In(customerWIBLocation()).Format("20060102-150405") + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

func buildCustomerWorkbookV12(rows []dashboardRow, filters url.Values, generatedAt time.Time) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	summaryXML := buildB2BSummarySheetXMLV12(rows, filters, generatedAt)
	leadsXML, links := buildLeadsSheetXMLV12(rows, filters, generatedAt)

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
		}{"xl/worksheets/_rels/sheet2.xml.rels", buildHyperlinkRelsXML(links)})
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

func buildB2BSummarySheetXMLV12(rows []dashboardRow, filters url.Values, generatedAt time.Time) string {
	metrics := summarizeCustomerRows(rows)
	area := customerAreaScope(rows, filters)
	freshness := "Belum tersedia"
	if metrics.HasFreshness {
		freshness = formatCustomerDateTime(metrics.LatestChecked)
	}
	priority := "Semua prioritas"
	if value := strings.TrimSpace(filters.Get("target")); value != "" {
		priority = priorityLabel(value)
	}
	scale := "Semua skala"
	if value := strings.TrimSpace(filters.Get("segment")); value != "" {
		scale = segmentLabel(value)
	}

	rowsXML := []workbookRow{
		{Index: 1, Height: 32, Cells: []workbookCell{{Col: 1, Row: 1, Value: "DATABASE PROSPEK B2B", Style: 1}}},
		{Index: 2, Height: 22, Cells: []workbookCell{{Col: 1, Row: 2, Value: "Bisnis publik Jawa & Sumatra · database prospecting terkurasi", Style: 2}}},
		{Index: 4, Height: 22, Cells: []workbookCell{{Col: 1, Row: 4, Value: "RINGKASAN", Style: 3}, {Col: 5, Row: 4, Value: "COVERAGE & KUALITAS", Style: 3}}},
		{Index: 5, Cells: []workbookCell{{Col: 1, Row: 5, Value: "Wilayah", Style: 4}, {Col: 2, Row: 5, Value: area, Style: 5}, {Col: 5, Row: 5, Value: "Total Prospek", Style: 6}, {Col: 6, Row: 5, Value: strconv.Itoa(metrics.Total), Style: 7, Numeric: true}, {Col: 7, Row: 5, Value: "Rata-rata Rating", Style: 6}, {Col: 8, Row: 5, Value: fmt.Sprintf("%.1f", metrics.AvgRating), Style: 8, Numeric: true}}},
		{Index: 6, Cells: []workbookCell{{Col: 1, Row: 6, Value: "Skala Usaha", Style: 4}, {Col: 2, Row: 6, Value: scale, Style: 5}, {Col: 5, Row: 6, Value: "Coverage Telepon", Style: 6}, {Col: 6, Row: 6, Value: formatCoverage(metrics.WithPhone, metrics.Total), Style: 7}, {Col: 7, Row: 6, Value: "Coverage Skala", Style: 6}, {Col: 8, Row: 6, Value: formatCoverage(metrics.KnownSegment, metrics.Total), Style: 7}}},
		{Index: 7, Cells: []workbookCell{{Col: 1, Row: 7, Value: "Prioritas", Style: 4}, {Col: 2, Row: 7, Value: priority, Style: 5}, {Col: 5, Row: 7, Value: "Coverage Produk/Layanan", Style: 6}, {Col: 6, Row: 7, Value: formatCoverage(metrics.WithFacilities, metrics.Total), Style: 7}, {Col: 7, Row: 7, Value: "Terverifikasi", Style: 6}, {Col: 8, Row: 7, Value: formatCoverage(metrics.Verified, metrics.Total), Style: 7}}},
		{Index: 8, Cells: []workbookCell{{Col: 1, Row: 8, Value: "Data Terakhir Dicek", Style: 4}, {Col: 2, Row: 8, Value: freshness, Style: 5}, {Col: 5, Row: 8, Value: "Coverage Website", Style: 6}, {Col: 6, Row: 8, Value: formatCoverage(metrics.WithWebsite, metrics.Total), Style: 7}, {Col: 7, Row: 8, Value: "Coverage Operasional", Style: 6}, {Col: 8, Row: 8, Value: formatCoverage(metrics.WithPrice, metrics.Total), Style: 7}}},
		{Index: 10, Height: 22, Cells: []workbookCell{{Col: 1, Row: 10, Value: "DIBUAT", Style: 3}}},
		{Index: 11, Cells: []workbookCell{{Col: 1, Row: 11, Value: formatCustomerDateTime(generatedAt), Style: 5}}},
		{Index: 13, Height: 22, Cells: []workbookCell{{Col: 1, Row: 13, Value: "CATATAN PENGGUNAAN", Style: 3}}},
		{Index: 14, Height: 46, Cells: []workbookCell{{Col: 1, Row: 14, Value: "Data berasal dari listing bisnis publik dan enrichment internal. Informasi kontak, status usaha, serta detail layanan dapat berubah. Verifikasi kembali sebelum outreach.", Style: 9}}},
	}

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0" showGridLines="0"/></sheetViews>`)
	b.WriteString(`<cols><col min="1" max="1" width="22" customWidth="1"/><col min="2" max="4" width="19" customWidth="1"/><col min="5" max="5" width="22" customWidth="1"/><col min="6" max="6" width="21" customWidth="1"/><col min="7" max="7" width="22" customWidth="1"/><col min="8" max="8" width="21" customWidth="1"/></cols><sheetData>`)
	for _, row := range rowsXML {
		b.WriteString(writeWorkbookRow(row))
	}
	b.WriteString(`</sheetData><mergeCells count="6"><mergeCell ref="A1:H1"/><mergeCell ref="A2:H2"/><mergeCell ref="A4:D4"/><mergeCell ref="E4:H4"/><mergeCell ref="A13:H13"/><mergeCell ref="A14:H15"/></mergeCells><pageMargins left="0.3" right="0.3" top="0.5" bottom="0.5" header="0.2" footer="0.2"/></worksheet>`)
	return b.String()
}

func buildLeadsSheetXMLV12(rows []dashboardRow, filters url.Values, generatedAt time.Time) (string, []workbookHyperlink) {
	headers := []string{"No", "Nama Usaha", "Skala Usaha", "Prioritas Prospek", "Alamat", "Wilayah", "WhatsApp", "Website", "Rating", "Jumlah Review", "Skala Operasional", "Produk / Layanan", "Jenis Usaha Detail", "Status Kontak", "Catatan Internal", "Area / Cakupan Layanan", "Alasan Prioritas", "Status Verifikasi", "Google Maps"}
	links := make([]workbookHyperlink, 0, len(rows)*3)
	var sheetRows []workbookRow
	metrics := summarizeCustomerRows(rows)
	freshness := "freshness belum tersedia"
	if metrics.HasFreshness {
		freshness = "freshness " + formatCustomerDateTime(metrics.LatestChecked)
	}
	sheetRows = append(sheetRows,
		workbookRow{Index: 1, Height: 28, Cells: []workbookCell{{Col: 1, Row: 1, Value: "DATABASE PROSPEK B2B", Style: 1}}},
		workbookRow{Index: 2, Height: 20, Cells: []workbookCell{{Col: 1, Row: 2, Value: fmt.Sprintf("%d prospek · %s · dibuat %s", len(rows), freshness, formatCustomerDateTime(generatedAt)), Style: 10}}},
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
			{Col: 4, Row: excelRow, Value: priorityLabel(e.Target), Style: 12},
			{Col: 5, Row: excelRow, Value: strings.TrimSpace(row.Address), Style: 13},
			{Col: 6, Row: excelRow, Value: wilayah, Style: 12},
			{Col: 7, Row: excelRow, Value: linkLabel(waTarget, row.Phone, "Chat WA"), Style: linkStyle(waTarget), Hyperlink: waTarget},
			{Col: 8, Row: excelRow, Value: linkLabel(websiteTarget, row.Website, "Website"), Style: linkStyle(websiteTarget), Hyperlink: websiteTarget},
			{Col: 9, Row: excelRow, Value: fmt.Sprintf("%.1f", row.Rating), Style: 17, Numeric: true},
			{Col: 10, Row: excelRow, Value: strconv.Itoa(row.ReviewCount), Style: 12, Numeric: true},
			{Col: 11, Row: excelRow, Value: customerValue(e.PriceRange), Style: 13},
			{Col: 12, Row: excelRow, Value: customerValue(e.Facilities), Style: 13},
			{Col: 13, Row: excelRow, Value: customerValue(e.RentalType), Style: 12},
			{Col: 14, Row: excelRow, Value: contactStatusLabel(e.Furnish), Style: 12},
			{Col: 15, Row: excelRow, Value: customerValue(e.Rules), Style: 13},
			{Col: 16, Row: excelRow, Value: customerValue(e.Landmark), Style: 13},
			{Col: 17, Row: excelRow, Value: customerValue(e.SellingPoint), Style: 13},
			{Col: 18, Row: excelRow, Value: verificationText, Style: verificationStyle},
			{Col: 19, Row: excelRow, Value: linkLabel(mapsTarget, row.Link, "Buka Maps"), Style: linkStyle(mapsTarget), Hyperlink: mapsTarget},
		}
		for _, c := range cells {
			if c.Hyperlink != "" {
				links = append(links, workbookHyperlink{Ref: cellRef(c.Col, c.Row), Target: c.Hyperlink})
			}
		}
		sheetRows = append(sheetRows, workbookRow{Index: excelRow, Height: 32, Cells: cells})
	}

	lastRow := 4
	if len(rows) > 0 {
		lastRow = len(rows) + 4
	}
	hiddenCols := customerEmptyLeadColumnsV12(rows)
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0" showGridLines="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft" activeCell="A5" sqref="A5"/></sheetView></sheetViews><cols>`)
	widths := []float64{6, 28, 14, 16, 44, 24, 16, 15, 10, 13, 20, 32, 22, 18, 28, 25, 28, 18, 14}
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
	b.WriteString(`</sheetData><autoFilter ref="A4:S` + strconv.Itoa(lastRow) + `"/><mergeCells count="2"><mergeCell ref="A1:S1"/><mergeCell ref="A2:S2"/></mergeCells>`)
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

func customerEmptyLeadColumnsV12(rows []dashboardRow) map[int]bool {
	present := map[int]bool{}
	for _, row := range rows {
		e := row.Enrichment
		if customerValue(e.Segment) != "" { present[3] = true }
		if customerValue(e.Target) != "" { present[4] = true }
		if strings.TrimSpace(row.Phone) != "" { present[7] = true }
		if safeHTTPURL(row.Website) != "" || strings.TrimSpace(row.Website) != "" { present[8] = true }
		if customerValue(e.PriceRange) != "" { present[11] = true }
		if customerValue(e.Facilities) != "" { present[12] = true }
		if customerValue(e.RentalType) != "" { present[13] = true }
		if customerValue(e.Furnish) != "" { present[14] = true }
		if customerValue(e.Rules) != "" { present[15] = true }
		if customerValue(e.Landmark) != "" { present[16] = true }
		if customerValue(e.SellingPoint) != "" { present[17] = true }
		if safeHTTPURL(row.Link) != "" { present[19] = true }
	}
	optional := []int{3, 4, 7, 8, 11, 12, 13, 14, 15, 16, 17, 19}
	hidden := make(map[int]bool, len(optional))
	for _, col := range optional {
		if !present[col] { hidden[col] = true }
	}
	return hidden
}