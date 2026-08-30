package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

var customerExportHeader = []string{
	"business_name",
	"google_category",
	"address",
	"area",
	"location_scope",
	"phone",
	"website",
	"rating",
	"review_count",
	"maps_url",
	"photo_url",
	"business_scale",
	"prospect_priority",
	"business_type_detail",
	"operational_scale",
	"products_services",
	"contact_status",
	"internal_notes",
	"service_area",
	"prospect_fit",
	"verification_status",
}

func (a *app) handleCustomerExportCSV(w http.ResponseWriter, r *http.Request) {
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
	filename := "business-prospects-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// UTF-8 BOM helps Excel open Indonesian text without encoding issues.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write(customerExportHeader)
	for _, row := range rows {
		_ = writer.Write(customerExportRecord(row))
	}
}

func customerSafeRows(rows []dashboardRow) []dashboardRow {
	out := make([]dashboardRow, 0, len(rows))
	for _, row := range rows {
		if row.Review.Status == leadstore.ReviewExclude {
			continue
		}
		out = append(out, row)
	}
	return out
}

func customerExportRecord(row dashboardRow) []string {
	lead := row.Lead
	e := row.Enrichment
	photo := ""
	if images := leadImages(lead.Images, lead.Thumbnail, 1); len(images) > 0 {
		photo = images[0]
	}
	return []string{
		strings.TrimSpace(lead.Title),
		strings.TrimSpace(lead.Category),
		strings.TrimSpace(lead.Address),
		strings.TrimSpace(lead.Area),
		strings.TrimSpace(lead.Subarea),
		strings.TrimSpace(lead.Phone),
		strings.TrimSpace(lead.Website),
		strconv.FormatFloat(lead.Rating, 'f', 1, 64),
		strconv.Itoa(lead.ReviewCount),
		strings.TrimSpace(lead.Link),
		photo,
		customerValue(e.Segment),
		customerValue(e.Target),
		customerValue(e.RentalType),
		customerValue(e.PriceRange),
		customerValue(e.Facilities),
		customerValue(e.Furnish),
		customerValue(e.Rules),
		customerValue(e.Landmark),
		customerValue(e.SellingPoint),
		strings.TrimSpace(e.VerificationStatus),
	}
}

func customerValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, leadstore.EnrichmentUnknown) {
		return ""
	}
	return value
}