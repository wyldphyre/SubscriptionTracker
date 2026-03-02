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
	frankfurterURL = "https://api.frankfurter.app/latest?from=USD&to=AUD"
	fallbackRate   = 1.60 // used only when no cached rate exists and fetch fails
)

// Converter fetches and caches the USD→AUD exchange rate.
type Converter struct {
	mu         sync.RWMutex
	rate       float64
	fetchedAt  time.Time
	ttl        time.Duration
	httpClient *http.Client
}

type frankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// New creates a Converter with the given TTL for the cached rate.
func New(ttl time.Duration) *Converter {
	return &Converter{
		ttl: ttl,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// USDToAUD returns the current USD→AUD exchange rate.
// Uses cached value if within TTL; fetches from Frankfurter API otherwise.
func (c *Converter) USDToAUD() float64 {
	c.mu.RLock()
	if c.rate != 0 && time.Since(c.fetchedAt) < c.ttl {
		rate := c.rate
		c.mu.RUnlock()
		return rate
	}
	c.mu.RUnlock()

	rate, err := c.fetchRate()
	if err != nil {
		log.Printf("currency: fetch failed: %v", err)
		c.mu.RLock()
		cached := c.rate
		c.mu.RUnlock()
		if cached != 0 {
			log.Printf("currency: using stale cached rate %.4f", cached)
			return cached
		}
		log.Printf("currency: no cached rate available, using fallback %.4f", fallbackRate)
		return fallbackRate
	}

	c.mu.Lock()
	c.rate = rate
	c.fetchedAt = time.Now()
	c.mu.Unlock()

	return rate
}

// RateInfo returns the current rate and when it was fetched.
func (c *Converter) RateInfo() (rate float64, fetchedAt time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rate, c.fetchedAt
}

// Refresh forces a new fetch regardless of TTL.
func (c *Converter) Refresh() error {
	rate, err := c.fetchRate()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.rate = rate
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return nil
}

func (c *Converter) fetchRate() (float64, error) {
	resp, err := c.httpClient.Get(frankfurterURL)
	if err != nil {
		return 0, fmt.Errorf("GET %s: %w", frankfurterURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d from Frankfurter", resp.StatusCode)
	}

	var result frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding Frankfurter response: %w", err)
	}

	rate, ok := result.Rates["AUD"]
	if !ok || rate == 0 {
		return 0, fmt.Errorf("AUD rate not found in Frankfurter response")
	}
	return rate, nil
}
