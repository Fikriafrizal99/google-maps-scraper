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

type workbookHyperlink struct {
	Ref    string
	Target string
}

type workbookCell struct {
	Col       int
	Row       int
	Value     string
	Style     int
	Numeric   bool
	Hyperlink string
}

type workbookRow struct {
	Index  int
	Height float64
	Cells  []workbookCell
}

func (a *app) handleCustomerExportXLSX(w http.ResponseWriter, r *http.Request) {
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

	content, err := buildCustomerWorkbook(rows, r.URL.Query(), time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := "customer-leads-" + time.Now().Format("20060102-150405") + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

func buildCustomerWorkbook(rows []dashboardRow, filters url.Values, generatedAt time.Time) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	summaryXML := buildSummarySheetXML(rows, filters, generatedAt)
	leadsXML, links := buildLeadsSheetXML(rows, generatedAt)

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

func buildSummarySheetXML(rows []dashboardRow, filters url.Values, generatedAt time.Time) string {
	total := len(rows)
	withPhone := 0
	withWebsite := 0
	verified := 0
	ratingTotal := 0.0
	ratingCount := 0
	for _, row := range rows {
		if strings.TrimSpace(row.Phone) != "" {
			withPhone++
		}
		if strings.TrimSpace(row.Website) != "" {
			withWebsite++
		}
		if row.Enrichment.VerificationStatus == "verified" {
			verified++
		}
		if row.Rating > 0 {
			ratingTotal += row.Rating
			ratingCount++
		}
	}
	avgRating := 0.0
	if ratingCount > 0 {
		avgRating = ratingTotal / float64(ratingCount)
	}

	filterText := describeCustomerFilters(filters)
	niche := fallbackValue(filters.Get("preset"), "Semua niche")
	area := fallbackValue(filters.Get("subarea"), filters.Get("area"))
	if area == "" {
		area = "Semua area"
	}

	rowsXML := []workbookRow{
		{Index: 1, Height: 30, Cells: []workbookCell{{Col: 1, Row: 1, Value: "DATABASE LEADS KOST", Style: 1}}},
		{Index: 2, Height: 22, Cells: []workbookCell{{Col: 1, Row: 2, Value: "Customer Delivery • Data terfilter siap digunakan", Style: 2}}},
		{Index: 4, Height: 22, Cells: []workbookCell{{Col: 1, Row: 4, Value: "RINGKASAN PAKET", Style: 3}, {Col: 5, Row: 4, Value: "KPI DATA", Style: 3}}},
		{Index: 5, Cells: []workbookCell{{Col: 1, Row: 5, Value: "Niche", Style: 4}, {Col: 2, Row: 5, Value: niche, Style: 5}, {Col: 5, Row: 5, Value: "Total Lead", Style: 6}, {Col: 6, Row: 5, Value: strconv.Itoa(total), Style: 7, Numeric: true}, {Col: 7, Row: 5, Value: "Dengan HP", Style: 6}, {Col: 8, Row: 5, Value: strconv.Itoa(withPhone), Style: 7, Numeric: true}}},
		{Index: 6, Cells: []workbookCell{{Col: 1, Row: 6, Value: "Wilayah", Style: 4}, {Col: 2, Row: 6, Value: area, Style: 5}, {Col: 5, Row: 6, Value: "Terverifikasi", Style: 6}, {Col: 6, Row: 6, Value: strconv.Itoa(verified), Style: 7, Numeric: true}, {Col: 7, Row: 6, Value: "Rata-rata Rating", Style: 6}, {Col: 8, Row: 6, Value: fmt.Sprintf("%.1f", avgRating), Style: 8, Numeric: true}}},
		{Index: 7, Cells: []workbookCell{{Col: 1, Row: 7, Value: "Dibuat", Style: 4}, {Col: 2, Row: 7, Value: generatedAt.Format("02 Jan 2006 15:04"), Style: 5}, {Col: 5, Row: 7, Value: "Dengan Website", Style: 6}, {Col: 6, Row: 7, Value: strconv.Itoa(withWebsite), Style: 7, Numeric: true}}},
		{Index: 9, Height: 22, Cells: []workbookCell{{Col: 1, Row: 9, Value: "FILTER YANG DIGUNAKAN", Style: 3}}},
		{Index: 10, Height: 34, Cells: []workbookCell{{Col: 1, Row: 10, Value: filterText, Style: 9}}},
		{Index: 12, Height: 22, Cells: []workbookCell{{Col: 1, Row: 12, Value: "CATATAN PENGGUNAAN", Style: 3}}},
		{Index: 13, Height: 46, Cells: []workbookCell{{Col: 1, Row: 13, Value: "Data disusun dari listing bisnis publik dan hasil enrichment internal. Nomor kontak, harga, fasilitas, kebijakan, dan ketersediaan dapat berubah. Lakukan verifikasi kembali sebelum keputusan atau outreach komersial.", Style: 9}}},
		{Index: 16, Cells: []workbookCell{{Col: 1, Row: 16, Value: "Sheet LEADS berisi seluruh data yang lolos filter paket ini.", Style: 10}}},
	}

	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0" showGridLines="0"/></sheetViews>`)
	b.WriteString(`<cols><col min="1" max="1" width="22" customWidth="1"/><col min="2" max="4" width="18" customWidth="1"/><col min="5" max="5" width="20" customWidth="1"/><col min="6" max="6" width="14" customWidth="1"/><col min="7" max="7" width="20" customWidth="1"/><col min="8" max="8" width="14" customWidth="1"/></cols>`)
	b.WriteString(`<sheetData>`)
	for _, row := range rowsXML {
		b.WriteString(writeWorkbookRow(row))
	}
	b.WriteString(`</sheetData>`)
	b.WriteString(`<mergeCells count="8"><mergeCell ref="A1:H1"/><mergeCell ref="A2:H2"/><mergeCell ref="A4:D4"/><mergeCell ref="E4:H4"/><mergeCell ref="A9:H9"/><mergeCell ref="A10:H10"/><mergeCell ref="A12:H12"/><mergeCell ref="A13:H14"/></mergeCells>`)
	b.WriteString(`<pageMargins left="0.3" right="0.3" top="0.5" bottom="0.5" header="0.2" footer="0.2"/></worksheet>`)
	return b.String()
}

func buildLeadsSheetXML(rows []dashboardRow, generatedAt time.Time) (string, []workbookHyperlink) {
	headers := []string{"No", "Nama Kost", "Segment", "Target", "Alamat", "Wilayah", "WhatsApp", "Website", "Rating", "Jumlah Review", "Kisaran Harga", "Fasilitas", "Tipe Sewa", "Furnish", "Aturan", "Landmark", "Selling Point", "Status Verifikasi", "Google Maps", "Foto"}
	links := make([]workbookHyperlink, 0, len(rows)*4)
	var sheetRows []workbookRow
	sheetRows = append(sheetRows,
		workbookRow{Index: 1, Height: 28, Cells: []workbookCell{{Col: 1, Row: 1, Value: "LEADS CUSTOMER", Style: 1}}},
		workbookRow{Index: 2, Height: 20, Cells: []workbookCell{{Col: 1, Row: 2, Value: fmt.Sprintf("%d lead • Dibuat %s", len(rows), generatedAt.Format("02 Jan 2006 15:04")), Style: 10}}},
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
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0" showGridLines="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft" activeCell="A5" sqref="A5"/></sheetView></sheetViews>`)
	b.WriteString(`<cols>`)
	widths := []float64{6, 28, 12, 14, 46, 18, 16, 15, 10, 13, 18, 34, 16, 16, 28, 24, 28, 18, 14, 14}
	for i, width := range widths {
		b.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, width))
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

func writeWorkbookRow(row workbookRow) string {
	var b strings.Builder
	b.WriteString(`<row r="` + strconv.Itoa(row.Index) + `"`)
	if row.Height > 0 {
		b.WriteString(fmt.Sprintf(` ht="%.1f" customHeight="1"`, row.Height))
	}
	b.WriteString(`>`)
	for _, cell := range row.Cells {
		b.WriteString(writeWorkbookCell(cell))
	}
	b.WriteString(`</row>`)
	return b.String()
}

func writeWorkbookCell(cell workbookCell) string {
	ref := cellRef(cell.Col, cell.Row)
	if cell.Numeric {
		value := strings.TrimSpace(cell.Value)
		if value == "" {
			value = "0"
		}
		return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, cell.Style, xmlEscape(value))
	}
	return fmt.Sprintf(`<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, cell.Style, xmlEscape(cell.Value))
}

func cellRef(col, row int) string {
	return excelColumn(col) + strconv.Itoa(row)
}

func excelColumn(col int) string {
	if col < 1 {
		return "A"
	}
	var out []byte
	for col > 0 {
		col--
		out = append([]byte{byte('A' + col%26)}, out...)
		col /= 26
	}
	return string(out)
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(value)
}

func safeHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return value
}

func linkStyle(target string) int {
	if target == "" {
		return 12
	}
	return 18
}

func linkLabel(target, fallback, label string) string {
	if target != "" {
		return label
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "-"
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

func describeCustomerFilters(values url.Values) string {
	parts := make([]string, 0, 9)
	appendPart := func(label, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, label+": "+value)
		}
	}
	appendPart("Pencarian", values.Get("q"))
	appendPart("Niche", values.Get("preset"))
	appendPart("Area", values.Get("area"))
	appendPart("Subarea", values.Get("subarea"))
	appendPart("Min rating", values.Get("min_rating"))
	if values.Get("has_phone") == "1" {
		parts = append(parts, "Kontak: hanya yang memiliki HP")
	}
	if segment := values.Get("segment"); segment != "" {
		parts = append(parts, "Segment: "+segmentLabel(segment))
	}
	appendPart("Target", values.Get("target"))
	if verification := values.Get("verification_status"); verification != "" {
		parts = append(parts, "Verifikasi: "+verificationLabel(verification))
	}
	if len(parts) == 0 {
		return "Tidak ada filter khusus. File berisi seluruh lead customer-safe yang tersedia."
	}
	return strings.Join(parts, " • ")
}

func buildHyperlinkRelsXML(links []workbookHyperlink) string {
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i, link := range links {
		b.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`, i+1, xmlEscape(link.Target)))
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`

const workbookContentTypesXML = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`
const workbookRootRelsXML = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
const workbookXML = xmlHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView xWindow="0" yWindow="0" windowWidth="24000" windowHeight="12000"/></bookViews><sheets><sheet name="SUMMARY" sheetId="1" r:id="rId1"/><sheet name="LEADS" sheetId="2" r:id="rId2"/></sheets></workbook>`
const workbookRelsXML = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`

const workbookStylesXML = xmlHeader + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="6"><font><sz val="11"/><name val="Calibri"/><family val="2"/><scheme val="minor"/></font><font><b/><sz val="20"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font><font><b/><sz val="12"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font><font><b/><sz val="15"/><color rgb="FF172033"/><name val="Calibri"/></font><font><b/><sz val="11"/><color rgb="FF172033"/><name val="Calibri"/></font><font><u/><sz val="11"/><color rgb="FF0563C1"/><name val="Calibri"/></font></fonts><fills count="7"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF172033"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FF217A3C"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFF5F7FB"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFEAF8EF"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFFFF5D9"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFD9E1EC"/></left><right style="thin"><color rgb="FFD9E1EC"/></right><top style="thin"><color rgb="FFD9E1EC"/></top><bottom style="thin"><color rgb="FFD9E1EC"/></bottom><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="19"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="2" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="2" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center"/></xf><xf numFmtId="0" fontId="4" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyAlignment="1"><alignment wrapText="1" vertical="top"/></xf><xf numFmtId="0" fontId="2" fillId="3" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/><xf numFmtId="0" fontId="3" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="2" fontId="3" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyNumberFormat="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="0" fillId="4" borderId="1" xfId="0" applyFill="1" applyBorder="1" applyAlignment="1"><alignment wrapText="1" vertical="top"/></xf><xf numFmtId="0" fontId="4" fillId="0" borderId="0" xfId="0" applyFont="1"/><xf numFmtId="0" fontId="2" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="top" wrapText="1"/></xf><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf><xf numFmtId="0" fontId="0" fillId="4" borderId="1" xfId="0" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="4" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="4" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="2" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyNumberFormat="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="5" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`
