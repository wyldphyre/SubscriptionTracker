package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/craigr/subscriptiontracker/internal/importer"
)

// ImportXLSX handles POST /import/xlsx
func (h *Handlers) ImportXLSX(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
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
	if len(result.Warnings) > 0 {
		msg += fmt.Sprintf(" (%d warnings: %s)", len(result.Warnings), strings.Join(result.Warnings, "; "))
	}

	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast":%q}`, msg))
	redirect(w, r, "/subscriptions")
}
