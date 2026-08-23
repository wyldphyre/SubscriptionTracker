package importer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/craigr/subscriptiontracker/internal/id"
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
		switch currencyStr {
		case "USD":
			currency = model.CurrencyUSD
		case "EUR":
			currency = model.CurrencyEUR
		}

		// Cycle
		cycleStr := get(row, "Cycle")
		cycle, cycleOK := parseCycle(cycleStr)
		if !cycleOK {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("row %d (%s): unknown billing cycle %q, using monthly", lineNum, name, cycleStr))
		}

		// Notes
		notes := get(row, "Notes")

		// Status: cancelled if the notes mention "cancelled" (a zero cost alone
		// is not enough — some active subscriptions are free).
		status := model.StatusActive
		if strings.Contains(strings.ToLower(notes), "cancelled") {
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
			ID:          id.NewUUID(),
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

// parseCycle maps a spreadsheet cycle label to a billing cycle. The bool
// reports whether the label was recognised; unrecognised labels fall back to
// monthly and are reported to the user as warnings rather than applied silently.
//
// "Bi-annual"/"biannual" mean twice a year here, matching common usage; every
// two years is spelled "biennial" or "every 2 years".
func parseCycle(s string) (model.BillingCycle, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return model.CycleMonthly, true
	case "weekly", "every week":
		return model.CycleWeekly, true
	case "monthly", "every month":
		return model.CycleMonthly, true
	case "quarterly", "every 3 months", "every quarter":
		return model.CycleQuarterly, true
	case "six monthly", "sixmonthly", "6 monthly", "every 6 months",
		"half-yearly", "half yearly", "semi-annual", "semiannual",
		"bi-annual", "biannual":
		return model.CycleSixMonthly, true
	case "yearly", "annual", "annually", "every year":
		return model.CycleYearly, true
	case "every 2 years", "every2years", "2 yearly", "biennial", "bi-ennial":
		return model.CycleEvery2Year, true
	default:
		return model.CycleMonthly, false
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
		"01-02-06",   // MM-DD-YY  (most common from this spreadsheet)
		"2-Jan",      // D-MMM     (e.g. "3-Jul" = July 3 current year)
		"2-Jan-06",   // D-MMM-YY
		"2-Jan-2006", // D-MMM-YYYY
		"2006-01-02", // ISO
		"02/01/2006", // DD/MM/YYYY
		"01/02/2006", // MM/DD/YYYY
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
