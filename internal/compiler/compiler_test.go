package compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func fixture(parts ...string) string {
	all := append([]string{"..", "..", "fixtures"}, parts...)
	return filepath.Join(all...)
}

func compileFile(t testing.TB, path string) (*ir.Extractor, diagnostic.List) {
	t.Helper()
	extractor, diagnostics, err := CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return extractor, diagnostics
}

func validateFile(t testing.TB, path string) diagnostic.List {
	t.Helper()
	diagnostics, err := ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}

func compileText(t *testing.T, content string) (*ir.Extractor, []string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extractor.kdl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	extractor, diagnostics := compileFile(t, path)
	codes := make([]string, len(diagnostics))
	for i := range diagnostics {
		codes[i] = diagnostics[i].Code
	}
	return extractor, codes
}

func TestCompileBasicHTTP(t *testing.T) {
	got, diags := compileFile(t, fixture("valid", "basic-http.kdl"))
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

func TestCompileSourceRetainsVirtualDisplayPath(t *testing.T) {
	source, err := os.ReadFile(fixture("valid", "basic-http.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	got, diagnostics, err := CompileSource(context.Background(), "<stdin>", source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.HasErrors() || got == nil {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "<stdin>" || got.Span.File != "<stdin>" {
		t.Fatalf("virtual source metadata = %#v, span = %#v", got.Files, got.Span)
	}
}

func TestCompileAllowsIndependentDocumentVersion(t *testing.T) {
	got, diags := compileFile(t, fixture("valid", "document-version-advance.kdl"))
	if diags.HasErrors() || got == nil {
		t.Fatalf("compile diagnostics = %#v", diags)
	}
	if got.Version != "2026-07-16" || got.LanguageVersion != "2026-07-15" || got.IRVersion != "2026-07-15" {
		t.Fatalf("version metadata = %#v", got)
	}
}

func TestCompileRaceDetailMatchesGolden(t *testing.T) {
	got, diags := compileFile(t, fixture("valid", "race-detail.kdl"))
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
		{name: "timeout overflow", path: fixture("invalid", "timeout-overflow.kdl"), code: "E_TYPE_MISMATCH"},
		{name: "integer version", path: fixture("invalid", "integer-version.kdl"), code: "E_DOCUMENT_VERSION_INVALID"},
		{name: "missing document version", path: fixture("invalid", "missing-document-version.kdl"), code: "E_DOCUMENT_VERSION_REQUIRED"},
		{name: "missing language version", path: fixture("invalid", "missing-language-version.kdl"), code: "E_LANGUAGE_VERSION_REQUIRED"},
		{name: "malformed document version", path: fixture("invalid", "malformed-document-version.kdl"), code: "E_DOCUMENT_VERSION_INVALID"},
		{name: "malformed language version", path: fixture("invalid", "malformed-language-version.kdl"), code: "E_LANGUAGE_VERSION_INVALID"},
		{name: "unknown language version", path: fixture("invalid", "unknown-language-version.kdl"), code: "E_LANGUAGE_VERSION_UNSUPPORTED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateFile(t, tt.path)
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

func TestVersionFixturesHaveExactDiagnostics(t *testing.T) {
	data, err := os.ReadFile(fixture("expected-diagnostics", "version-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]diagnostic.List
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	for path, want := range expected {
		t.Run(path, func(t *testing.T) {
			got := validateFile(t, fixture("invalid", path))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("diagnostics = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCompileInitializesIRArrays(t *testing.T) {
	result, diagnostics := compileFile(t, fixture("valid", "basic-http.kdl"))
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
	extractor, codes := compileText(t, `extractor "workflow" version="2026-07-15" language-version="2026-07-15" {
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
	extractor, codes := compileText(t, `extractor "invalid-workflow" version="2026-07-15" language-version="2026-07-15" {
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

func TestCompileRejectsDurationOverflow(t *testing.T) {
	extractor, codes := compileText(t, `extractor "duration-overflow" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="browser" url="https://example.invalid/"
    workflow {
      wait-for "#ready" timeout-ms=9223372036855
      wait-for-network-idle idle-ms=9223372036855
    }
  }
  field "title" type="string" required=#true {
    evaluate-js "() => 'title'" scope="document" returns="string" timeout-ms=9223372036855
  }
}`)
	if extractor != nil || !slices.Contains(codes, "E_TYPE_MISMATCH") {
		t.Fatalf("extractor = %#v, diagnostics = %v", extractor, codes)
	}
}

func TestCompileInputDefaults(t *testing.T) {
	extractor, codes := compileText(t, `extractor "input-defaults" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/{lang}" }
  input "lang" type="string" required=#false default="ja"
  input "enabled" type="bool" required=#false default=#true
  input "count" type="int" required=#false default=-2
  input "ratio" type="float" required=#false default=1.5
  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
  }
}`)
	if extractor == nil {
		t.Fatalf("compile diagnostics = %v", codes)
	}
	want := map[string]string{
		"lang": `"ja"`, "enabled": "true", "count": "-2", "ratio": "1.5",
	}
	for _, input := range extractor.Inputs {
		if input.Required || input.Default == nil || string(*input.Default) != want[input.Name] {
			t.Fatalf("input %q = %#v", input.Name, input)
		}
		delete(want, input.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing inputs = %v", want)
	}
	if len(extractor.Source.Fetch.URLTemplate.Segments) != 2 {
		t.Fatalf("URL template = %#v", extractor.Source.Fetch.URLTemplate)
	}
}

func TestCompileRejectsInvalidDefaults(t *testing.T) {
	extractor, codes := compileText(t, `extractor "invalid-defaults" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  input "required_value" type="string" required=#true default="x"
  input "wrong_bool" type="bool" required=#false default="true"
  field "title" type="string" required=#false default=1 {
    select "h1" match="one"
    value "text"
    on-error "default"
  }
}`)
	if extractor != nil {
		t.Fatalf("extractor = %#v", extractor)
	}
	for _, code := range []string{"E_INPUT_REQUIRED_DEFAULT", "E_DEFAULT_INVALID"} {
		if !slices.Contains(codes, code) {
			t.Fatalf("diagnostics %v do not contain %q", codes, code)
		}
	}
}

func TestCompileTokenizesEscapedURLTemplate(t *testing.T) {
	extractor, codes := compileText(t, `extractor "escaped-template" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/{{literal}}/{id}" }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	if extractor == nil {
		t.Fatalf("compile diagnostics = %v", codes)
	}
	template := extractor.Source.Fetch.URLTemplate
	if template.Raw != "https://example.invalid/{{literal}}/{id}" || len(template.Segments) != 2 {
		t.Fatalf("template = %#v", template)
	}
	literal, literalOK := template.Segments[0].(ir.LiteralTemplateSegment)
	input, inputOK := template.Segments[1].(ir.InputTemplateSegment)
	if !literalOK || literal.Value != "https://example.invalid/{literal}/" || !inputOK || input.Name != "id" {
		t.Fatalf("segments = %#v", template.Segments)
	}

	extractor, codes = compileText(t, `extractor "literal-placeholder" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/{{id}}" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	if extractor == nil || len(extractor.Source.Fetch.URLTemplate.Segments) != 1 {
		t.Fatalf("escaped placeholder compile = %#v, diagnostics = %v", extractor, codes)
	}
}

func TestCompileRejectsInvalidURLTemplates(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		input    string
		wantCode string
	}{
		{name: "unmatched open", url: "https://example.invalid/{id", input: `input "id" type="string" required=#true`, wantCode: "E_TEMPLATE_INVALID"},
		{name: "unmatched close", url: "https://example.invalid/id}", wantCode: "E_TEMPLATE_INVALID"},
		{name: "invalid placeholder", url: "https://example.invalid/{bad-name}", wantCode: "E_TEMPLATE_INVALID"},
		{name: "undeclared input", url: "https://example.invalid/{missing}", wantCode: "E_INPUT_UNDECLARED"},
		{name: "optional input", url: "https://example.invalid/{id}", input: `input "id" type="string" required=#false`, wantCode: "E_TEMPLATE_OPTIONAL_INPUT"},
		{name: "relative", url: "/path", wantCode: "E_TEMPLATE_INVALID"},
		{name: "unsupported scheme", url: "ftp://example.invalid/path", wantCode: "E_TEMPLATE_INVALID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, codes := compileText(t, fmt.Sprintf(`extractor "invalid-template" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url=%q }
  %s
  field "title" type="string" required=#true { select "h1"; value "text" }
}`, tt.url, tt.input))
			if !slices.Contains(codes, tt.wantCode) {
				t.Fatalf("diagnostics %v do not contain %q", codes, tt.wantCode)
			}
		})
	}
}

func TestCompileClassifiesPortableSelectors(t *testing.T) {
	compileSelector := func(t *testing.T, selector string) (*ir.Extractor, []string) {
		t.Helper()
		return compileText(t, fmt.Sprintf(`extractor "selector" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "value" type="string" required=#true { select %q match="one"; value "text" }
}`, selector))
	}
	supported := `ul.items > li:nth-child(2n+1):not(.disabled) a[href^="https"]`
	if extractor, codes := compileSelector(t, supported); extractor == nil {
		t.Fatalf("supported selector diagnostics = %v", codes)
	}

	for _, tt := range []struct {
		name     string
		selector string
		wantCode string
	}{
		{name: "empty", selector: "", wantCode: "E_SELECTOR_INVALID"},
		{name: "malformed attribute", selector: "[", wantCode: "E_SELECTOR_INVALID"},
		{name: "unsupported pseudo", selector: "a:hover", wantCode: "E_SELECTOR_UNSUPPORTED"},
		{name: "pseudo element", selector: "a::before", wantCode: "E_SELECTOR_UNSUPPORTED"},
		{name: "attribute flag", selector: `a[href="x" i]`, wantCode: "E_SELECTOR_UNSUPPORTED"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, codes := compileSelector(t, tt.selector)
			if !slices.Contains(codes, tt.wantCode) {
				t.Fatalf("diagnostics %v do not contain %q", codes, tt.wantCode)
			}
		})
	}
}
