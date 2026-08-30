package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

type app struct {
	store         *leadstore.Store
	user          string
	pass          string
	dbPath        string
	collectorPath string
	enginePath    string
	configDir     string
	collectMu     sync.RWMutex
	collect       collectState
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
	if err := store.EnsureEnrichmentSchema(context.Background()); err != nil {
		log.Fatalf("open enrichment database: %v", err)
	}

	a := &app{
		store:         store,
		user:          strings.TrimSpace(os.Getenv("LEADS_USER")),
		pass:          os.Getenv("LEADS_PASS"),
		dbPath:        *dbPath,
		collectorPath: *collectorPath,
		enginePath:    *enginePath,
		configDir:     *configDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboardV2)
	mux.HandleFunc("GET /lead/{id}", a.handleLeadDetailV2)
	mux.HandleFunc("POST /lead/{id}/review", a.handleLeadReviewV2)
	mux.HandleFunc("POST /lead/{id}/enrichment", a.handleLeadEnrichment)
	mux.HandleFunc("GET /queue", a.handleQueue)
	mux.HandleFunc("POST /queue/{id}", a.handleQueueSave)
	mux.HandleFunc("GET /api/leads", a.handleAPILeadsV2)
	mux.HandleFunc("GET /export.csv", a.handleExportCSVV2)
	mux.HandleFunc("GET /export/customer.csv", a.handleCustomerExportCSV)
	mux.HandleFunc("GET /export/customer.pdf", a.handleCustomerExportPDF)
	mux.HandleFunc("GET /export/customer.xlsx", a.handleCustomerExportXLSXV12)
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
