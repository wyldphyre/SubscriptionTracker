package handler

import (
	"net/http"
	"net/url"
)

// RequireSameOrigin blocks state-changing requests that the browser tells us
// originated on another site.
//
// The app has no login, so without this any page the user happens to have open
// could POST to it in the background — and POST /import/xlsx with replace_all
// set is a single request that erases the entire dataset.
func RequireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Safe methods. Responses carry no CORS headers, so a cross-origin
			// page cannot read them.
		default:
			if !sameOrigin(r) {
				http.Error(w, "cross-site request blocked", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin reports whether a state-changing request looks same-origin.
func sameOrigin(r *http.Request) bool {
	// Fetch metadata is the most direct signal, and every current browser sends
	// it. "none" means the user themselves initiated the navigation.
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "same-site", "cross-site":
		return false
	}

	// Fall back to Origin where Sec-Fetch-* was stripped by a proxy.
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Neither header: not a browser (curl, scripts, the backup job).
		// There is nothing to protect against here — CSRF needs a browser.
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}
