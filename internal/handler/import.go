package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/craigr/subscriptiontracker/internal/importer"
)

// maxUploadBytes caps an xlsx upload. ParseMultipartForm's argument only caps
// how much is held in memory — without this the rest spools to disk unbounded.
const maxUploadBytes = 32 << 20 // 32 MiB

// maxWarningsShown limits how many import warnings go into the toast message,
// which travels in a response header and must stay a sane length.
const maxWarningsShown = 5

// ImportXLSX handles POST /import/xlsx
func (h *Handlers) ImportXLSX(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "could not parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("xlsx_file")
	if err != nil {
		http.Error(w, "xlsx_file field missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Write to a temp file (excelize needs a file path)
	tmp, err := os.CreateTemp("", "import-*.xlsx")
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.ReadFrom(file); err != nil {
		tmp.Close()
		http.Error(w, "failed to read upload", http.StatusInternalServerError)
		return
	}
	tmp.Close()

	result, err := importer.ImportXLSX(tmp.Name())
	if err != nil {
		log.Printf("import: parse error: %v", err)
		http.Error(w, fmt.Sprintf("import failed: %v", err), http.StatusBadRequest)
		return
	}
	log.Printf("import: parsed %d subscriptions, %d warnings", result.Count, len(result.Warnings))

	replaceAll := r.FormValue("replace_all") == "1" || r.FormValue("replace_all") == "true"

	// A sheet with the right headers but no usable rows parses successfully and
	// yields zero subscriptions. Replacing everything with that silently erases
	// the entire dataset, so refuse rather than guess the user meant it.
	if replaceAll && result.Count == 0 {
		http.Error(w,
			"Import cancelled: the file contained no subscription rows, and \"Replace all existing data\" "+
				"would have deleted everything. Check the file, or untick Replace all.",
			http.StatusBadRequest)
		return
	}

	if replaceAll {
		if err := h.store.ReplaceAll(result.Subscriptions); err != nil {
			log.Printf("import: ReplaceAll error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.store.AppendAll(result.Subscriptions); err != nil {
			log.Printf("import: AppendAll error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	msg := fmt.Sprintf("Imported %d subscriptions", result.Count)
	if n := len(result.Warnings); n > 0 {
		shown := result.Warnings
		if n > maxWarningsShown {
			shown = shown[:maxWarningsShown]
		}
		msg += fmt.Sprintf(" (%d warning%s: %s", n, plural(n), strings.Join(shown, "; "))
		if n > maxWarningsShown {
			msg += fmt.Sprintf("; and %d more — see the server log", n-maxWarningsShown)
		}
		msg += ")"
		for _, warn := range result.Warnings {
			log.Printf("import: warning: %s", warn)
		}
	}

	notifyChanged(w, msg)
	redirect(w, r, "/subscriptions")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
