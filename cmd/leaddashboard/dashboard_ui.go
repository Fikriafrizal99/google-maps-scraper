package main

import (
	"context"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

type dashboardRow struct {
	leadstore.Lead
	Review     leadstore.Review
	Enrichment leadstore.KostEnrichment
}

type dashboardDetailData struct {
	leadstore.Lead
	Images     []string
	Review     leadstore.Review
	Enrichment leadstore.KostEnrichment
}

type dashboardPageData struct {
	Stats              leadstore.Stats
	Leads              []dashboardRow
	Query              string
	Preset             string
	Area               string
	Subarea            string
	HasPhone           bool
	MinRating          string
	ReviewStatus       string
	Segment            string
	Target             string
	VerificationStatus string
	FilteredTotal      int
	Page               int
	TotalPages         int
	PrevPage           int
	NextPage           int
	HasPrev            bool
	HasNext            bool
	FilterQuery        string
	Collect            collectState
}

var dashboardV2Tmpl = template.Must(template.New("dashboard-v2").Funcs(template.FuncMap{
	"wa":                waNumber,
	"shortTime":         shortTime,
	"reviewLabel":       reviewLabel,
	"displayValue":      displayValue,
	"editValue":         editValue,
	"segmentLabel":      segmentLabel,
	"verificationLabel": verificationLabel,
}).Parse(dashboardV2HTML))

var detailV2Tmpl = template.Must(template.New("detail-v2").Funcs(template.FuncMap{
	"wa":                waNumber,
	"shortTime":         shortTime,
	"reviewLabel":       reviewLabel,
	"displayValue":      displayValue,
	"editValue":         editValue,
	"segmentLabel":      segmentLabel,
	"verificationLabel": verificationLabel,
}).Parse(detailV2HTML))

func (a *app) filterFromRequest(r *http.Request, limit int) leadstore.Filter {
	minRating, _ := strconv.ParseFloat(r.URL.Query().Get("min_rating"), 64)
	return leadstore.Filter{
		Preset:    strings.TrimSpace(r.URL.Query().Get("preset")),
		Area:      strings.TrimSpace(r.URL.Query().Get("area")),
		Subarea:   strings.TrimSpace(r.URL.Query().Get("subarea")),
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		HasPhone:  r.URL.Query().Get("has_phone") == "1",
		MinRating: minRating,
		Limit:     limit,
	}
}

func (a *app) dashboardRows(ctx context.Context, leads []leadstore.Lead, reviewStatus, segment, target, verification string) ([]dashboardRow, error) {
	ids := make([]int64, len(leads))
	for i, lead := range leads {
		ids[i] = lead.ID
	}
	reviews, err := a.store.ReviewMap(ctx, ids)
	if err != nil {
		return nil, err
	}
	enrichment, err := a.store.EnrichmentMap(ctx, ids)
	if err != nil {
		return nil, err
	}

	reviewStatus = strings.TrimSpace(reviewStatus)
	segment = strings.ToLower(strings.TrimSpace(segment))
	target = strings.ToLower(strings.TrimSpace(target))
	verification = strings.ToLower(strings.TrimSpace(verification))

	if reviewStatus != "" {
		if _, err := leadstore.NormalizeReviewStatus(reviewStatus); err != nil {
			return nil, err
		}
	}
	if verification != "" && verification != leadstore.VerificationUnverified && verification != leadstore.VerificationVerified && verification != leadstore.VerificationNeedsCheck {
		return nil, fmt.Errorf("invalid verification status %q", verification)
	}

	rows := make([]dashboardRow, 0, len(leads))
	for _, lead := range leads {
		review := reviews[lead.ID]
		if review.Status == "" {
			review.Status = leadstore.ReviewUnreviewed
		}
		e := enrichment[lead.ID]
		if reviewStatus != "" && review.Status != reviewStatus {
			continue
		}
		if segment != "" && strings.ToLower(e.Segment) != segment {
			continue
		}
		if target != "" && strings.ToLower(e.Target) != target {
			continue
		}
		if verification != "" && strings.ToLower(e.VerificationStatus) != verification {
			continue
		}
		rows = append(rows, dashboardRow{Lead: lead, Review: review, Enrichment: e})
	}
	return rows, nil
}

func (a *app) handleDashboardV2(w http.ResponseWriter, r *http.Request) {
	filter := a.filterFromRequest(r, 5000)
	allLeads, err := a.store.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reviewStatus := strings.TrimSpace(r.URL.Query().Get("review_status"))
	segment := strings.TrimSpace(r.URL.Query().Get("segment"))
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	verification := strings.TrimSpace(r.URL.Query().Get("verification_status"))
	rows, err := a.dashboardRows(r.Context(), allLeads, reviewStatus, segment, target, verification)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stats, err := a.store.Stats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	totalPages := (len(rows) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	if start > len(rows) {
		start = len(rows)
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}

	values := url.Values{}
	if filter.Query != "" {
		values.Set("q", filter.Query)
	}
	if filter.Preset != "" {
		values.Set("preset", filter.Preset)
	}
	if filter.Area != "" {
		values.Set("area", filter.Area)
	}
	if filter.Subarea != "" {
		values.Set("subarea", filter.Subarea)
	}
	if filter.MinRating > 0 {
		values.Set("min_rating", r.URL.Query().Get("min_rating"))
	}
	if filter.HasPhone {
		values.Set("has_phone", "1")
	}
	if reviewStatus != "" {
		values.Set("review_status", reviewStatus)
	}
	if segment != "" {
		values.Set("segment", segment)
	}
	if target != "" {
		values.Set("target", target)
	}
	if verification != "" {
		values.Set("verification_status", verification)
	}

	data := dashboardPageData{
		Stats: stats, Leads: rows[start:end], Query: filter.Query, Preset: filter.Preset,
		Area: filter.Area, Subarea: filter.Subarea, HasPhone: filter.HasPhone,
		MinRating: r.URL.Query().Get("min_rating"), ReviewStatus: reviewStatus,
		Segment: segment, Target: target, VerificationStatus: verification,
		FilteredTotal: len(rows), Page: page, TotalPages: totalPages,
		PrevPage: page - 1, NextPage: page + 1, HasPrev: page > 1, HasNext: page < totalPages,
		FilterQuery: values.Encode(), Collect: a.collectStatus(),
	}
	if err := dashboardV2Tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) handleLeadDetailV2(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid lead id", http.StatusBadRequest)
		return
	}
	lead, err := a.store.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	review, err := a.store.GetReview(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	enrichment, err := a.store.GetEnrichment(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := dashboardDetailData{
		Lead: lead, Images: leadImages(lead.Images, lead.Thumbnail, 5),
		Review: review, Enrichment: enrichment,
	}
	if err := detailV2Tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) handleLeadReviewV2(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "invalid review form", http.StatusBadRequest)
		return
	}
	if err := a.store.UpdateReview(r.Context(), id, r.FormValue("status"), r.FormValue("note")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/lead/%d?review=saved", id), http.StatusSeeOther)
}

func (a *app) handleLeadEnrichment(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "invalid enrichment form", http.StatusBadRequest)
		return
	}
	if err := validateEnrichmentForm(r); err != nil {
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
		VerificationStatus: r.FormValue("verification_status"),
	}
	if err := a.store.UpdateEnrichment(r.Context(), e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/lead/%d?enrichment=saved", id), http.StatusSeeOther)
}

func validateEnrichmentForm(r *http.Request) error {
	limits := map[string]int{
		"segment": 100, "target": 100, "rental_type": 200, "price_range": 200,
		"facilities": 2000, "furnish": 500, "rules": 2000, "landmark": 500,
		"selling_point": 1000, "verification_status": 50,
	}
	for field, limit := range limits {
		if len(strings.TrimSpace(r.FormValue(field))) > limit {
			return fmt.Errorf("%s terlalu panjang", field)
		}
	}
	return nil
}

func (a *app) handleAPILeadsV2(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func (a *app) handleExportCSVV2(w http.ResponseWriter, r *http.Request) {
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

	filename := "leads-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{
		"preset", "area", "subarea", "title", "category", "address", "phone", "website",
		"rating", "review_count", "latitude", "longitude", "maps_url", "thumbnail", "images",
		"segment", "target", "rental_type", "price_range", "facilities", "furnish", "rules",
		"landmark", "selling_point", "verification_status", "enrichment_source", "enrichment_updated_at",
		"review_status", "review_note", "reviewed_at", "first_seen", "last_checked",
	})
	for _, row := range rows {
		lead := row.Lead
		e := row.Enrichment
		_ = writer.Write([]string{
			lead.Preset, lead.Area, lead.Subarea, lead.Title, lead.Category, lead.Address,
			lead.Phone, lead.Website, strconv.FormatFloat(lead.Rating, 'f', 1, 64), strconv.Itoa(lead.ReviewCount),
			lead.Latitude, lead.Longitude, lead.Link, lead.Thumbnail, lead.Images,
			e.Segment, e.Target, e.RentalType, e.PriceRange, e.Facilities, e.Furnish, e.Rules,
			e.Landmark, e.SellingPoint, e.VerificationStatus, e.Source, e.UpdatedAt,
			row.Review.Status, row.Review.Note, row.Review.ReviewedAt, lead.FirstSeen, lead.LastChecked,
		})
	}
}

func displayValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, leadstore.EnrichmentUnknown) {
		return "-"
	}
	return value
}

func editValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, leadstore.EnrichmentUnknown) {
		return ""
	}
	return value
}

func segmentLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "putra":
		return "Putra"
	case "putri":
		return "Putri"
	case "campur":
		return "Campur"
	case "pasutri":
		return "Pasutri"
	case "umum":
		return "Umum"
	default:
		return "Belum diketahui"
	}
}

func verificationLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case leadstore.VerificationVerified:
		return "Terverifikasi"
	case leadstore.VerificationNeedsCheck:
		return "Perlu dicek"
	default:
		return "Belum diverifikasi"
	}
}

//go:embed dashboard.html
var dashboardV2HTML string

//go:embed detail.html
var detailV2HTML string
