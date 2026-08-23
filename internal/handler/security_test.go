package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSameOrigin(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		headers  map[string]string
		wantCode int
	}{
		{"GET is always allowed", "GET", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
		{"same-origin POST", "POST", map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"user-initiated navigation", "POST", map[string]string{"Sec-Fetch-Site": "none"}, http.StatusOK},
		{"cross-site POST", "POST", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"same-site POST", "POST", map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"cross-site DELETE", "DELETE", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"matching Origin fallback", "PUT", map[string]string{"Origin": "http://example.test"}, http.StatusOK},
		{"foreign Origin fallback", "PUT", map[string]string{"Origin": "http://evil.test"}, http.StatusForbidden},
		{"no browser headers at all", "POST", nil, http.StatusOK},
	}

	h := RequireSameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "http://example.test/subscriptions", nil)
			r.Host = "example.test"
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}
