package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func fixture(parts ...string) string {
	all := append([]string{"..", "..", "fixtures"}, parts...)
	return filepath.Join(all...)
}

func compileText(t *testing.T, content string) (*ir.Extractor, []string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extractor.kdl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	extractor, diagnostics := CompileFile(path)
	codes := make([]string, len(diagnostics))
	for i := range diagnostics {
		codes[i] = diagnostics[i].Code
	}
	return extractor, codes
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

func TestCompileBrowserWorkflowSteps(t *testing.T) {
	extractor, codes := compileText(t, `extractor "workflow" version=1 {
  source "html" {
    fetch mode="browser" url="https://example.invalid/"
    workflow {
      wait-for "#ready" state="attached" timeout-ms=100
      click "button" timeout-ms=200
      fill "input" "value" timeout-ms=300
      press "input" "Enter" timeout-ms=400
      scroll 1.5 -2
      wait-for-network-idle idle-ms=250 timeout-ms=500
      evaluate-js "() => null" timeout-ms=600
    }
  }
  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
  }
}`)
	if slices.Contains(codes, "E_TYPE_MISMATCH") || extractor == nil {
		t.Fatalf("compile = %#v, diagnostics = %v", extractor, codes)
	}
	if len(extractor.Source.Workflow) != 7 {
		t.Fatalf("workflow = %#v", extractor.Source.Workflow)
	}
	if step, ok := extractor.Source.Workflow[0].(ir.WaitForStep); !ok || step.State != "attached" || step.TimeoutMS == nil || *step.TimeoutMS != 100 {
		t.Fatalf("wait-for = %#v", extractor.Source.Workflow[0])
	}
	if step, ok := extractor.Source.Workflow[4].(ir.ScrollStep); !ok || step.X != 1.5 || step.Y != -2 {
		t.Fatalf("scroll = %#v", extractor.Source.Workflow[4])
	}
	if step, ok := extractor.Source.Workflow[5].(ir.NetworkIdleStep); !ok || step.IdleMS != 250 || step.TimeoutMS == nil || *step.TimeoutMS != 500 {
		t.Fatalf("network idle = %#v", extractor.Source.Workflow[5])
	}
	for _, capability := range []string{"browser.evaluate-js", "browser.input", "browser.navigate", "browser.network-idle", "browser.query", "browser.read-text", "browser.scroll", "browser.wait"} {
		if !slices.Contains(extractor.Capabilities, capability) {
			t.Fatalf("capabilities %v do not contain %q", extractor.Capabilities, capability)
		}
	}
}

func TestCompileRejectsInvalidBrowserWorkflowSteps(t *testing.T) {
	extractor, codes := compileText(t, `extractor "invalid-workflow" version=1 {
  source "html" {
    fetch mode="browser" url="https://example.invalid/"
    workflow {
      wait-for "[" state="moving" timeout-ms=0
      scroll "right" 0
      wait-for-network-idle idle-ms=0 timeout-ms=0
      unsupported-step
    }
  }
  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
  }
}`)
	if extractor != nil {
		t.Fatalf("extractor = %#v", extractor)
	}
	for _, code := range []string{"E_SELECTOR_INVALID", "E_TYPE_MISMATCH", "E_UNKNOWN_NODE"} {
		if !slices.Contains(codes, code) {
			t.Fatalf("diagnostics %v do not contain %q", codes, code)
		}
	}
}
