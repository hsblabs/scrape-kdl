package exampleharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckedInExamples(t *testing.T) {
	report, err := Check(filepath.Join("..", ".."), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Examples < 4 {
		t.Fatalf("examples = %d, want at least 4", report.Examples)
	}
	if len(report.Updated) != 0 {
		t.Fatalf("ordinary check updated files: %v", report.Updated)
	}
}

func TestCompareJSONReportsFocusedPath(t *testing.T) {
	difference := compareJSON(
		[]byte(`{"value":{"items":[{"number":1}]}}`),
		[]byte(`{"value":{"items":[{"number":2}]}}`),
	)
	if !strings.Contains(difference, "$.value.items[0].number: expected 1, got 2") {
		t.Fatalf("difference = %q", difference)
	}
}

func TestManifestRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.json")
	if err := os.WriteFile(path, []byte(`{"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readManifest(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field", err)
	}
}
