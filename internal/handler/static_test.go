package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticHandlerServesCacheValidators(t *testing.T) {
	fsys := fstest.MapFS{"app.css": &fstest.MapFile{Data: []byte("body{}")}}
	h, err := StaticHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}
	srv := http.StripPrefix("/static/", h)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest("GET", "/static/app.css", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag: every page load would re-download the asset")
	}
	if w.Header().Get("Cache-Control") == "" {
		t.Error("no Cache-Control header")
	}

	// A conditional request with the same validator must be a cheap 304.
	r := httptest.NewRequest("GET", "/static/app.css", nil)
	r.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusNotModified {
		t.Errorf("conditional request status = %d, want 304", w.Code)
	}
}

func TestTagDOMIDIsSelectorSafe(t *testing.T) {
	// "home theatre" used to render as "#tag-row-home%20theatre", which makes
	// querySelector throw and silently broke rename/delete for the tag.
	for _, tag := range []string{"home theatre", "a/b", "café", "x#y"} {
		got := tagDOMID(tag)
		for _, r := range got {
			if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
				t.Errorf("tagDOMID(%q) = %q contains %q, which is not selector-safe", tag, got, r)
			}
		}
	}
}
