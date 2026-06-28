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
	url        string
	httpClient *http.Client
}

type frankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// retryInterval is how soon the background loop retries after a failed fetch
// while no valid rates are cached yet.
const retryInterval = 30 * time.Second

// New creates a Converter with the given TTL for the cached rates.
func New(ttl time.Duration) *Converter {
	return &Converter{
		ttl: ttl,
		url: frankfurterURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start launches a background goroutine that keeps the cached rates fresh.
// Rates are refreshed every TTL; if no valid rate has been fetched yet, it
// retries more frequently. GetRates never blocks on the network, so request
// handling is never stalled by a slow or failing upstream API.
func (c *Converter) Start() {
	go func() {
		for {
			err := c.refreshOnce()
			c.mu.RLock()
			haveRates := c.rates.USDToAUD != 0
			c.mu.RUnlock()

			wait := c.ttl
			if err != nil && !haveRates {
				wait = retryInterval
			}
			time.Sleep(wait)
		}
	}()
}

// refreshOnce fetches rates once and stores them on success.
func (c *Converter) refreshOnce() error {
	rates, err := c.fetchRates()
	if err != nil {
		log.Printf("currency: fetch failed: %v", err)
		return err
	}
	c.mu.Lock()
	c.rates = rates
	c.mu.Unlock()
	return nil
}

// GetRates returns the most recently cached exchange rates without blocking.
// If no rates have been fetched yet, it returns the built-in fallback rates.
func (c *Converter) GetRates() Rates {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.rates.USDToAUD != 0 {
		return c.rates
	}
	return Rates{USDToAUD: fallbackUSDRate, EURToAUD: fallbackEURRate}
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
	return c.refreshOnce()
}

func (c *Converter) fetchRates() (Rates, error) {
	resp, err := c.httpClient.Get(c.url)
	if err != nil {
		return Rates{}, fmt.Errorf("GET %s: %w", c.url, err)
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
		FetchedAt: time.Now().UTC(),
	}, nil
}
