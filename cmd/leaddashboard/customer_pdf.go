package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	catalogPDFWidth  = 595.28
	catalogPDFHeight = 841.89
)

type catalogImage struct {
	JPEG   []byte
	Width  int
	Height int
}

type catalogImageFetcher func(context.Context, string) (catalogImage, error)

type catalogPDFLink struct {
	X, Top, W, H float64
	URL          string
}

type catalogPDFPageImage struct {
	Name         string
	Image        catalogImage
	X, Top, W, H float64
}

type catalogPDFPage struct {
	content strings.Builder
	images  []catalogPDFPageImage
	links   []catalogPDFLink
}

type catalogPDFDocument struct {
	pages []catalogPDFPage
}

func (a *app) handleCustomerExportPDF(w http.ResponseWriter, r *http.Request) {
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
	content, err := buildCustomerPDF(r.Context(), rows, r.URL.Query(), now, downloadCatalogImage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filename := strings.TrimSuffix(customerWorkbookFilename(rows, r.URL.Query(), now), ".xlsx") + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

func buildCustomerPDF(ctx context.Context, rows []dashboardRow, filters url.Values, generatedAt time.Time, fetchImage catalogImageFetcher) ([]byte, error) {
	doc := &catalogPDFDocument{}
	doc.pages = append(doc.pages, buildCatalogCoverPage(rows, filters, generatedAt))
	packageTitle := customerPackageTitle(rows, filters)
	imageLimit := catalogImageLimit(len(rows))
	cache := map[string]catalogImage{}

	for i, row := range rows {
		images := make([]catalogImage, 0, imageLimit)
		for _, imageURL := range leadImages(row.Images, row.Thumbnail, imageLimit) {
			imageURL = safeHTTPURL(imageURL)
			if imageURL == "" {
				continue
			}
			if cached, ok := cache[imageURL]; ok {
				images = append(images, cached)
				continue
			}
			if fetchImage == nil {
				continue
			}
			img, err := fetchImage(ctx, imageURL)
			if err != nil {
				continue
			}
			cache[imageURL] = img
			images = append(images, img)
		}
		doc.pages = append(doc.pages, buildCatalogLeadPage(row, i+1, len(rows), packageTitle, generatedAt, images, imageLimit))
	}
	return doc.bytes()
}

func catalogImageLimit(total int) int {
	switch {
	case total <= 60:
		return 3
	case total <= 150:
		return 2
	default:
		return 1
	}
}

func catalogWhatsAppURL(phone string) string {
	phone = waNumber(phone)
	if phone == "" {
		return ""
	}
	return "https://wa.me/" + phone
}

var (
	pdfDark        = [3]float64{23.0 / 255, 32.0 / 255, 51.0 / 255}
	pdfGreen       = [3]float64{33.0 / 255, 122.0 / 255, 60.0 / 255}
	pdfGreenLight  = [3]float64{234.0 / 255, 248.0 / 255, 239.0 / 255}
	pdfYellowLight = [3]float64{1, 245.0 / 255, 217.0 / 255}
	pdfLight       = [3]float64{245.0 / 255, 247.0 / 255, 251.0 / 255}
	pdfBorder      = [3]float64{217.0 / 255, 225.0 / 255, 236.0 / 255}
	pdfMuted       = [3]float64{113.0 / 255, 128.0 / 255, 150.0 / 255}
	pdfWhite       = [3]float64{1, 1, 1}
)
