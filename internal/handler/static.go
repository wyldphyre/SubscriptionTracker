package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// staticCacheControl is deliberately short with revalidation: the ETag makes
// repeat loads a cheap 304, while a new build is still picked up promptly.
const staticCacheControl = "public, max-age=3600, must-revalidate"

// StaticHandler serves embedded assets with cache validators.
//
// embed.FS reports a zero modification time, so http.FileServer emits neither
// Last-Modified nor ETag and the browser re-downloads every asset on every page
// load — around 135 KB of htmx and Pico CSS each time. Hashing the contents at
// startup gives a stable validator, and http.ServeContent turns a matching
// If-None-Match into a 304 on our behalf.
func StaticHandler(fsys fs.FS) (http.Handler, error) {
	etags := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		etags[p] = `"` + hex.EncodeToString(sum[:16]) + `"`
		return nil
	})
	if err != nil {
		return nil, err
	}

	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// StripPrefix leaves the path without a leading slash; normalise so the
		// lookup matches the keys collected from the FS walk.
		key := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if tag, ok := etags[key]; ok {
			w.Header().Set("ETag", tag)
			w.Header().Set("Cache-Control", staticCacheControl)
		}
		files.ServeHTTP(w, r)
	}), nil
}
