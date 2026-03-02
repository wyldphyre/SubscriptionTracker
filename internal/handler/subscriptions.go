package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/craigr/subscriptiontracker/internal/model"
)

// NewForm handles GET /subscriptions/new
func (h *Handlers) NewForm(w http.ResponseWriter, r *http.Request) {
	vm := FormViewModel{
		ActivePage:    "subscriptions",
		Sub:           &model.Subscription{},
		AllTags:       h.store.ListTags(),
		AllCycles:     model.AllCycles,
		AllCurrencies: model.AllCurrencies,
	}
	if isHTMX(r) {
		h.render(w, r, "subscription_form_modal.html", vm)
		return
	}
	h.render(w, r, "subscription_form.html", vm)
}

// EditForm handles GET /subscriptions/{id}/edit
func (h *Handlers) EditForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sub, ok := h.store.GetByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := FormViewModel{
		ActivePage:    "subscriptions",
		Sub:           sub,
		AllTags:       h.store.ListTags(),
		AllCycles:     model.AllCycles,
		AllCurrencies: model.AllCurrencies,
	}
	if isHTMX(r) {
		h.render(w, r, "subscription_form_modal.html", vm)
		return
	}
	h.render(w, r, "subscription_form.html", vm)
}

// GetSubscription handles GET /subscriptions/{id} — returns a single row partial.
func (h *Handlers) GetSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sub, ok := h.store.GetByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	rate := h.converter.USDToAUD()
	vm := toViewModel(*sub, rate)
	h.render(w, r, "subscription_row.html", vm)
}

// CreateSubscription handles POST /subscriptions
func (h *Handlers) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	sub, errMsg := parseSubscriptionForm(r)
	if errMsg != "" {
		vm := FormViewModel{
			Sub:           sub,
			AllTags:       h.store.ListTags(),
			AllCycles:     model.AllCycles,
			AllCurrencies: model.AllCurrencies,
			Error:         errMsg,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		if isHTMX(r) {
			h.render(w, r, "subscription_form_modal.html", vm)
		} else {
			h.render(w, r, "subscription_form.html", vm)
		}
		return
	}

	if err := h.store.Create(sub); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast":"Subscription added"}`)
	redirect(w, r, "/subscriptions")
}

// UpdateSubscription handles PUT /subscriptions/{id}
func (h *Handlers) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok := h.store.GetByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sub, errMsg := parseSubscriptionForm(r)
	if errMsg != "" {
		vm := FormViewModel{
			Sub:           sub,
			AllTags:       h.store.ListTags(),
			AllCycles:     model.AllCycles,
			AllCurrencies: model.AllCurrencies,
			Error:         errMsg,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		if isHTMX(r) {
			h.render(w, r, "subscription_form_modal.html", vm)
		} else {
			h.render(w, r, "subscription_form.html", vm)
		}
		return
	}

	sub.ID = id
	sub.CreatedAt = existing.CreatedAt

	if err := h.store.Update(sub); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rate := h.converter.USDToAUD()
	vm := toViewModel(*sub, rate)
	w.Header().Set("HX-Trigger", `{"showToast":"Subscription updated"}`)
	h.render(w, r, "subscription_row.html", vm)
}

// DeleteSubscription handles DELETE /subscriptions/{id}
func (h *Handlers) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"showToast":"Subscription deleted"}`)
	w.WriteHeader(http.StatusOK)
}

// RefreshCurrency handles POST /currency/refresh
func (h *Handlers) RefreshCurrency(w http.ResponseWriter, r *http.Request) {
	if err := h.converter.Refresh(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vm := h.buildDashboardVM()
	w.Header().Set("HX-Trigger", `{"showToast":"Exchange rate refreshed"}`)
	h.render(w, r, "dashboard_summary.html", vm)
}

// parseSubscriptionForm parses and validates a subscription form POST.
func parseSubscriptionForm(r *http.Request) (*model.Subscription, string) {
	if err := r.ParseForm(); err != nil {
		return &model.Subscription{}, "invalid form data"
	}

	name := r.FormValue("name")
	if name == "" {
		return &model.Subscription{}, "Name is required"
	}

	var cost float64
	if costStr := r.FormValue("cost"); costStr != "" {
		v, err := strconv.ParseFloat(costStr, 64)
		if err != nil {
			return &model.Subscription{}, "Cost must be a number"
		}
		cost = v
	}

	currency := model.Currency(r.FormValue("currency"))
	if currency == "" {
		currency = model.CurrencyAUD
	}

	cycle := model.BillingCycle(r.FormValue("cycle"))
	if cycle == "" {
		cycle = model.CycleMonthly
	}

	status := model.Status(r.FormValue("status"))
	if status == "" {
		status = model.StatusActive
	}

	tags := parseTagsField(r.FormValue("tags"))

	var startDate time.Time
	if s := r.FormValue("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}

	sub := &model.Subscription{
		Name:        name,
		Description: r.FormValue("description"),
		StartDate:   startDate,
		Cost:        cost,
		Currency:    currency,
		Cycle:       cycle,
		Tags:        tags,
		Notes:       r.FormValue("notes"),
		Status:      status,
	}
	return sub, ""
}
