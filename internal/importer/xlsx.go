package importer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/craigr/subscriptiontracker/internal/model"
	"github.com/xuri/excelize/v2"
)

// Result holds the outcome of an xlsx import.
type Result struct {
	Subscriptions []model.Subscription
	Warnings      []string
	Count         int
}

// ImportXLSX parses an xlsx file and returns subscriptions mapped to the new schema.
// It maps columns by header name, so column order doesn't matter.
func ImportXLSX(path string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening xlsx: %w", err)
	}
	defer f.Close()

	// Use the first sheet
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("reading sheet %q: %w", sheetName, err)
	}
	if len(rows) < 2 {
		return &Result{}, nil
	}

	// Build column index map from header row
	colIndex := map[string]int{}
	for i, h := range rows[0] {
		colIndex[strings.TrimSpace(h)] = i
	}
	if _, ok := colIndex["Name"]; !ok {
		return nil, fmt.Errorf("sheet %q is missing a 'Name' column — is this the correct file? Headers found: %v", sheetName, rows[0])
	}

	// Helper to get a cell value by column name
	get := func(row []string, colName string) string {
		idx, ok := colIndex[colName]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	result := &Result{}

	for rowNum, row := range rows[1:] {
		// Skip empty rows and the totals row
		name := get(row, "Name")
		if name == "" {
			continue
		}

		lineNum := rowNum + 2 // 1-based, account for header

		// Cost
		costStr := get(row, "Cost")
		var cost float64
		if costStr != "" {
			// excelize evaluates formula cells to their numeric string
			if v, err := strconv.ParseFloat(costStr, 64); err == nil {
				cost = v
			} else {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("row %d (%s): invalid cost %q, using 0", lineNum, name, costStr))
			}
		}

		// Currency
		currencyStr := strings.ToUpper(get(row, "Cost Curency")) // note: misspelled in spreadsheet
		if currencyStr == "" {
			currencyStr = strings.ToUpper(get(row, "Cost Currency"))
		}
		currency := model.CurrencyAUD
		if currencyStr == "USD" {
			currency = model.CurrencyUSD
		}

		// Cycle
		cycle := parseCycle(get(row, "Cycle"))

		// Notes
		notes := get(row, "Notes")

		// Status: cancelled if cost == 0 AND notes mentions "cancelled"
		status := model.StatusActive
		if cost == 0 && strings.Contains(strings.ToLower(notes), "cancelled") {
			status = model.StatusCancelled
		}

		// Category → Tags (single tag, normalised)
		category := get(row, "Category")
		var tags []string
		if category != "" {
			tags = []string{normaliseTag(category)}
		}

		// Start Date
		var startDate time.Time
		dateStr := get(row, "Start Date")
		if dateStr != "" {
			if t, err := parseDate(dateStr); err == nil {
				startDate = t
			} else {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("row %d (%s): could not parse date %q", lineNum, name, dateStr))
			}
		}

		sub := model.Subscription{
			ID:          newUUID(),
			Name:        name,
			Description: get(row, "Description"),
			StartDate:   startDate,
			Cost:        cost,
			Currency:    currency,
			Cycle:       cycle,
			Tags:        tags,
			Notes:       notes,
			Status:      status,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}

		result.Subscriptions = append(result.Subscriptions, sub)
		result.Count++
	}

	if result.Subscriptions == nil {
		result.Subscriptions = []model.Subscription{}
	}

	return result, nil
}

func parseCycle(s string) model.BillingCycle {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "monthly":
		return model.CycleMonthly
	case "yearly", "annual", "annually":
		return model.CycleYearly
	case "every 2 years", "every2years", "bi-annual", "biannual":
		return model.CycleEvery2Year
	default:
		if s != "" {
			log.Printf("importer: unknown cycle %q, defaulting to monthly", s)
		}
		return model.CycleMonthly
	}
}

func normaliseTag(category string) string {
	// "Entertainment - Podcast" → "entertainment-podcast"
	// "Productivity" → "productivity"
	tag := strings.ToLower(strings.TrimSpace(category))
	tag = strings.ReplaceAll(tag, " - ", "-")
	tag = strings.ReplaceAll(tag, " ", "-")
	return tag
}

func parseDate(s string) (time.Time, error) {
	// Excel via excelize often returns dates as "MM-DD-YY" (e.g. "10-20-18")
	// or "D-MMM" (e.g. "3-Jul"). We try the most common formats first.
	formats := []string{
		"01-02-06",          // MM-DD-YY  (most common from this spreadsheet)
		"2-Jan",             // D-MMM     (e.g. "3-Jul" = July 3 current year)
		"2-Jan-06",          // D-MMM-YY
		"2-Jan-2006",        // D-MMM-YYYY
		"2006-01-02",        // ISO
		"02/01/2006",        // DD/MM/YYYY
		"01/02/2006",        // MM/DD/YYYY
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err != nil {
			continue
		}
		// "2-Jan" has no year; time.Parse gives year 0 — use current year
		if t.Year() == 0 {
			t = t.AddDate(time.Now().Year(), 0, 0)
		}
		// Reject clearly bogus dates (e.g. Excel serial number artefacts like 01-08-00)
		if t.Year() < 1990 || t.Year() > 2100 {
			continue
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised date format: %q", s)
}

// newUUID generates a random UUID v4 string.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
