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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/internal/leadstore"
)

type app struct {
	store *leadstore.Store
	tmpl  *template.Template
	user  string
	pass  string
}

type pageData struct {
	Stats leadstore.Stats
	Leads []leadstore.Lead
	Query string
	Preset string
	Area string
	Subarea string
	HasPhone bool
	MinRating string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "dashboard listen address")
	dbPath := flag.String("db", "data/leads.db", "SQLite master database path")
	flag.Parse()

	if err := os.MkdirAll("data", 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	store, err := leadstore.Open(*dbPath)
	if err != nil {
		log.Fatalf("open lead database: %v", err)
	}
	defer store.Close()

	tmpl, err := template.New("dashboard").Parse(dashboardHTML)
	if err != nil {
		log.Fatalf("parse dashboard template: %v", err)
	}

	a := &app{
		store: store,
		tmpl: tmpl,
		user: strings.TrimSpace(os.Getenv("LEADS_USER")),
		pass: os.Getenv("LEADS_PASS"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("GET /api/leads", a.handleAPILeads)
	mux.HandleFunc("GET /export.csv", a.handleExportCSV)
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

func (a *app) filterFromRequest(r *http.Request) leadstore.Filter {
	minRating, _ := strconv.ParseFloat(r.URL.Query().Get("min_rating"), 64)
	return leadstore.Filter{
		Preset: strings.TrimSpace(r.URL.Query().Get("preset")),
		Area: strings.TrimSpace(r.URL.Query().Get("area")),
		Subarea: strings.TrimSpace(r.URL.Query().Get("subarea")),
		Query: strings.TrimSpace(r.URL.Query().Get("q")),
		HasPhone: r.URL.Query().Get("has_phone") == "1",
		MinRating: minRating,
		Limit: 1000,
	}
}

func (a *app) handleDashboard(w http.ResponseWriter, r *http.Request) {
	filter := a.filterFromRequest(r)
	leads, err := a.store.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stats, err := a.store.Stats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := pageData{
		Stats: stats,
		Leads: leads,
		Query: filter.Query,
		Preset: filter.Preset,
		Area: filter.Area,
		Subarea: filter.Subarea,
		HasPhone: filter.HasPhone,
		MinRating: r.URL.Query().Get("min_rating"),
	}
	if err := a.tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) handleAPILeads(w http.ResponseWriter, r *http.Request) {
	leads, err := a.store.List(r.Context(), a.filterFromRequest(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leads)
}

func (a *app) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	leads, err := a.store.List(r.Context(), a.filterFromRequest(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filename := "leads-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"preset","area","subarea","title","category","address","phone","website","rating","review_count","latitude","longitude","maps_url","last_checked"})
	for _, lead := range leads {
		_ = writer.Write([]string{
			lead.Preset, lead.Area, lead.Subarea, lead.Title, lead.Category, lead.Address,
			lead.Phone, lead.Website, strconv.FormatFloat(lead.Rating, 'f', 1, 64),
			strconv.Itoa(lead.ReviewCount), lead.Latitude, lead.Longitude, lead.Link, lead.LastChecked,
		})
	}
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

const dashboardHTML = `<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Lead Database</title>
<style>
:root{font-family:Inter,system-ui,-apple-system,sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0}.wrap{max-width:1500px;margin:auto;padding:24px}.top{display:flex;justify-content:space-between;gap:20px;align-items:flex-end;margin-bottom:20px}h1{margin:0;font-size:28px}.muted{color:#718096;font-size:13px}.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin:18px 0}.card,.panel{background:#fff;border:1px solid #e8edf5;border-radius:14px;box-shadow:0 4px 18px rgba(18,38,63,.04)}.card{padding:18px}.card b{display:block;font-size:26px;margin-top:5px}.panel{padding:16px;margin-bottom:16px}.filters{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr auto auto;gap:8px}input,button{min-height:40px;border:1px solid #d9e1ec;border-radius:9px;padding:8px 10px;background:#fff}button,.btn{cursor:pointer;text-decoration:none;color:#172033}.primary{background:#172033;color:#fff;border-color:#172033}.tablewrap{overflow:auto;background:#fff;border:1px solid #e8edf5;border-radius:14px}table{width:100%;border-collapse:collapse;min-width:1150px}th,td{text-align:left;padding:11px 12px;border-bottom:1px solid #eef2f7;font-size:13px;vertical-align:top}th{position:sticky;top:0;background:#fafbfd;color:#5c677d}.title{font-weight:700}.actions a{margin-right:8px}.import{display:flex;gap:8px;align-items:end;flex-wrap:wrap}.import label{font-size:12px;color:#667085}.import label span{display:block;margin-bottom:4px}@media(max-width:900px){.stats{grid-template-columns:1fr 1fr}.filters{grid-template-columns:1fr 1fr}.top{align-items:flex-start;flex-direction:column}}@media(max-width:560px){.wrap{padding:14px}.stats,.filters{grid-template-columns:1fr}}
</style>
</head><body><div class="wrap">
<div class="top"><div><h1>Lead Database</h1><div class="muted">Master data internal · filter → export → kirim ke customer</div></div><a class="btn" href="/export.csv?preset={{.Preset}}&area={{.Area}}&subarea={{.Subarea}}&q={{.Query}}&min_rating={{.MinRating}}{{if .HasPhone}}&has_phone=1{{end}}">Export hasil filter CSV</a></div>
<div class="stats"><div class="card"><span class="muted">Total leads</span><b>{{.Stats.Total}}</b></div><div class="card"><span class="muted">Dengan telepon</span><b>{{.Stats.WithPhone}}</b></div><div class="card"><span class="muted">Dengan website</span><b>{{.Stats.WithWebsite}}</b></div><div class="card"><span class="muted">Rata-rata rating</span><b>{{printf "%.1f" .Stats.AvgRating}}</b></div></div>
<div class="panel"><form class="filters" method="get"><input name="q" placeholder="Cari nama, alamat, kategori..." value="{{.Query}}"><input name="preset" placeholder="Niche/preset" value="{{.Preset}}"><input name="area" placeholder="Area" value="{{.Area}}"><input name="subarea" placeholder="Subarea" value="{{.Subarea}}"><input name="min_rating" type="number" step="0.1" min="0" max="5" placeholder="Min rating" value="{{.MinRating}}"><label><input type="checkbox" name="has_phone" value="1" {{if .HasPhone}}checked{{end}}> Ada HP</label><button class="primary">Filter</button></form></div>
<div class="panel"><form class="import" action="/import" method="post" enctype="multipart/form-data"><label><span>Preset</span><input name="preset" placeholder="kost" required></label><label><span>Area</span><input name="area" placeholder="jakarta" required></label><label><span>Subarea</span><input name="subarea" placeholder="Jakarta Selatan"></label><label><span>CSV hasil collector</span><input type="file" name="file" accept=".csv,text/csv" required></label><button class="primary">Import ke database</button></form></div>
<div class="tablewrap"><table><thead><tr><th>Lead</th><th>Lokasi</th><th>Kontak</th><th>Rating</th><th>Niche</th><th>Last checked</th><th>Aksi</th></tr></thead><tbody>{{range .Leads}}<tr><td><div class="title">{{.Title}}</div><div class="muted">{{.Category}}</div></td><td>{{.Address}}<div class="muted">{{.Area}} {{.Subarea}}</div></td><td>{{if .Phone}}{{.Phone}}{{else}}-{{end}}<div>{{if .Website}}<a href="{{.Website}}" target="_blank">Website</a>{{end}}</div></td><td>{{printf "%.1f" .Rating}} <span class="muted">({{.ReviewCount}})</span></td><td>{{.Preset}}</td><td class="muted">{{.LastChecked}}</td><td class="actions">{{if .Link}}<a href="{{.Link}}" target="_blank">Maps</a>{{end}}{{if .Phone}}<a href="https://wa.me/{{.Phone}}" target="_blank">WA</a>{{end}}</td></tr>{{else}}<tr><td colspan="7">Belum ada data. Import CSV hasil collector dulu.</td></tr>{{end}}</tbody></table></div>
</div></body></html>`
