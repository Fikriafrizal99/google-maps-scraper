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

	filename := customerWorkbookFilename(rows, r.URL.Query(), now)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

func buildCustomerWorkbookV12(rows []dashboardRow, filters url.Values, generatedAt time.Time) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	summaryXML := buildSummarySheetXMLV11(rows, filters, generatedAt)
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

func buildLeadsSheetXMLV12(rows []dashboardRow, filters url.Values, generatedAt time.Time) (string, []workbookHyperlink) {
	headers := []string{"No", "Nama Kost", "Segment", "Target", "Alamat", "Wilayah", "WhatsApp", "Website", "Rating", "Jumlah Review", "Kisaran Harga", "Fasilitas", "Tipe Sewa", "Furnish", "Aturan", "Landmark", "Selling Point", "Status Verifikasi", "Google Maps"}
	links := make([]workbookHyperlink, 0, len(rows)*3)
	var sheetRows []workbookRow
	metrics := summarizeCustomerRows(rows)
	freshness := "freshness belum tersedia"
	if metrics.HasFreshness {
		freshness = "freshness " + formatCustomerDateTime(metrics.LatestChecked)
	}
	sheetRows = append(sheetRows,
		workbookRow{Index: 1, Height: 28, Cells: []workbookCell{{Col: 1, Row: 1, Value: "DATABASE - " + customerPackageTitle(rows, filters), Style: 1}}},
		workbookRow{Index: 2, Height: 20, Cells: []workbookCell{{Col: 1, Row: 2, Value: fmt.Sprintf("%d lead - %s - dibuat %s - foto visual tersedia di PDF Catalog", len(rows), freshness, formatCustomerDateTime(generatedAt)), Style: 10}}},
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
	b.WriteString(`<sheetViews><sheetView workbookViewId="0" showGridLines="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft" activeCell="A5" sqref="A5"/></sheetView></sheetViews>`)
	b.WriteString(`<cols>`)
	widths := []float64{6, 28, 12, 14, 44, 18, 16, 15, 10, 13, 18, 32, 16, 16, 26, 22, 26, 18, 14}
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
	b.WriteString(`<autoFilter ref="A4:S` + strconv.Itoa(lastRow) + `"/>`)
	b.WriteString(`<mergeCells count="2"><mergeCell ref="A1:S1"/><mergeCell ref="A2:S2"/></mergeCells>`)
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
	}
	optional := []int{3, 4, 7, 8, 11, 12, 13, 14, 15, 16, 17, 19}
	hidden := make(map[int]bool, len(optional))
	for _, col := range optional {
		if !present[col] {
			hidden[col] = true
		}
	}
	return hidden
}
