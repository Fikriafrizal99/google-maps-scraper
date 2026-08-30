package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

type queuePageData struct {
	HasLead       bool
	Lead          leadstore.Lead
	Images        []string
	Enrichment    leadstore.KostEnrichment
	Review        leadstore.Review
	Position      int
	Total         int
	HasPrev       bool
	HasNext       bool
	PrevURL       string
	NextURL       string
	CurrentURL    string
	BackURL       string
	FilterSummary string
	PendingOnly   bool
}

var queueTmpl = template.Must(template.New("queue").Funcs(template.FuncMap{
	"wa":                waNumber,
	"reviewLabel":       reviewLabel,
	"editValue":         editValue,
	"verificationLabel": verificationLabel,
}).Parse(queueHTML))

func (a *app) handleQueue(w http.ResponseWriter, r *http.Request) {
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
		"",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	verification := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("verification_status")))
	pendingOnly := verification == "" && !strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("queue")), "all")
	rows = filterQueueRows(rows, verification, pendingOnly)

	baseValues := queueBaseValues(r.URL.Query())
	backURL := buildExportURL("/", dashboardValues(baseValues))
	data := queuePageData{
		Total:         len(rows),
		BackURL:       backURL,
		FilterSummary: describeCustomerFilters(baseValues),
		PendingOnly:   pendingOnly,
	}
	if len(rows) == 0 {
		if err := queueTmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	index := queueIndex(rows, r.URL.Query().Get("id"))
	row := rows[index]
	data.HasLead = true
	data.Lead = row.Lead
	data.Images = leadImages(row.Images, row.Thumbnail, 4)
	data.Enrichment = row.Enrichment
	data.Review = row.Review
	data.Position = index + 1
	data.HasPrev = index > 0
	data.HasNext = index+1 < len(rows)
	data.CurrentURL = queueURL(baseValues, row.ID)
	if data.HasPrev {
		data.PrevURL = queueURL(baseValues, rows[index-1].ID)
	} else {
		data.PrevURL = data.CurrentURL
	}
	if data.HasNext {
		data.NextURL = queueURL(baseValues, rows[index+1].ID)
	} else {
		data.NextURL = buildExportURL("/queue", baseValues)
	}

	if err := queueTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) handleQueueSave(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid lead id", http.StatusBadRequest)
		return
	}
	if _, err := a.store.Get(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid queue form", http.StatusBadRequest)
		return
	}
	if err := validateEnrichmentForm(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	verification := r.FormValue("verification_status")
	reviewStatus := r.FormValue("review_status")
	switch strings.ToLower(strings.TrimSpace(r.FormValue("quick"))) {
	case "verified":
		verification = leadstore.VerificationVerified
	case "needs_check":
		verification = leadstore.VerificationNeedsCheck
	case "exclude":
		reviewStatus = leadstore.ReviewExclude
	}
	if _, err := leadstore.NormalizeReviewStatus(reviewStatus); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	e := leadstore.KostEnrichment{
		LeadID:             id,
		Segment:            r.FormValue("segment"),
		Target:             r.FormValue("target"),
		RentalType:         r.FormValue("rental_type"),
		PriceRange:         r.FormValue("price_range"),
		Facilities:         r.FormValue("facilities"),
		Furnish:            r.FormValue("furnish"),
		Rules:              r.FormValue("rules"),
		Landmark:           r.FormValue("landmark"),
		SellingPoint:       r.FormValue("selling_point"),
		VerificationStatus: verification,
	}
	if err := a.store.UpdateEnrichment(r.Context(), e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.store.UpdateReview(r.Context(), id, reviewStatus, r.FormValue("review_note")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	redirect := safeQueueRedirect(r.FormValue("next_url"))
	if redirect == "" {
		redirect = "/queue"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func filterQueueRows(rows []dashboardRow, verification string, pendingOnly bool) []dashboardRow {
	verification = strings.ToLower(strings.TrimSpace(verification))
	out := make([]dashboardRow, 0, len(rows))
	for _, row := range rows {
		if row.Review.Status == leadstore.ReviewExclude {
			continue
		}
		current := strings.ToLower(strings.TrimSpace(row.Enrichment.VerificationStatus))
		if current == "" {
			current = leadstore.VerificationUnverified
		}
		if verification != "" && current != verification {
			continue
		}
		if verification == "" && pendingOnly && current == leadstore.VerificationVerified {
			continue
		}
		out = append(out, row)
	}
	return out
}

func queueIndex(rows []dashboardRow, rawID string) int {
	id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return 0
}

func queueBaseValues(values url.Values) url.Values {
	out := url.Values{}
	for _, key := range []string{"q", "preset", "area", "subarea", "min_rating", "has_phone", "review_status", "segment", "target", "verification_status", "queue"} {
		for _, value := range values[key] {
			if strings.TrimSpace(value) != "" {
				out.Add(key, value)
			}
		}
	}
	return out
}

func dashboardValues(values url.Values) url.Values {
	out := url.Values{}
	for key, items := range values {
		if key == "queue" || key == "id" {
			continue
		}
		for _, item := range items {
			out.Add(key, item)
		}
	}
	return out
}

func queueURL(values url.Values, id int64) string {
	copyValues := queueBaseValues(values)
	copyValues.Set("id", strconv.FormatInt(id, 10))
	query := copyValues.Encode()
	if query == "" {
		return fmt.Sprintf("/queue?id=%d", id)
	}
	return "/queue?" + query
}

func safeQueueRedirect(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/queue") {
		return ""
	}
	return parsed.RequestURI()
}

//go:embed queue.html
var queueHTML string
