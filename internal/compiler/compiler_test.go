package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func fixture(parts ...string) string {
	all := append([]string{"..", "..", "fixtures"}, parts...)
	return filepath.Join(all...)
}

func TestCompileBasicHTTP(t *testing.T) {
	got, diags := CompileFile(fixture("valid", "basic-http.kdl"))
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	if got == nil {
		t.Fatal("expected IR")
	}
	if got.Name != "basic-http" || got.Source.Fetch.Mode != "http" {
		t.Fatalf("unexpected extractor: %#v", got)
	}
	if !slices.Equal(got.Capabilities, []string{"http.fetch"}) {
		t.Fatalf("unexpected capabilities: %v", got.Capabilities)
	}
	if len(got.Output.Members) != 2 {
		t.Fatalf("expected two output members, got %d", len(got.Output.Members))
	}
}

func TestCompileRaceDetailMatchesGolden(t *testing.T) {
	got, diags := CompileFile(fixture("valid", "race-detail.kdl"))
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	want, err := os.ReadFile(fixture("expected-ir", "race-detail.ir.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(want) {
		t.Fatalf("IR differs from golden; regenerate with: go run ./cmd/scrape-kdl compile ./fixtures/valid/race-detail.kdl --out ./fixtures/expected-ir/race-detail.ir.json")
	}
}

func TestInvalidFixtures(t *testing.T) {
	tests := []struct {
		name string
		path string
		code string
	}{
		{name: "http javascript", path: fixture("invalid", "http-js.kdl"), code: "E_BROWSER_CAPABILITY_REQUIRED"},
		{name: "transform type mismatch", path: fixture("invalid", "transform-type-mismatch.kdl"), code: "E_TRANSFORM_TYPE_MISMATCH"},
		{name: "duplicate property", path: fixture("invalid", "duplicate-property.kdl"), code: "E_DUPLICATE_PROPERTY"},
		{name: "import cycle", path: fixture("invalid", "import-cycle.kdl"), code: "E_IMPORT_CYCLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := ValidateFile(tt.path)
			if !diags.HasErrors() {
				t.Fatalf("expected an error, got %#v", diags)
			}
			found := false
			for _, d := range diags {
				if d.Code == tt.code {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected diagnostic %s, got %#v", tt.code, diags)
			}
		})
	}
}

func TestCompileInitializesIRArrays(t *testing.T) {
	result, diagnostics := CompileFile(fixture("valid", "basic-http.kdl"))
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	if result.Transforms == nil {
		t.Fatal("transforms must be an empty array, not null")
	}
	if result.Source.Workflow == nil {
		t.Fatal("workflow must be an empty array, not null")
	}
	if result.Output.Members == nil {
		t.Fatal("output members must be an array")
	}
}
