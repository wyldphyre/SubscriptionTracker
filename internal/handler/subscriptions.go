package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/craigr/subscriptiontracker/internal/model"
	"github.com/craigr/subscriptiontracker/internal/store"
)

// NewForm handles GET /subscriptions/new
func (h *Handlers) NewForm(w http.ResponseWriter, r *http.Request) {
	vm := FormViewModel{
		ActivePage:    "subscriptions",
		Sub:           &model.Subscription{Status: model.StatusActive},
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

// CreateSubscription handles POST /subscriptions
func (h *Handlers) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	sub, errMsg := parseSubscriptionForm(r)
	if errMsg != "" {
		vm := FormViewModel{
			ActivePage:    "subscriptions",
			Sub:           sub,
			AllTags:       h.store.ListTags(),
			AllCycles:     model.AllCycles,
			AllCurrencies: model.AllCurrencies,
			Error:         errMsg,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		if isHTMX(r) {
			h.render(w, r, "subscription_form_fields", vm)
		} else {
			h.render(w, r, "subscription_form.html", vm)
		}
		return
	}

	if err := h.store.Create(sub); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notifyChanged(w, "Subscription added")
	redirect(w, r, "/subscriptions")
}

// UpdateSubscription handles PUT /subscriptions/{id}
func (h *Handlers) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sub, errMsg := parseSubscriptionForm(r)
	if errMsg != "" {
		vm := FormViewModel{
			ActivePage:    "subscriptions",
			Sub:           sub,
			AllTags:       h.store.ListTags(),
			AllCycles:     model.AllCycles,
			AllCurrencies: model.AllCurrencies,
			Error:         errMsg,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		if isHTMX(r) {
			h.render(w, r, "subscription_form_fields", vm)
		} else {
			h.render(w, r, "subscription_form.html", vm)
		}
		return
	}

	sub.ID = id

	// Update preserves CreatedAt internally and returns ErrNotFound if the
	// record no longer exists, so there is no read-modify-write race here.
	if err := h.store.Update(sub); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notifyChanged(w, "Subscription updated")
	w.WriteHeader(http.StatusNoContent)
}

// SetStatus handles POST /subscriptions/{id}/status — flips a subscription
// between active and cancelled.
//
// This replaces the old approach of having the Cancel/Reactivate buttons post
// the whole record back as JSON in hx-vals: that JSON was built inside an HTML
// attribute, so any name containing a quote, or any multi-line note, produced
// invalid JSON and the button did nothing at all.
func (h *Handlers) SetStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	status := model.Status(r.FormValue("status"))
	if !status.Valid() {
		http.Error(w, "Unknown status", http.StatusBadRequest)
		return
	}

	sub, ok := h.store.GetByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if sub.Status == status {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sub.Status = status

	if err := h.store.Update(sub); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msg := "Subscription cancelled"
	if status == model.StatusActive {
		msg = "Subscription reactivated"
	}
	notifyChanged(w, msg)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteSubscription handles DELETE /subscriptions/{id}
func (h *Handlers) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	notifyChanged(w, "Subscription deleted")
	w.WriteHeader(http.StatusNoContent)
}

// RefreshCurrency handles POST /currency/refresh
func (h *Handlers) RefreshCurrency(w http.ResponseWriter, r *http.Request) {
	if err := h.converter.Refresh(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vm := h.buildDashboardVM()
	hxTrigger(w, map[string]any{"showToast": "Exchange rate refreshed"})
	h.render(w, r, "dashboard_summary.html", vm)
}

// parseSubscriptionForm parses and validates a subscription form POST.
func parseSubscriptionForm(r *http.Request) (*model.Subscription, string) {
	if err := r.ParseForm(); err != nil {
		return &model.Subscription{}, "invalid form data"
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		return &model.Subscription{}, "Name is required"
	}

	var cost float64
	if costStr := r.FormValue("cost"); costStr != "" {
		v, err := strconv.ParseFloat(costStr, 64)
		if err != nil {
			return &model.Subscription{}, "Cost must be a number"
		}
		if v < 0 {
			return &model.Subscription{}, "Cost cannot be negative"
		}
		cost = v
	}

	currency := model.Currency(r.FormValue("currency"))
	if currency == "" {
		currency = model.CurrencyAUD
	} else if !currency.Valid() {
		return &model.Subscription{}, "Unknown currency"
	}

	cycle := model.BillingCycle(r.FormValue("cycle"))
	if cycle == "" {
		cycle = model.CycleMonthly
	} else if !cycle.Valid() {
		return &model.Subscription{}, "Unknown billing cycle"
	}

	status := model.Status(r.FormValue("status"))
	if status == "" {
		status = model.StatusActive
	} else if !status.Valid() {
		return &model.Subscription{}, "Unknown status"
	}

	tags := parseTagsField(r.FormValue("tags"))

	var startDate time.Time
	if s := r.FormValue("start_date"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return &model.Subscription{}, "Start date must be in YYYY-MM-DD format"
		}
		startDate = t
	}

	sub := &model.Subscription{
		Name:        name,
		Description: strings.TrimSpace(r.FormValue("description")),
		StartDate:   startDate,
		Cost:        cost,
		Currency:    currency,
		Cycle:       cycle,
		Tags:        tags,
		Notes:       strings.TrimSpace(r.FormValue("notes")),
		Status:      status,
	}
	return sub, ""
}
