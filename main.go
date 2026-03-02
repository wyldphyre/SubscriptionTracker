package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/craigr/subscriptiontracker/internal/currency"
	"github.com/craigr/subscriptiontracker/internal/handler"
	"github.com/craigr/subscriptiontracker/internal/store"
)

//go:embed web/static
var staticFiles embed.FS

//go:embed web/templates
var templateFiles embed.FS

type config struct {
	port        string
	dataFile    string
	currencyTTL time.Duration
}

func loadConfig() config {
	ttlMin := 360
	if s := os.Getenv("CURRENCY_TTL_MINUTES"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			ttlMin = v
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "./subscriptions.json"
	}
	return config{
		port:        port,
		dataFile:    dataFile,
		currencyTTL: time.Duration(ttlMin) * time.Minute,
	}
}

func main() {
	cfg := loadConfig()

	// Initialise store
	st, err := store.New(cfg.dataFile)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	// Initialise currency converter
	conv := currency.New(cfg.currencyTTL)

	// Parse all templates from embedded FS
	tmpl, err := template.New("").
		Funcs(handler.FuncMap).
		ParseFS(templateFiles,
			"web/templates/layout.html",
			"web/templates/dashboard.html",
			"web/templates/subscriptions.html",
			"web/templates/subscription_form.html",
			"web/templates/partials/*.html",
		)
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	h := handler.New(st, conv, tmpl)

	mux := http.NewServeMux()

	// Static assets — strip the embedded path prefix
	staticSub, err := fs.Sub(staticFiles, "web/static")
	if err != nil {
		log.Fatalf("static files: %v", err)
	}
	mux.Handle("GET /static/",
		http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))),
	)

	// Full pages
	mux.HandleFunc("GET /{$}", h.Dashboard)
	mux.HandleFunc("GET /subscriptions", h.ListPage)
	mux.HandleFunc("GET /subscriptions/search", h.SearchSubscriptions)
	mux.HandleFunc("GET /subscriptions/new", h.NewForm)
	mux.HandleFunc("GET /import", h.ImportModal)

	// Subscription CRUD
	mux.HandleFunc("POST /subscriptions", h.CreateSubscription)
	mux.HandleFunc("GET /subscriptions/{id}", h.GetSubscription)
	mux.HandleFunc("GET /subscriptions/{id}/edit", h.EditForm)
	mux.HandleFunc("PUT /subscriptions/{id}", h.UpdateSubscription)
	mux.HandleFunc("DELETE /subscriptions/{id}", h.DeleteSubscription)

	// Tags
	mux.HandleFunc("GET /tags", h.ListTags)

	// Currency
	mux.HandleFunc("POST /currency/refresh", h.RefreshCurrency)

	// Import / Export
	mux.HandleFunc("POST /import/xlsx", h.ImportXLSX)
	mux.HandleFunc("GET /export/csv", h.ExportCSV)
	mux.HandleFunc("GET /export/json", h.ExportJSON)

	addr := fmt.Sprintf(":%s", cfg.port)
	log.Printf("Subscription Tracker listening on http://localhost%s", addr)
	log.Printf("Data file: %s", cfg.dataFile)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
