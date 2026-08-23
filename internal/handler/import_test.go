package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/craigr/subscriptiontracker/internal/currency"
	"github.com/craigr/subscriptiontracker/internal/model"
	"github.com/craigr/subscriptiontracker/internal/store"
	"github.com/xuri/excelize/v2"
)

// headerOnlyXLSX writes a workbook with valid headers but no data rows.
func headerOnlyXLSX(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	row := []string{"Name", "Cost", "Cost Curency", "Cycle", "Category", "Notes"}
	if err := f.SetSheetRow(f.GetSheetName(0), "A1", &row); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "empty.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func importRequest(t *testing.T, xlsx []byte, replaceAll bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("xlsx_file", "book.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(xlsx); err != nil {
		t.Fatal(err)
	}
	if replaceAll {
		if err := mw.WriteField("replace_all", "1"); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()

	r := httptest.NewRequest("POST", "/import/xlsx", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

// TestReplaceAllImportRefusesEmptySheet covers the data-loss path: a workbook
// with the right headers and no rows parses fine, and replacing everything with
// it used to wipe the dataset while reporting success.
func TestReplaceAllImportRefusesEmptySheet(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Create(&model.Subscription{Name: "Netflix", Cost: 20, Tags: []string{"tv"}}); err != nil {
		t.Fatal(err)
	}
	h := New(st, currency.New(time.Hour), nil)

	w := httptest.NewRecorder()
	h.ImportXLSX(w, importRequest(t, headerOnlyXLSX(t), true))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if got := len(st.GetAll()); got != 1 {
		t.Fatalf("store holds %d subscriptions, want the existing 1 left intact", got)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Replace all")) {
		t.Errorf("response body does not explain the refusal: %q", w.Body.String())
	}
}

// An additive import of the same empty sheet is harmless and should succeed.
func TestAdditiveImportOfEmptySheetIsAllowed(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Create(&model.Subscription{Name: "Netflix"}); err != nil {
		t.Fatal(err)
	}
	h := New(st, currency.New(time.Hour), nil)

	w := httptest.NewRecorder()
	h.ImportXLSX(w, importRequest(t, headerOnlyXLSX(t), false))

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", w.Code)
	}
	if got := len(st.GetAll()); got != 1 {
		t.Errorf("store holds %d subscriptions, want 1", got)
	}
}
