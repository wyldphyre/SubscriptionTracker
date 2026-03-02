package handler

import (
	"net/http"
	"strings"
)

// Dashboard handles GET / — renders the dashboard page.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	vm := h.buildDashboardVM()
	if isHTMX(r) {
		h.render(w, r, "dashboard_summary.html", vm)
		return
	}
	h.render(w, r, "dashboard.html", vm)
}

// ListPage handles GET /subscriptions — renders the full subscription list.
func (h *Handlers) ListPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	activeTags := parseTags(q.Get("tag"))
	showCancelled := q.Get("cancelled") == "1"
	query := q.Get("q")

	vm := h.buildListVM(activeTags, showCancelled, query)
	if isHTMX(r) {
		h.render(w, r, "subscription_list.html", vm)
		return
	}
	h.render(w, r, "subscriptions.html", vm)
}

// SearchSubscriptions handles GET /subscriptions/search — returns the list tbody partial.
// Used by HTMX search/filter interactions.
func (h *Handlers) SearchSubscriptions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	activeTags := parseTags(q.Get("tag"))
	showCancelled := q.Get("cancelled") == "1"
	query := q.Get("q")

	vm := h.buildListVM(activeTags, showCancelled, query)
	h.render(w, r, "subscription_list.html", vm)
}

// ImportModal handles GET /import — renders the import form modal.
func (h *Handlers) ImportModal(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "import_modal.html", nil)
}

// parseTags splits a comma-separated tag query param into a slice.
func parseTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
