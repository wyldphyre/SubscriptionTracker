package currency

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// almostEqual reports whether two float rates are within a small tolerance.
func almostEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	return d < eps && d > -eps
}

// TestGetRatesFallback verifies that a fresh converter that has never fetched
// returns the built-in fallback rates rather than zero values.
func TestGetRatesFallback(t *testing.T) {
	c := New(time.Hour)

	got := c.GetRates()
	if !almostEqual(got.USDToAUD, fallbackUSDRate) {
		t.Errorf("USDToAUD = %v, want fallback %v", got.USDToAUD, fallbackUSDRate)
	}
	if !almostEqual(got.EURToAUD, fallbackEURRate) {
		t.Errorf("EURToAUD = %v, want fallback %v", got.EURToAUD, fallbackEURRate)
	}
}

// TestRefreshStoresRates verifies a successful fetch is parsed and cached, and
// that EUR→AUD is derived correctly from the USD-based response.
func TestRefreshStoresRates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// USD base: 1 USD = 1.5 AUD, 1 USD = 0.9 EUR → 1 EUR = 1.5/0.9 AUD
		_, _ = w.Write([]byte(`{"base":"USD","date":"2026-06-28","rates":{"AUD":1.5,"EUR":0.9}}`))
	}))
	defer srv.Close()

	c := New(time.Hour)
	c.url = srv.URL

	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	got := c.GetRates()
	if !almostEqual(got.USDToAUD, 1.5) {
		t.Errorf("USDToAUD = %v, want 1.5", got.USDToAUD)
	}
	if want := 1.5 / 0.9; !almostEqual(got.EURToAUD, want) {
		t.Errorf("EURToAUD = %v, want %v", got.EURToAUD, want)
	}
	if got.FetchedAt.IsZero() {
		t.Error("FetchedAt is zero after a successful refresh")
	}
}

// TestRefreshFailureKeepsCachedRates verifies that a failed fetch returns an
// error and does not clobber previously-cached good rates.
func TestRefreshFailureKeepsCachedRates(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"base":"USD","date":"2026-06-28","rates":{"AUD":1.5,"EUR":0.9}}`))
	}))
	defer srv.Close()

	c := New(time.Hour)
	c.url = srv.URL

	if err := c.Refresh(); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}

	fail = true
	if err := c.Refresh(); err == nil {
		t.Fatal("Refresh() error = nil, want error on upstream failure")
	}

	// Cached good rates must survive the failed refresh.
	got := c.GetRates()
	if !almostEqual(got.USDToAUD, 1.5) {
		t.Errorf("USDToAUD = %v, want cached 1.5 after failed refresh", got.USDToAUD)
	}
}

// TestRefreshMissingRate verifies a response without the AUD rate is rejected.
func TestRefreshMissingRate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"base":"USD","date":"2026-06-28","rates":{"EUR":0.9}}`))
	}))
	defer srv.Close()

	c := New(time.Hour)
	c.url = srv.URL

	if err := c.Refresh(); err == nil {
		t.Fatal("Refresh() error = nil, want error when AUD rate is missing")
	}
}
