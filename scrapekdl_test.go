package scrapekdl_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestPublicCompileAndExtract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`<h1>Hello</h1>`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "example.kdl")
	spec := fmt.Sprintf(`extractor "example" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL)
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	program, diagnostics := scrapekdl.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	result, err := program.Extract(context.Background(), nil, scrapekdl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["title"] != "Hello" {
		t.Fatalf("result = %#v", result)
	}
}

func TestPublicValidationAndProgramMetadata(t *testing.T) {
	program, diagnostics := scrapekdl.CompileFile("fixtures/valid/basic-http.kdl")
	if diagnostics.HasErrors() || program == nil {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	if program.Name() != "basic-http" || program.Version() != 1 {
		t.Fatalf("program metadata = %q v%d", program.Name(), program.Version())
	}
	capabilities := program.Capabilities()
	if len(capabilities) != 1 || capabilities[0] != "http.fetch" {
		t.Fatalf("capabilities = %#v", capabilities)
	}
	capabilities[0] = "mutated"
	if got := program.Capabilities(); len(got) != 1 || got[0] != "http.fetch" {
		t.Fatalf("Capabilities returned aliased storage: %#v", got)
	}

	irJSON, err := program.IRJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(irJSON, &document); err != nil {
		t.Fatal(err)
	}
	if document["kind"] != "extractor" || document["name"] != "basic-http" {
		t.Fatalf("IR JSON metadata = %#v", document)
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.kdl")
	if err := os.WriteFile(invalidPath, []byte(`extractor "invalid" version=1 {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	validation := scrapekdl.ValidateFile(invalidPath)
	if !validation.HasErrors() {
		t.Fatalf("ValidateFile diagnostics = %#v", validation)
	}
	invalidProgram, compileDiagnostics := scrapekdl.CompileFile(invalidPath)
	if invalidProgram != nil || !compileDiagnostics.HasErrors() {
		t.Fatalf("CompileFile invalid result = %#v, %#v", invalidProgram, compileDiagnostics)
	}
}

func TestPublicExtractHTMLAndExecutionError(t *testing.T) {
	program, diagnostics := scrapekdl.CompileFile("fixtures/valid/basic-http.kdl")
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	html, err := os.ReadFile("fixtures/html/basic-http.html")
	if err != nil {
		t.Fatal(err)
	}
	result, err := program.ExtractHTML(context.Background(), string(html), scrapekdl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["title"] != "Scraping KDL Runtime" || result.Partial || len(result.Warnings) != 0 {
		t.Fatalf("result = %#v", result)
	}

	_, err = program.ExtractHTML(context.Background(), `<ul class="items"><li><span class="value">1</span></li></ul>`, scrapekdl.Options{})
	var executionError *scrapekdl.ExecutionError
	if !errors.As(err, &executionError) || executionError.Code != "E_REQUIRED_VALUE_MISSING" || executionError.Path != "output.title" {
		t.Fatalf("missing title error = %v", err)
	}
	if got := executionError.Error(); got != `E_REQUIRED_VALUE_MISSING at output.title: selector "h1" matched no elements` {
		t.Fatalf("ExecutionError.Error() = %q", got)
	}
}

func TestPublicExtractPreservesContextCancellation(t *testing.T) {
	program, diagnostics := scrapekdl.CompileFile("fixtures/valid/basic-http.kdl")
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := program.Extract(ctx, map[string]any{"id": "42"}, scrapekdl.Options{})
	var executionError *scrapekdl.ExecutionError
	if !errors.As(err, &executionError) || executionError.Code != "E_HTTP_FETCH" {
		t.Fatalf("canceled extraction error = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("public error does not preserve context cancellation: %v", err)
	}
}

func TestNormalizeBrowserResult(t *testing.T) {
	value, err := scrapekdl.NormalizeBrowserResult([]any{[]string{"one", "two"}, map[string]any{"count": int64(2)}})
	if err != nil {
		t.Fatal(err)
	}
	want := []any{[]any{"one", "two"}, map[string]any{"count": int64(2)}}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("normalized = %#v, want %#v", value, want)
	}
	for _, invalid := range []any{
		map[string]string{"key": "value"},
		[]map[string]any{{"key": "value"}},
		math.Inf(1),
	} {
		if _, err := scrapekdl.NormalizeBrowserResult(invalid); err == nil {
			t.Fatalf("invalid browser result succeeded: %#v", invalid)
		}
	}
}
