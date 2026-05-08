package currency

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	frankfurterURL  = "https://api.frankfurter.app/latest?from=USD&to=AUD,EUR"
	fallbackUSDRate = 1.60 // used only when no cached rate exists and fetch fails
	fallbackEURRate = 1.75
)

// Rates holds the current exchange rates used for currency conversion.
type Rates struct {
	USDToAUD  float64
	EURToAUD  float64
	FetchedAt time.Time
}

// Converter fetches and caches exchange rates.
type Converter struct {
	mu         sync.RWMutex
	rates      Rates
	ttl        time.Duration
	httpClient *http.Client
}

type frankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// New creates a Converter with the given TTL for the cached rates.
func New(ttl time.Duration) *Converter {
	return &Converter{
		ttl: ttl,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetRates returns the current exchange rates.
// Uses cached values if within TTL; fetches from Frankfurter API otherwise.
func (c *Converter) GetRates() Rates {
	c.mu.RLock()
	if c.rates.USDToAUD != 0 && time.Since(c.rates.FetchedAt) < c.ttl {
		rates := c.rates
		c.mu.RUnlock()
		return rates
	}
	c.mu.RUnlock()

	rates, err := c.fetchRates()
	if err != nil {
		log.Printf("currency: fetch failed: %v", err)
		c.mu.RLock()
		cached := c.rates
		c.mu.RUnlock()
		if cached.USDToAUD != 0 {
			log.Printf("currency: using stale cached rates USD=%.4f EUR=%.4f", cached.USDToAUD, cached.EURToAUD)
			return cached
		}
		log.Printf("currency: no cached rates available, using fallback")
		return Rates{USDToAUD: fallbackUSDRate, EURToAUD: fallbackEURRate}
	}

	c.mu.Lock()
	c.rates = rates
	c.mu.Unlock()

	return rates
}

// USDToAUD returns the current USD→AUD exchange rate.
func (c *Converter) USDToAUD() float64 {
	return c.GetRates().USDToAUD
}

// RateInfo returns the current rates and when they were fetched.
func (c *Converter) RateInfo() (Rates, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rates, c.rates.FetchedAt
}

// Refresh forces a new fetch regardless of TTL.
func (c *Converter) Refresh() error {
	rates, err := c.fetchRates()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.rates = rates
	c.mu.Unlock()
	return nil
}

func (c *Converter) fetchRates() (Rates, error) {
	resp, err := c.httpClient.Get(frankfurterURL)
	if err != nil {
		return Rates{}, fmt.Errorf("GET %s: %w", frankfurterURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Rates{}, fmt.Errorf("unexpected status %d from Frankfurter", resp.StatusCode)
	}

	var result frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Rates{}, fmt.Errorf("decoding Frankfurter response: %w", err)
	}

	usdToAUD, ok := result.Rates["AUD"]
	if !ok || usdToAUD == 0 {
		return Rates{}, fmt.Errorf("AUD rate not found in Frankfurter response")
	}

	usdToEUR, ok := result.Rates["EUR"]
	if !ok || usdToEUR == 0 {
		return Rates{}, fmt.Errorf("EUR rate not found in Frankfurter response")
	}

	// EUR→AUD derived from USD as base: divide AUD rate by EUR rate
	eurToAUD := usdToAUD / usdToEUR

	return Rates{
		USDToAUD:  usdToAUD,
		EURToAUD:  eurToAUD,
		FetchedAt: time.Now(),
	}, nil
}
