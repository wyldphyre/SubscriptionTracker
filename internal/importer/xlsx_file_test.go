package importer

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// writeSheet builds a throwaway xlsx from rows and returns its path.
func writeSheet(t *testing.T, rows [][]string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "book.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportXLSXMapsCyclesFromSpreadsheetLabels(t *testing.T) {
	path := writeSheet(t, [][]string{
		{"Name", "Cost", "Cost Curency", "Cycle", "Category", "Notes"},
		{"Paper", "5", "AUD", "Weekly", "News", ""},
		{"Gym", "30", "AUD", "Quarterly", "Health", ""},
		{"Insurance", "600", "AUD", "Six Monthly", "Home", ""},
		{"Domain", "40", "USD", "Every 2 Years", "Tech", ""},
	})

	res, err := ImportXLSX(path)
	if err != nil {
		t.Fatalf("ImportXLSX() error = %v", err)
	}
	if res.Count != 4 {
		t.Fatalf("Count = %d, want 4", res.Count)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}

	want := map[string]string{
		"Paper": "weekly", "Gym": "quarterly",
		"Insurance": "sixmonthly", "Domain": "every2years",
	}
	for _, sub := range res.Subscriptions {
		if got := string(sub.Cycle); got != want[sub.Name] {
			t.Errorf("%s cycle = %q, want %q", sub.Name, got, want[sub.Name])
		}
	}
}

func TestImportXLSXWarnsOnUnknownCycle(t *testing.T) {
	path := writeSheet(t, [][]string{
		{"Name", "Cost", "Cycle"},
		{"Mystery", "10", "whenever"},
	})

	res, err := ImportXLSX(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want one about the unknown cycle", res.Warnings)
	}
	// Still imported, just flagged rather than silently mis-costed.
	if res.Count != 1 || res.Subscriptions[0].Cycle != "monthly" {
		t.Errorf("got %d subs with cycle %q, want 1 defaulted to monthly",
			res.Count, res.Subscriptions[0].Cycle)
	}
}

func TestImportXLSXHeaderOnlySheetYieldsNothing(t *testing.T) {
	path := writeSheet(t, [][]string{{"Name", "Cost", "Cycle"}})

	res, err := ImportXLSX(path)
	if err != nil {
		t.Fatalf("ImportXLSX() error = %v", err)
	}
	if res.Count != 0 || len(res.Subscriptions) != 0 {
		t.Errorf("Count = %d, Subscriptions = %v, want empty", res.Count, res.Subscriptions)
	}
}

func TestImportXLSXRejectsUnrelatedSheet(t *testing.T) {
	path := writeSheet(t, [][]string{{"Foo", "Bar"}, {"1", "2"}})

	if _, err := ImportXLSX(path); err == nil {
		t.Fatal("ImportXLSX() error = nil, want an error about the missing Name column")
	}
}
