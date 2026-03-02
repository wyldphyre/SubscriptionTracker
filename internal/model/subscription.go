package model

import "time"

type BillingCycle string

const (
	CycleMonthly    BillingCycle = "monthly"
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
)

// AllCycles is the ordered list of valid billing cycles for UI display.
var AllCycles = []BillingCycle{CycleMonthly, CycleYearly, CycleEvery2Year}

// CycleLabel returns a human-readable label for the billing cycle.
func (c BillingCycle) Label() string {
	switch c {
	case CycleMonthly:
		return "Monthly"
	case CycleYearly:
		return "Yearly"
	case CycleEvery2Year:
		return "Every 2 Years"
	default:
		return string(c)
	}
}

// AllCurrencies is the ordered list of valid currencies for UI display.
var AllCurrencies = []Currency{CurrencyAUD, CurrencyUSD}

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

// CostAUD returns cost converted to AUD using the provided USD→AUD rate.
// If the subscription is cancelled, returns 0.
func (s *Subscription) CostAUD(usdToAUD float64) float64 {
	if s.Status == StatusCancelled {
		return 0
	}
	if s.Currency == CurrencyAUD {
		return s.Cost
	}
	return s.Cost * usdToAUD
}

// CostPerMonthAUD returns the monthly equivalent cost in AUD.
func (s *Subscription) CostPerMonthAUD(usdToAUD float64) float64 {
	base := s.CostAUD(usdToAUD)
	switch s.Cycle {
	case CycleMonthly:
		return base
	case CycleYearly:
		return base / 12
	case CycleEvery2Year:
		return base / 24
	default:
		return base
	}
}

// CostPerYearAUD returns the annual equivalent cost in AUD.
func (s *Subscription) CostPerYearAUD(usdToAUD float64) float64 {
	return s.CostPerMonthAUD(usdToAUD) * 12
}

// Store is the top-level JSON file structure.
type Store struct {
	Version       int            `json:"version"`
	Subscriptions []Subscription `json:"subscriptions"`
	Tags          []string       `json:"tags"`
}
