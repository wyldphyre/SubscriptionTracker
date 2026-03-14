package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/craigr/subscriptiontracker/internal/currency"
	"github.com/craigr/subscriptiontracker/internal/model"
	"github.com/craigr/subscriptiontracker/internal/store"
)

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	store     *store.JSONStore
	converter *currency.Converter
	templates *template.Template
}

func New(st *store.JSONStore, conv *currency.Converter, tmpl *template.Template) *Handlers {
	return &Handlers{store: st, converter: conv, templates: tmpl}
}

// isHTMX returns true if the request was made by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// render renders a named template to the response writer.
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}

// redirect performs an HTMX-aware redirect: uses HX-Redirect header for HTMX
// requests, standard http.Redirect for full-page requests.
func redirect(w http.ResponseWriter, r *http.Request, url string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// ---- View Models ----

// SubscriptionViewModel adds computed AUD costs and display-friendly fields.
type SubscriptionViewModel struct {
	model.Subscription
	CostAUD      float64
	CostPerMonth float64
	CostPerYear  float64
	StartDisplay string
	PctOfYearly  float64 // percentage of the current view's total yearly spend; 0 = not computed
}

func toViewModel(sub model.Subscription, rate float64) SubscriptionViewModel {
	vm := SubscriptionViewModel{
		Subscription: sub,
		CostAUD:      sub.CostAUD(rate),
		CostPerMonth: sub.CostPerMonthAUD(rate),
		CostPerYear:  sub.CostPerYearAUD(rate),
	}
	if !sub.StartDate.IsZero() {
		vm.StartDisplay = sub.StartDate.Format("Jan 2006")
	}
	return vm
}

// TagSummary holds per-tag spending totals.
type TagSummary struct {
	Tag         string
	Count       int
	MonthlyAUD  float64
	YearlyAUD   float64
	PctOfYearly float64
}

// DashboardViewModel is passed to the dashboard template.
type DashboardViewModel struct {
	ActivePage      string
	TotalMonthlyAUD float64
	TotalYearlyAUD  float64
	ActiveCount     int
	CancelledCount  int
	ByTag           []TagSummary
	TopByYearlyCost []SubscriptionViewModel
	Rate            float64
	RateFetchedAt   time.Time
	AllTags         []string
	Subscriptions   []SubscriptionViewModel
}

// ListViewModel is passed to the subscription list template.
type ListViewModel struct {
	ActivePage    string
	Subscriptions []SubscriptionViewModel
	AllTags       []string
	ActiveTags    []string
	ShowCancelled bool
	Query         string
	TotalMonthly  float64
	TotalYearly   float64
}

// FormViewModel is passed to add/edit form templates.
type FormViewModel struct {
	ActivePage    string
	Sub           *model.Subscription
	AllTags       []string
	AllCycles     []model.BillingCycle
	AllCurrencies []model.Currency
	Error         string
}

// ---- Shared helpers ----

// buildDashboardVM builds the DashboardViewModel from store data.
func (h *Handlers) buildDashboardVM() DashboardViewModel {
	rate := h.converter.USDToAUD()
	_, fetchedAt := h.converter.RateInfo()

	subs := h.store.GetAll()
	allTags := h.store.ListTags()

	var totalMonthly, totalYearly float64
	var activeCount, cancelledCount int
	tagMap := map[string]*TagSummary{}

	vms := make([]SubscriptionViewModel, 0, len(subs))
	for _, sub := range subs {
		vm := toViewModel(sub, rate)
		vms = append(vms, vm)

		if sub.Status == model.StatusCancelled {
			cancelledCount++
		} else {
			activeCount++
			totalMonthly += vm.CostPerMonth
			totalYearly += vm.CostPerYear

			for _, tag := range sub.Tags {
				if _, ok := tagMap[tag]; !ok {
					tagMap[tag] = &TagSummary{Tag: tag}
				}
				ts := tagMap[tag]
				ts.Count++
				ts.MonthlyAUD += vm.CostPerMonth
				ts.YearlyAUD += vm.CostPerYear
			}
		}
	}

	// Build sorted tag summaries with percentages
	byTag := make([]TagSummary, 0, len(tagMap))
	for _, ts := range tagMap {
		if totalYearly > 0 {
			ts.PctOfYearly = ts.YearlyAUD / totalYearly * 100
		}
		byTag = append(byTag, *ts)
	}
	sort.Slice(byTag, func(i, j int) bool {
		return byTag[i].MonthlyAUD > byTag[j].MonthlyAUD
	})

	// Top 10 active by yearly cost with percentages
	activeVMs := make([]SubscriptionViewModel, 0)
	for _, vm := range vms {
		if vm.Status == model.StatusActive {
			if totalYearly > 0 {
				vm.PctOfYearly = vm.CostPerYear / totalYearly * 100
			}
			activeVMs = append(activeVMs, vm)
		}
	}
	sort.Slice(activeVMs, func(i, j int) bool {
		return activeVMs[i].CostPerYear > activeVMs[j].CostPerYear
	})
	top10 := activeVMs
	if len(top10) > 10 {
		top10 = top10[:10]
	}

	return DashboardViewModel{
		ActivePage:      "dashboard",
		TotalMonthlyAUD: totalMonthly,
		TotalYearlyAUD:  totalYearly,
		ActiveCount:     activeCount,
		CancelledCount:  cancelledCount,
		ByTag:           byTag,
		TopByYearlyCost: top10,
		Rate:            rate,
		RateFetchedAt:   fetchedAt,
		AllTags:         allTags,
		Subscriptions:   vms,
	}
}

// buildListVM builds the ListViewModel, applying optional filters.
func (h *Handlers) buildListVM(activeTags []string, showCancelled bool, query string) ListViewModel {
	rate := h.converter.USDToAUD()
	subs := h.store.GetAll()
	allTags := h.store.ListTags()

	queryLower := strings.ToLower(strings.TrimSpace(query))

	var filtered []SubscriptionViewModel
	var totalMonthly, totalYearly float64

	for _, sub := range subs {
		// Status filter
		if !showCancelled && sub.Status == model.StatusCancelled {
			continue
		}

		// Tag filter (all selected tags must be present)
		if len(activeTags) > 0 {
			tagSet := map[string]bool{}
			for _, t := range sub.Tags {
				tagSet[t] = true
			}
			match := true
			for _, at := range activeTags {
				if !tagSet[at] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// Search filter
		if queryLower != "" {
			haystack := strings.ToLower(sub.Name + " " + sub.Description + " " + sub.Notes + " " + strings.Join(sub.Tags, " "))
			if !strings.Contains(haystack, queryLower) {
				continue
			}
		}

		vm := toViewModel(sub, rate)
		filtered = append(filtered, vm)
		if sub.Status == model.StatusActive {
			totalMonthly += vm.CostPerMonth
			totalYearly += vm.CostPerYear
		}
	}

	// Sort by name by default
	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	// Set percentage of yearly total for each row
	if totalYearly > 0 {
		for i := range filtered {
			filtered[i].PctOfYearly = filtered[i].CostPerYear / totalYearly * 100
		}
	}

	return ListViewModel{
		ActivePage:    "subscriptions",
		Subscriptions: filtered,
		AllTags:       allTags,
		ActiveTags:    activeTags,
		ShowCancelled: showCancelled,
		Query:         query,
		TotalMonthly:  totalMonthly,
		TotalYearly:   totalYearly,
	}
}

// parseTagsField parses a comma-separated tags string into a cleaned slice.
func parseTagsField(raw string) []string {
	parts := strings.Split(raw, ",")
	var tags []string
	seen := map[string]bool{}
	for _, p := range parts {
		t := strings.ToLower(strings.TrimSpace(p))
		if t != "" && !seen[t] {
			tags = append(tags, t)
			seen[t] = true
		}
	}
	return tags
}

// tagsToField joins a tag slice back to a comma-separated string for form display.
func tagsToField(tags []string) string {
	return strings.Join(tags, ", ")
}

// template helper functions registered on the template set
var FuncMap = template.FuncMap{
	"formatAUD": func(v float64) string {
		return fmt.Sprintf("$%.2f", v)
	},
	"tagsField": tagsToField,
	"cycleLabel": func(c model.BillingCycle) string {
		return c.Label()
	},
	"joinTags": func(tags []string) string {
		return strings.Join(tags, ", ")
	},
	"hasTag": func(tags []string, tag string) bool {
		for _, t := range tags {
			if t == tag {
				return true
			}
		}
		return false
	},
	"contains": func(slice []string, s string) bool {
		for _, v := range slice {
			if v == s {
				return true
			}
		}
		return false
	},
	"isActive": func(status model.Status) bool {
		return status == model.StatusActive
	},
	"dateInput": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	},
	"urlenc": url.PathEscape,
}
