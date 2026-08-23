package model

import "time"

type BillingCycle string

const (
	CycleWeekly     BillingCycle = "weekly"
	CycleMonthly    BillingCycle = "monthly"
	CycleQuarterly  BillingCycle = "quarterly"
	CycleSixMonthly BillingCycle = "sixmonthly"
	CycleYearly     BillingCycle = "yearly"
	CycleEvery2Year BillingCycle = "every2years"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCancelled Status = "cancelled"
)

type Currency string

const (
	CurrencyAUD Currency = "AUD"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
)

// AllCycles is the ordered list of valid billing cycles for UI display.
var AllCycles = []BillingCycle{
	CycleWeekly,
	CycleMonthly,
	CycleQuarterly,
	CycleSixMonthly,
	CycleYearly,
	CycleEvery2Year,
}

// CycleLabel returns a human-readable label for the billing cycle.
func (c BillingCycle) Label() string {
	switch c {
	case CycleWeekly:
		return "Weekly"
	case CycleMonthly:
		return "Monthly"
	case CycleQuarterly:
		return "Quarterly"
	case CycleSixMonthly:
		return "Six Monthly"
	case CycleYearly:
		return "Yearly"
	case CycleEvery2Year:
		return "Every 2 Years"
	default:
		return string(c)
	}
}

// Valid reports whether the billing cycle is a known value.
func (c BillingCycle) Valid() bool {
	switch c {
	case CycleWeekly, CycleMonthly, CycleQuarterly, CycleSixMonthly, CycleYearly, CycleEvery2Year:
		return true
	default:
		return false
	}
}

// AllCurrencies is the ordered list of valid currencies for UI display.
var AllCurrencies = []Currency{CurrencyAUD, CurrencyUSD, CurrencyEUR}

// Valid reports whether the currency is a known value.
func (c Currency) Valid() bool {
	switch c {
	case CurrencyAUD, CurrencyUSD, CurrencyEUR:
		return true
	default:
		return false
	}
}

// Valid reports whether the status is a known value.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusCancelled:
		return true
	default:
		return false
	}
}

type Subscription struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	StartDate   time.Time    `json:"start_date"`
	Cost        float64      `json:"cost"`
	Currency    Currency     `json:"currency"`
	Cycle       BillingCycle `json:"cycle"`
	Tags        []string     `json:"tags"`
	Notes       string       `json:"notes"`
	Status      Status       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// CostAUD returns cost converted to AUD using the provided exchange rates.
// If the subscription is cancelled, returns 0.
func (s *Subscription) CostAUD(usdToAUD, eurToAUD float64) float64 {
	if s.Status == StatusCancelled {
		return 0
	}
	switch s.Currency {
	case CurrencyAUD:
		return s.Cost
	case CurrencyUSD:
		return s.Cost * usdToAUD
	case CurrencyEUR:
		return s.Cost * eurToAUD
	default:
		return s.Cost
	}
}

// CostPerMonthAUD returns the monthly equivalent cost in AUD.
func (s *Subscription) CostPerMonthAUD(usdToAUD, eurToAUD float64) float64 {
	base := s.CostAUD(usdToAUD, eurToAUD)
	return base / s.Cycle.months()
}

// months returns how many months one billing period covers. Unknown cycles are
// treated as monthly, matching the importer's fallback.
func (c BillingCycle) months() float64 {
	switch c {
	case CycleWeekly:
		return 12.0 / 52.0 // a week is 1/52 of a year
	case CycleMonthly:
		return 1
	case CycleQuarterly:
		return 3
	case CycleSixMonthly:
		return 6
	case CycleYearly:
		return 12
	case CycleEvery2Year:
		return 24
	default:
		return 1
	}
}

// CostPerYearAUD returns the annual equivalent cost in AUD.
func (s *Subscription) CostPerYearAUD(usdToAUD, eurToAUD float64) float64 {
	return s.CostPerMonthAUD(usdToAUD, eurToAUD) * 12
}

// Store is the top-level JSON file structure.
type Store struct {
	Version       int            `json:"version"`
	Subscriptions []Subscription `json:"subscriptions"`
	Tags          []string       `json:"tags"`
}
