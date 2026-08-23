package importer

import (
	"testing"

	"github.com/craigr/subscriptiontracker/internal/model"
)

func TestParseCycle(t *testing.T) {
	tests := []struct {
		in     string
		want   model.BillingCycle
		wantOK bool
	}{
		// Labels used by the source spreadsheet — these previously all fell
		// through to monthly, silently mis-costing the subscription.
		{"Weekly", model.CycleWeekly, true},
		{"Monthly", model.CycleMonthly, true},
		{"Quarterly", model.CycleQuarterly, true},
		{"Six Monthly", model.CycleSixMonthly, true},
		{"Yearly", model.CycleYearly, true},
		{"Every 2 Years", model.CycleEvery2Year, true},

		{"  yearly  ", model.CycleYearly, true},
		{"annually", model.CycleYearly, true},
		{"biennial", model.CycleEvery2Year, true},
		// "Biannual" means twice a year, not every two years.
		{"bi-annual", model.CycleSixMonthly, true},
		{"semi-annual", model.CycleSixMonthly, true},
		{"", model.CycleMonthly, true},

		// Unrecognised labels still default to monthly, but say so.
		{"whenever", model.CycleMonthly, false},
	}
	for _, tt := range tests {
		got, ok := parseCycle(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("parseCycle(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestNormaliseTag(t *testing.T) {
	tests := map[string]string{
		"Entertainment - Podcast": "entertainment-podcast",
		"Productivity":            "productivity",
		"  Home  ":                "home",
	}
	for in, want := range tests {
		if got := normaliseTag(in); got != want {
			t.Errorf("normaliseTag(%q) = %q, want %q", in, got, want)
		}
	}
}
