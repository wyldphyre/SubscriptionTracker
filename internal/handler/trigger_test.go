package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHXTriggerSurvivesAwkwardText covers import warnings, which embed
// spreadsheet-derived names. These used to be interpolated into the header with
// %q, so a name with a newline or a quote produced a malformed header.
func TestHXTriggerSurvivesAwkwardText(t *testing.T) {
	msg := "Imported 2 subscriptions (1 warning: row 3 (\"Bad\" name\nwith newline): unknown cycle)"

	w := httptest.NewRecorder()
	notifyChanged(w, msg)

	got := w.Header().Get("HX-Trigger")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("header contains a line break, which would corrupt the response: %q", got)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("header is not valid JSON (%v): %q", err, got)
	}
	if decoded["subsChanged"] != true {
		t.Errorf("subsChanged = %v, want true", decoded["subsChanged"])
	}
	toast, _ := decoded["showToast"].(string)
	if !strings.Contains(toast, "Imported 2 subscriptions") {
		t.Errorf("toast text lost: %q", toast)
	}
	if !strings.Contains(toast, `"Bad" name`) {
		t.Errorf("quotes in the message were not preserved: %q", toast)
	}
}
