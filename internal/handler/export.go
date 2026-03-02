package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ExportCSV handles GET /export/csv
func (h *Handlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	subs := h.store.GetAll()
	rate := h.converter.USDToAUD()

	filename := fmt.Sprintf("subscriptions-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"ID", "Name", "Description", "Start Date",
		"Cost", "Currency", "Cost AUD", "Cycle",
		"Tags", "Notes", "Status",
		"Cost Per Month AUD", "Cost Per Year AUD",
	})

	for _, sub := range subs {
		startStr := ""
		if !sub.StartDate.IsZero() {
			startStr = sub.StartDate.Format("2006-01-02")
		}
		vm := toViewModel(sub, rate)
		_ = cw.Write([]string{
			sub.ID,
			sub.Name,
			sub.Description,
			startStr,
			fmt.Sprintf("%.2f", sub.Cost),
			string(sub.Currency),
			fmt.Sprintf("%.2f", vm.CostAUD),
			string(sub.Cycle),
			strings.Join(sub.Tags, "|"),
			sub.Notes,
			string(sub.Status),
			fmt.Sprintf("%.2f", vm.CostPerMonth),
			fmt.Sprintf("%.2f", vm.CostPerYear),
		})
	}
	cw.Flush()
}

// ExportJSON handles GET /export/json
func (h *Handlers) ExportJSON(w http.ResponseWriter, r *http.Request) {
	subs := h.store.GetAll()
	tags := h.store.ListTags()

	filename := fmt.Sprintf("subscriptions-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	export := struct {
		ExportedAt    string      `json:"exported_at"`
		Tags          []string    `json:"tags"`
		Subscriptions interface{} `json:"subscriptions"`
	}{
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Tags:          tags,
		Subscriptions: subs,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(export)
}
