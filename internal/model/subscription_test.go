package model

import (
	"math"
	"testing"
)

func TestCostPerMonthAUDByCycle(t *testing.T) {
	// 120 units per period, at each cycle, in AUD.
	tests := []struct {
		cycle       BillingCycle
		wantMonthly float64
	}{
		{CycleWeekly, 120 * 52.0 / 12.0},
		{CycleMonthly, 120},
		{CycleQuarterly, 40},
		{CycleSixMonthly, 20},
		{CycleYearly, 10},
		{CycleEvery2Year, 5},
	}
	for _, tt := range tests {
		t.Run(string(tt.cycle), func(t *testing.T) {
			s := &Subscription{Cost: 120, Currency: CurrencyAUD, Cycle: tt.cycle, Status: StatusActive}
			got := s.CostPerMonthAUD(1, 1)
			if math.Abs(got-tt.wantMonthly) > 1e-9 {
				t.Errorf("CostPerMonthAUD() = %v, want %v", got, tt.wantMonthly)
			}
			if wantYearly := tt.wantMonthly * 12; math.Abs(s.CostPerYearAUD(1, 1)-wantYearly) > 1e-9 {
				t.Errorf("CostPerYearAUD() = %v, want %v", s.CostPerYearAUD(1, 1), wantYearly)
			}
		})
	}
}

func TestAllCyclesAreValidAndLabelled(t *testing.T) {
	for _, c := range AllCycles {
		if !c.Valid() {
			t.Errorf("cycle %q is offered in the UI but fails Valid()", c)
		}
		if c.Label() == string(c) {
			t.Errorf("cycle %q has no human-readable label", c)
		}
	}
}

func TestCancelledCostsNothing(t *testing.T) {
	s := &Subscription{Cost: 99, Currency: CurrencyUSD, Cycle: CycleMonthly, Status: StatusCancelled}
	if got := s.CostPerMonthAUD(1.6, 1.75); got != 0 {
		t.Errorf("CostPerMonthAUD() = %v for a cancelled subscription, want 0", got)
	}
}

func TestCurrencyConversion(t *testing.T) {
	tests := []struct {
		currency Currency
		want     float64
	}{
		{CurrencyAUD, 100},
		{CurrencyUSD, 160},
		{CurrencyEUR, 175},
	}
	for _, tt := range tests {
		s := &Subscription{Cost: 100, Currency: tt.currency, Cycle: CycleMonthly, Status: StatusActive}
		if got := s.CostAUD(1.6, 1.75); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("CostAUD(%s) = %v, want %v", tt.currency, got, tt.want)
		}
	}
}
