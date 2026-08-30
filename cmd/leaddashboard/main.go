package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

const pageSize = 25

type collectState struct {
	Running   bool
	Message   string
	StartedAt string
}

type imageRecord struct {
	Image string `json:"image"`
}

type leadRow struct {
	leadstore.Lead
	Review leadstore.Review
}

type detailData struct {
	leadstore.Lead
	Images []string
	Review leadstore.Review
}

type app struct {
	store         *leadstore.Store
	dashboardTmpl *template.Template
	detailTmpl    *template.Template
	user          string
	pass          string
	dbPath        string
	collectorPath string
	enginePath    string
	configDir     string
	collectMu     sync.RWMutex
	collect       collectState
}

type pageData struct {
	Stats         leadstore.Stats
	Leads         []leadRow
	Query         string
	Preset        string
	Area          string
	Subarea       string
	HasPhone      bool
	MinRating     string
	ReviewStatus  string
	FilteredTotal int
	Page          int
	TotalPages    int
	PrevPage      int
	NextPage      int
	HasPrev       bool
	HasNext       bool
	FilterQuery   string
	Collect       collectState
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "dashboard listen address")
	dbPath := flag.String("db", filepath.FromSlash("data/leads.db"), "SQLite master database path")
	collectorPath := flag.String("collector", filepath.FromSlash("bin/collector"), "collector binary path")
	enginePath := flag.String("engine", filepath.FromSlash("bin/google_maps_scraper"), "scraper engine binary path")
	configDir := flag.String("config-dir", "config", "collector config directory")
	flag.Parse()

	if err := os.MkdirAll("data", 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	store, err := leadstore.Open(*dbPath)
	if err != nil {
		log.Fatalf("open lead database: %v", err)
	}
	defer store.Close()
	if err := store.EnsureReviewSchema(context.Background()); err != nil {
		log.Fatalf("open review database: %v", err)
	}

	funcs := template.FuncMap{
		"wa":          waNumber,
		"shortTime":   shortTime,
		"reviewLabel": reviewLabel,
	}
	dashboardTmpl, err := template.New("dashboard").Funcs(funcs).Parse(dashboardHTML)
	if err != nil {
		log.Fatalf("parse dashboard template: %v", err)
	}
	detailTmpl, err := template.New("detail").Funcs(funcs).Parse(detailHTML)
	if err != nil {
		log.Fatalf("parse detail template: %v", err)
	}

	a := &app{
		store:         store,
		dashboardTmpl: dashboardTmpl,
		detailTmpl:    detailTmpl,
		user:          strings.TrimSpace(os.Getenv("LEADS_USER")),
		pass:          os.Getenv("LEADS_PASS"),
		dbPath:        *dbPath,
		collectorPath: *collectorPath,
		enginePath:    *enginePath,
		configDir:     *configDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("GET /lead/{id}", a.handleLeadDetail)
	mux.HandleFunc("POST /lead/{id}/review", a.handleLeadReview)
	mux.HandleFunc("GET /api/leads", a.handleAPILeads)
	mux.HandleFunc("GET /export.csv", a.handleExportCSV)
	mux.HandleFunc("POST /collect", a.handleCollect)
	mux.HandleFunc("POST /import", a.handleImport)

	server := &http.Server{
		Addr:              *addr,
		Handler:           a.auth(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("lead dashboard: http://%s", *addr)
	if a.user == "" || a.pass == "" {
		log.Printf("warning: LEADS_USER/LEADS_PASS not set; dashboard has no login")
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func (a *app) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.user == "" || a.pass == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != a.user || pass != a.pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Lead Dashboard"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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

func (a *app) reviewedRows(ctx context.Context, leads []leadstore.Lead, status string) ([]leadRow, error) {
	ids := make([]int64, len(leads))
	for i, lead := range leads {
		ids[i] = lead.ID
	}
	reviews, err := a.store.ReviewMap(ctx, ids)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	rows := make([]leadRow, 0, len(leads))
	for _, lead := range leads {
		review := reviews[lead.ID]
		if review.Status == "" {
			review.Status = leadstore.ReviewUnreviewed
		}
		if status != "" && review.Status != status {
			continue
		}
		rows = append(rows, leadRow{Lead: lead, Review: review})
	}
	return rows, nil
}

func (a *app) handleDashboard(w http.ResponseWriter, r *http.Request) {
	filter := a.filterFromRequest(r, 5000)
	allLeads, err := a.store.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reviewStatus := strings.TrimSpace(r.URL.Query().Get("review_status"))
	if reviewStatus != "" {
		if _, err := leadstore.NormalizeReviewStatus(reviewStatus); err != nil {
			http.Error(w, "invalid review status", http.StatusBadRequest)
			return
		}
	}
	rows, err := a.reviewedRows(r.Context(), allLeads, reviewStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	if filter.Query != "" { values.Set("q", filter.Query) }
	if filter.Preset != "" { values.Set("preset", filter.Preset) }
	if filter.Area != "" { values.Set("area", filter.Area) }
	if filter.Subarea != "" { values.Set("subarea", filter.Subarea) }
	if filter.MinRating > 0 { values.Set("min_rating", r.URL.Query().Get("min_rating")) }
	if filter.HasPhone { values.Set("has_phone", "1") }
	if reviewStatus != "" { values.Set("review_status", reviewStatus) }

	data := pageData{
		Stats: stats, Leads: rows[start:end], Query: filter.Query, Preset: filter.Preset,
		Area: filter.Area, Subarea: filter.Subarea, HasPhone: filter.HasPhone,
		MinRating: r.URL.Query().Get("min_rating"), ReviewStatus: reviewStatus,
		FilteredTotal: len(rows), Page: page, TotalPages: totalPages,
		PrevPage: page - 1, NextPage: page + 1, HasPrev: page > 1, HasNext: page < totalPages,
		FilterQuery: values.Encode(), Collect: a.collectStatus(),
	}
	if err := a.dashboardTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) handleLeadDetail(w http.ResponseWriter, r *http.Request) {
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
	data := detailData{Lead: lead, Images: leadImages(lead.Images, lead.Thumbnail, 5), Review: review}
	if err := a.detailTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) handleLeadReview(w http.ResponseWriter, r *http.Request) {
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

func (a *app) handleAPILeads(w http.ResponseWriter, r *http.Request) {
	leads, err := a.store.List(r.Context(), a.filterFromRequest(r, 5000))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leads)
}

func (a *app) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	leads, err := a.store.List(r.Context(), a.filterFromRequest(r, 5000))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("review_status"))
	rows, err := a.reviewedRows(r.Context(), leads, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filename := "leads-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"preset", "area", "subarea", "title", "category", "address", "phone", "website", "rating", "review_count", "latitude", "longitude", "maps_url", "thumbnail", "images", "review_status", "review_note", "reviewed_at", "first_seen", "last_checked"})
	for _, row := range rows {
		lead := row.Lead
		_ = writer.Write([]string{
			lead.Preset, lead.Area, lead.Subarea, lead.Title, lead.Category, lead.Address,
			lead.Phone, lead.Website, strconv.FormatFloat(lead.Rating, 'f', 1, 64), strconv.Itoa(lead.ReviewCount),
			lead.Latitude, lead.Longitude, lead.Link, lead.Thumbnail, lead.Images,
			row.Review.Status, row.Review.Note, row.Review.ReviewedAt, lead.FirstSeen, lead.LastChecked,
		})
	}
}

func (a *app) handleCollect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	preset := strings.TrimSpace(r.FormValue("preset"))
	area := strings.TrimSpace(r.FormValue("area"))
	subarea := strings.TrimSpace(r.FormValue("subarea"))
	if !safeConfigName(preset) || !safeConfigName(area) {
		http.Error(w, "preset/area hanya boleh huruf, angka, dash, underscore", http.StatusBadRequest)
		return
	}
	if len(subarea) > 120 {
		http.Error(w, "subarea terlalu panjang", http.StatusBadRequest)
		return
	}
	depth := parseBoundedInt(r.FormValue("depth"), 5, 1, 30)
	concurrency := parseBoundedInt(r.FormValue("concurrency"), 2, 1, 8)

	a.collectMu.Lock()
	if a.collect.Running {
		a.collectMu.Unlock()
		http.Error(w, "collector masih berjalan", http.StatusConflict)
		return
	}
	a.collect = collectState{Running: true, Message: fmt.Sprintf("Collect %s / %s dimulai", preset, area), StartedAt: time.Now().Format("2006-01-02 15:04:05")}
	a.collectMu.Unlock()

	go a.runCollector(preset, area, subarea, depth, concurrency)
	http.Redirect(w, r, "/?collect=started", http.StatusSeeOther)
}

func (a *app) runCollector(preset, area, subarea string, depth, concurrency int) {
	stamp := time.Now().Format("20060102-150405")
	output := filepath.Join("data", fmt.Sprintf("latest-%s-%s.csv", preset, area))
	logPath := filepath.Join("data", "collector-last.log")
	args := []string{"-preset", preset, "-area", area, "-config-dir", a.configDir, "-engine", a.enginePath, "-output", output, "-db", a.dbPath}
	if subarea != "" { args = append(args, "-subarea", subarea) }
	args = append(args, "--", "-c", strconv.Itoa(concurrency), "-depth", strconv.Itoa(depth))

	logFile, err := os.Create(logPath)
	if err != nil {
		a.finishCollect("Gagal membuat collector log: " + err.Error())
		return
	}
	defer logFile.Close()
	cmd := exec.CommandContext(context.Background(), a.collectorPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		a.finishCollect(fmt.Sprintf("Collect gagal (%s). Lihat %s", err, logPath))
		return
	}
	a.finishCollect(fmt.Sprintf("Collect selesai %s · output %s · log %s", stamp, output, logPath))
}

func (a *app) finishCollect(message string) {
	a.collectMu.Lock()
	a.collect = collectState{Running: false, Message: message}
	a.collectMu.Unlock()
}

func (a *app) collectStatus() collectState {
	a.collectMu.RLock()
	defer a.collectMu.RUnlock()
	return a.collect
}

func (a *app) handleImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	preset := strings.TrimSpace(r.FormValue("preset"))
	area := strings.TrimSpace(r.FormValue("area"))
	subarea := strings.TrimSpace(r.FormValue("subarea"))
	if preset == "" || area == "" {
		http.Error(w, "preset and area are required", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "csv file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "lead-import-*.csv")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.ReadFrom(file); err != nil {
		tmp.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	count, err := a.store.ImportCSV(context.Background(), path, preset, area, subarea)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?imported="+strconv.Itoa(count), http.StatusSeeOther)
}

func safeConfigName(value string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(value)
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 { return fallback }
	return n
}

func parseBoundedInt(value string, fallback, min, max int) int {
	n, err := strconv.Atoi(value)
	if err != nil { return fallback }
	if n < min { return min }
	if n > max { return max }
	return n
}

func waNumber(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' { digits.WriteRune(r) }
	}
	value := digits.String()
	if strings.HasPrefix(value, "0") { return "62" + strings.TrimPrefix(value, "0") }
	return value
}

func shortTime(value string) string {
	if strings.TrimSpace(value) == "" { return "-" }
	t, err := time.Parse(time.RFC3339, value)
	if err != nil { return value }
	return t.Local().Format("02 Jan 2006 15:04")
}

func reviewLabel(status string) string {
	switch status {
	case leadstore.ReviewValid: return "Valid"
	case leadstore.ReviewNeedsReview: return "Needs Review"
	case leadstore.ReviewExclude: return "Exclude"
	default: return "Unreviewed"
	}
}

func leadImages(raw, thumbnail string, limit int) []string {
	if limit <= 0 { limit = 5 }
	seen := make(map[string]struct{})
	images := make([]string, 0, limit)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || len(images) >= limit { return }
		if _, ok := seen[value]; ok { return }
		seen[value] = struct{}{}
		images = append(images, value)
	}
	var records []imageRecord
	if json.Unmarshal([]byte(raw), &records) == nil {
		for _, record := range records { add(record.Image) }
	}
	add(thumbnail)
	return images
}

const dashboardHTML = `<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Lead Control</title>
<style>
:root{font-family:Inter,system-ui,-apple-system,sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0}.wrap{max-width:1500px;margin:auto;padding:24px}.top{display:flex;justify-content:space-between;gap:20px;align-items:flex-end;margin-bottom:18px}h1{margin:0;font-size:28px}.muted{color:#718096;font-size:13px}.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin:18px 0}.card,.panel{background:#fff;border:1px solid #e8edf5;border-radius:14px;box-shadow:0 4px 18px rgba(18,38,63,.04)}.card{padding:18px}.card b{display:block;font-size:26px;margin-top:5px}.panel{padding:16px;margin-bottom:16px}.section-title{font-size:15px;font-weight:700;margin:0 0 12px}.collect-grid{display:grid;grid-template-columns:1fr 1fr 1.5fr .65fr .65fr auto;gap:8px;align-items:end}.filters{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr 1.1fr auto auto;gap:8px;align-items:center}input,select,textarea,button{border:1px solid #d9e1ec;border-radius:9px;padding:8px 10px;background:#fff}input,select,button{min-height:40px}button,.btn{cursor:pointer;text-decoration:none;color:#172033;display:inline-flex;align-items:center;justify-content:center}.primary{background:#172033;color:#fff;border-color:#172033}.statusbox{padding:10px 12px;border-radius:9px;background:#f7f9fc;font-size:13px;margin-top:10px}.statusbox.running{background:#fff8e6}.tablewrap{overflow:auto;background:#fff;border:1px solid #e8edf5;border-radius:14px}table{width:100%;border-collapse:collapse;min-width:1250px}th,td{text-align:left;padding:10px 12px;border-bottom:1px solid #eef2f7;font-size:13px;vertical-align:middle}th{position:sticky;top:0;background:#fafbfd;color:#5c677d;z-index:1}.title{font-weight:700}.title a{color:#172033;text-decoration:none}.title a:hover{text-decoration:underline}.photo{width:58px}.thumb{width:54px;height:54px;object-fit:cover;border-radius:10px;background:#edf1f6;display:block}.placeholder{width:54px;height:54px;border-radius:10px;background:#edf1f6;display:flex;align-items:center;justify-content:center;color:#9aa4b2;font-size:10px}.badge{display:inline-flex;padding:5px 9px;border-radius:999px;font-size:11px;font-weight:700;background:#eef2f7;color:#667085}.badge.valid{background:#eaf8ef;color:#217a3c}.badge.needs_review{background:#fff5d9;color:#8a6400}.badge.exclude{background:#fdecec;color:#a92d2d}.actions{white-space:nowrap}.actions a{margin-right:9px}.pagination{display:flex;justify-content:space-between;align-items:center;padding:14px 4px;gap:10px}.smallbtn{padding:7px 12px;border:1px solid #d9e1ec;border-radius:8px;background:#fff;text-decoration:none;color:#172033}.smallbtn.disabled{opacity:.4;pointer-events:none}.utility summary{cursor:pointer;font-size:13px;color:#667085}.import{display:flex;gap:8px;align-items:end;flex-wrap:wrap;margin-top:12px}.import label,.collect-grid label{font-size:12px;color:#667085}.import label span,.collect-grid label span{display:block;margin-bottom:4px}@media(max-width:1000px){.collect-grid{grid-template-columns:1fr 1fr 1fr}.stats{grid-template-columns:1fr 1fr}.filters{grid-template-columns:1fr 1fr}.top{align-items:flex-start;flex-direction:column}}@media(max-width:560px){.wrap{padding:14px}.stats,.filters,.collect-grid{grid-template-columns:1fr}}
</style></head><body><div class="wrap">
<div class="top"><div><h1>Lead Control</h1><div class="muted">Master database internal · collect · quality control · filter · export</div></div><a class="btn" href="/export.csv?{{.FilterQuery}}">Export hasil filter CSV</a></div>
<div class="stats"><div class="card"><span class="muted">Total leads</span><b>{{.Stats.Total}}</b></div><div class="card"><span class="muted">Dengan telepon</span><b>{{.Stats.WithPhone}}</b></div><div class="card"><span class="muted">Dengan website</span><b>{{.Stats.WithWebsite}}</b></div><div class="card"><span class="muted">Rata-rata rating</span><b>{{printf "%.1f" .Stats.AvgRating}}</b></div></div>
<div class="panel"><div class="section-title">Collect / Refresh Data</div><form class="collect-grid" action="/collect" method="post"><label><span>Preset</span><input name="preset" value="kost" required></label><label><span>Area</span><input name="area" value="jakarta" required></label><label><span>Subarea (opsional)</span><input name="subarea" placeholder="Jakarta Selatan"></label><label><span>Depth</span><input name="depth" type="number" min="1" max="30" value="5"></label><label><span>Concurrency</span><input name="concurrency" type="number" min="1" max="8" value="2"></label><button class="primary" {{if .Collect.Running}}disabled{{end}}>{{if .Collect.Running}}Sedang jalan...{{else}}Mulai collect{{end}}</button></form>{{if .Collect.Message}}<div class="statusbox {{if .Collect.Running}}running{{end}}">{{.Collect.Message}}{{if .Collect.StartedAt}} · {{.Collect.StartedAt}}{{end}}</div>{{end}}</div>
<div class="panel"><form class="filters" method="get"><input name="q" placeholder="Cari nama, alamat, kategori..." value="{{.Query}}"><input name="preset" placeholder="Niche/preset" value="{{.Preset}}"><input name="area" placeholder="Area" value="{{.Area}}"><input name="subarea" placeholder="Subarea" value="{{.Subarea}}"><input name="min_rating" type="number" step="0.1" min="0" max="5" placeholder="Min rating" value="{{.MinRating}}"><select name="review_status"><option value="">Semua status</option><option value="unreviewed" {{if eq .ReviewStatus "unreviewed"}}selected{{end}}>Unreviewed</option><option value="valid" {{if eq .ReviewStatus "valid"}}selected{{end}}>Valid</option><option value="needs_review" {{if eq .ReviewStatus "needs_review"}}selected{{end}}>Needs Review</option><option value="exclude" {{if eq .ReviewStatus "exclude"}}selected{{end}}>Exclude</option></select><label><input type="checkbox" name="has_phone" value="1" {{if .HasPhone}}checked{{end}}> Ada HP</label><button class="primary">Filter</button></form><div class="muted" style="margin-top:9px">{{.FilteredTotal}} lead cocok dengan filter saat ini.</div></div>
<div class="tablewrap"><table><thead><tr><th>Foto</th><th>Lead</th><th>Status</th><th>Lokasi</th><th>Kontak</th><th>Rating</th><th>Updated</th><th>Aksi</th></tr></thead><tbody>{{range .Leads}}<tr><td><a href="/lead/{{.ID}}">{{if .Thumbnail}}<img class="thumb" src="{{.Thumbnail}}" loading="lazy" referrerpolicy="no-referrer" onerror="this.style.display='none'">{{else}}<div class="placeholder">No photo</div>{{end}}</a></td><td><div class="title"><a href="/lead/{{.ID}}">{{.Title}}</a></div><div class="muted">{{.Category}} · {{.Preset}}</div></td><td><span class="badge {{.Review.Status}}">{{reviewLabel .Review.Status}}</span></td><td>{{.Address}}<div class="muted">{{.Area}} {{.Subarea}}</div></td><td>{{if .Phone}}{{.Phone}}{{else}}-{{end}}<div>{{if .Website}}<a href="{{.Website}}" target="_blank" rel="noreferrer">Website</a>{{end}}</div></td><td>{{printf "%.1f" .Rating}} <span class="muted">({{.ReviewCount}})</span></td><td class="muted">{{shortTime .LastChecked}}</td><td class="actions">{{if .Link}}<a href="{{.Link}}" target="_blank" rel="noreferrer">Maps</a>{{end}}{{if .Phone}}<a href="https://wa.me/{{wa .Phone}}" target="_blank" rel="noreferrer">WA</a>{{end}}</td></tr>{{else}}<tr><td colspan="8">Belum ada data yang cocok dengan filter.</td></tr>{{end}}</tbody></table></div>
<div class="pagination"><div class="muted">Halaman {{.Page}} / {{.TotalPages}} · maksimal 5.000 hasil per filter</div><div><a class="smallbtn {{if not .HasPrev}}disabled{{end}}" href="/?{{.FilterQuery}}&page={{.PrevPage}}">← Sebelumnya</a> <a class="smallbtn {{if not .HasNext}}disabled{{end}}" href="/?{{.FilterQuery}}&page={{.NextPage}}">Berikutnya →</a></div></div>
<div class="panel utility"><details><summary>Utility: import CSV manual</summary><form class="import" action="/import" method="post" enctype="multipart/form-data"><label><span>Preset</span><input name="preset" placeholder="kost" required></label><label><span>Area</span><input name="area" placeholder="jakarta" required></label><label><span>Subarea</span><input name="subarea" placeholder="Jakarta Selatan"></label><label><span>CSV hasil collector</span><input type="file" name="file" accept=".csv,text/csv" required></label><button class="primary">Import ke database</button></form></details></div>
</div></body></html>`

const detailHTML = `<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>
:root{font-family:Inter,system-ui,-apple-system,sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0}.wrap{max-width:1050px;margin:auto;padding:24px}.back{display:inline-block;margin-bottom:16px;color:#172033}.card{background:#fff;border:1px solid #e8edf5;border-radius:16px;padding:22px;box-shadow:0 4px 18px rgba(18,38,63,.04)}.gallery{display:grid;grid-template-columns:2fr 1fr 1fr;grid-auto-rows:105px;gap:8px;margin-bottom:22px}.gallery img{width:100%;height:100%;object-fit:cover;border-radius:11px;background:#edf1f6}.gallery a:first-child{grid-row:span 2}.ph{height:210px;border-radius:14px;background:#edf1f6;display:flex;align-items:center;justify-content:center;color:#8b95a5;margin-bottom:22px}.muted{color:#718096;font-size:13px}h1{margin:0 0 6px}.rating{font-size:18px;margin:12px 0}.actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:16px}.btn{padding:9px 13px;border:1px solid #d9e1ec;border-radius:9px;text-decoration:none;color:#172033}.primary{background:#172033;color:#fff;border-color:#172033}.grid{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:22px}.field{padding:13px;border:1px solid #edf1f6;border-radius:10px}.field b{display:block;font-size:12px;color:#667085;margin-bottom:5px}.field span{word-break:break-word}.reviewbox{margin-top:22px;padding:18px;border:1px solid #e3e8f0;border-radius:12px;background:#fafbfd}.reviewbox h2{font-size:16px;margin:0 0 12px}.reviewgrid{display:grid;grid-template-columns:220px 1fr auto;gap:10px;align-items:start}.reviewgrid select,.reviewgrid textarea,.reviewgrid button{border:1px solid #d9e1ec;border-radius:9px;padding:9px 10px;background:#fff}.reviewgrid textarea{min-height:78px;resize:vertical}.badge{display:inline-flex;padding:5px 9px;border-radius:999px;font-size:11px;font-weight:700;background:#eef2f7;color:#667085}.badge.valid{background:#eaf8ef;color:#217a3c}.badge.needs_review{background:#fff5d9;color:#8a6400}.badge.exclude{background:#fdecec;color:#a92d2d}@media(max-width:700px){.gallery{grid-template-columns:1fr 1fr;grid-auto-rows:130px}.gallery a:first-child{grid-column:span 2;grid-row:auto}.grid,.reviewgrid{grid-template-columns:1fr}}
</style></head><body><div class="wrap"><a class="back" href="/">← Kembali ke database</a><div class="card">{{if .Images}}<div class="gallery">{{range .Images}}<a href="{{.}}" target="_blank" rel="noreferrer"><img src="{{.}}" loading="lazy" referrerpolicy="no-referrer" onerror="this.closest('a').style.display='none'"></a>{{end}}</div>{{else}}<div class="ph">Tidak ada foto</div>{{end}}<div><h1>{{.Title}}</h1><div class="muted">{{.Category}} · {{.Preset}} · {{.Area}} {{.Subarea}}</div><div class="rating">★ {{printf "%.1f" .Rating}} <span class="muted">({{.ReviewCount}} review)</span></div><div>{{.Address}}</div><div class="actions">{{if .Link}}<a class="btn primary" href="{{.Link}}" target="_blank" rel="noreferrer">Buka Google Maps</a>{{end}}{{if .Phone}}<a class="btn" href="https://wa.me/{{wa .Phone}}" target="_blank" rel="noreferrer">WhatsApp</a>{{end}}{{if .Website}}<a class="btn" href="{{.Website}}" target="_blank" rel="noreferrer">Website</a>{{end}}</div></div>
<div class="reviewbox"><h2>Quality Control <span class="badge {{.Review.Status}}">{{reviewLabel .Review.Status}}</span></h2><form class="reviewgrid" method="post" action="/lead/{{.ID}}/review"><select name="status"><option value="unreviewed" {{if eq .Review.Status "unreviewed"}}selected{{end}}>Unreviewed</option><option value="valid" {{if eq .Review.Status "valid"}}selected{{end}}>Valid Lead</option><option value="needs_review" {{if eq .Review.Status "needs_review"}}selected{{end}}>Needs Review</option><option value="exclude" {{if eq .Review.Status "exclude"}}selected{{end}}>Exclude</option></select><textarea name="note" maxlength="2000" placeholder="Catatan internal...">{{.Review.Note}}</textarea><button class="btn primary">Simpan Review</button></form><div class="muted" style="margin-top:8px">Terakhir direview: {{shortTime .Review.ReviewedAt}}</div></div>
<div class="grid"><div class="field"><b>Telepon</b><span>{{if .Phone}}{{.Phone}}{{else}}-{{end}}</span></div><div class="field"><b>Koordinat</b><span>{{.Latitude}}, {{.Longitude}}</span></div><div class="field"><b>Place ID</b><span>{{.PlaceID}}</span></div><div class="field"><b>Data ID</b><span>{{.DataID}}</span></div><div class="field"><b>Pertama ditemukan</b><span>{{shortTime .FirstSeen}}</span></div><div class="field"><b>Terakhir dicek</b><span>{{shortTime .LastChecked}}</span></div></div></div></div></body></html>`
