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

func compileFile(t testing.TB, ctx context.Context, path string) (*scrapekdl.Program, scrapekdl.Diagnostics) {
	t.Helper()
	program, diagnostics, err := scrapekdl.CompileFile(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	return program, diagnostics
}

func compileSource(t testing.TB, ctx context.Context, source scrapekdl.Source, options scrapekdl.CompileOptions) (*scrapekdl.Program, scrapekdl.Diagnostics) {
	t.Helper()
	program, diagnostics, err := scrapekdl.Compile(ctx, source, options)
	if err != nil {
		t.Fatal(err)
	}
	return program, diagnostics
}

func validateFile(t testing.TB, ctx context.Context, path string) scrapekdl.Diagnostics {
	t.Helper()
	diagnostics, err := scrapekdl.ValidateFile(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}

func TestPublicCompileAndExtract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`<h1>Hello</h1>`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "example.kdl")
	spec := fmt.Sprintf(`extractor "example" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL)
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	program, diagnostics := compileFile(t, context.Background(), path)
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
	program, diagnostics := compileFile(t, context.Background(), "fixtures/valid/basic-http.kdl")
	if diagnostics.HasErrors() || program == nil {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	if program.Name() != "basic-http" || program.Version() != "2026-07-15" {
		t.Fatalf("program metadata = %q v%s", program.Name(), program.Version())
	}
	if got := scrapekdl.SupportedLanguageVersions(); !reflect.DeepEqual(got, []string{"2026-07-15"}) {
		t.Fatalf("supported language versions = %v", got)
	}
	if got := scrapekdl.SupportedIRVersions(); !reflect.DeepEqual(got, []string{"2026-07-15"}) {
		t.Fatalf("supported IR versions = %v", got)
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
	if err := os.WriteFile(invalidPath, []byte(`extractor "invalid" version="2026-07-15" language-version="2026-07-15" {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	validation := validateFile(t, context.Background(), invalidPath)
	if !validation.HasErrors() {
		t.Fatalf("ValidateFile diagnostics = %#v", validation)
	}
	invalidProgram, compileDiagnostics := compileFile(t, context.Background(), invalidPath)
	if invalidProgram != nil || !compileDiagnostics.HasErrors() {
		t.Fatalf("CompileFile invalid result = %#v, %#v", invalidProgram, compileDiagnostics)
	}
}

func TestPublicCompileUsesInjectedSourceLoader(t *testing.T) {
	const mainSource = `import "common.kdl" as="common"
extractor "memory" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	const moduleSource = `module "common" version="2026-07-15" language-version="2026-07-15" {}`
	loaded := []string{}
	program, diagnostics := compileSource(t, context.Background(), scrapekdl.Source{
		Path: "spec/main.kdl", Data: []byte(mainSource),
	}, scrapekdl.CompileOptions{Loader: func(_ context.Context, path string) ([]byte, error) {
		loaded = append(loaded, path)
		if path != "spec/common.kdl" {
			return nil, fmt.Errorf("unexpected path %q", path)
		}
		return []byte(moduleSource), nil
	}})
	if diagnostics.HasErrors() || program == nil {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	if !reflect.DeepEqual(loaded, []string{"spec/common.kdl"}) {
		t.Fatalf("loaded paths = %v", loaded)
	}
	metadata := program.Metadata()
	if metadata.Name != "memory" || metadata.Version != "2026-07-15" || metadata.LanguageVersion != "2026-07-15" || metadata.IRVersion != "2026-07-15" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if len(metadata.Files) != 2 || metadata.Files[0].Path != "common.kdl" || metadata.Files[0].ModuleName != "common" || metadata.Files[1].Path != "main.kdl" {
		t.Fatalf("source files = %#v", metadata.Files)
	}
	metadata.Files[0].Path = "mutated"
	metadata.Capabilities[0] = "mutated"
	again := program.Metadata()
	if again.Files[0].Path != "common.kdl" || again.Capabilities[0] != "http.fetch" {
		t.Fatalf("metadata aliases internal storage: %#v", again)
	}
}

func TestPublicCompilePreservesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loaderCalled := false
	program, diagnostics, err := scrapekdl.Compile(ctx, scrapekdl.Source{
		Path: "main.kdl", Data: []byte(`extractor "canceled" version="2026-07-15" language-version="2026-07-15" {}`),
	}, scrapekdl.CompileOptions{Loader: func(context.Context, string) ([]byte, error) {
		loaderCalled = true
		return nil, nil
	}})
	if program != nil || len(diagnostics) != 0 || !errors.Is(err, context.Canceled) || loaderCalled {
		t.Fatalf("program=%#v diagnostics=%#v err=%v loaderCalled=%v", program, diagnostics, err, loaderCalled)
	}
}

func TestPublicCompilePreservesLoaderErrors(t *testing.T) {
	const source = `import "common.kdl" as="common"
extractor "loader-error" version="2026-07-15" language-version="2026-07-15" {}`
	sentinel := errors.New("sentinel loader error")
	program, diagnostics, err := scrapekdl.Compile(context.Background(), scrapekdl.Source{
		Path: "main.kdl", Data: []byte(source),
	}, scrapekdl.CompileOptions{Loader: func(context.Context, string) ([]byte, error) {
		return nil, sentinel
	}})
	if program != nil || len(diagnostics) != 0 || !errors.Is(err, sentinel) {
		t.Fatalf("program=%#v diagnostics=%#v err=%v", program, diagnostics, err)
	}
}

func TestPublicCompilePreservesCancellationDuringLoader(t *testing.T) {
	const source = `import "common.kdl" as="common"
extractor "loader-canceled" version="2026-07-15" language-version="2026-07-15" {}`
	ctx, cancel := context.WithCancel(context.Background())
	program, diagnostics, err := scrapekdl.Compile(ctx, scrapekdl.Source{
		Path: "main.kdl", Data: []byte(source),
	}, scrapekdl.CompileOptions{Loader: func(ctx context.Context, _ string) ([]byte, error) {
		cancel()
		return nil, ctx.Err()
	}})
	if program != nil || len(diagnostics) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("program=%#v diagnostics=%#v err=%v", program, diagnostics, err)
	}
}

func TestPublicCompileSeparatesFilesystemErrorsFromDiagnostics(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.kdl")
	program, diagnostics, err := scrapekdl.CompileFile(context.Background(), missing)
	if program != nil || len(diagnostics) != 0 || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("program=%#v diagnostics=%#v err=%v", program, diagnostics, err)
	}

	program, diagnostics, err = scrapekdl.Compile(context.Background(), scrapekdl.Source{
		Path: "invalid.kdl", Data: []byte("not valid KDL !"),
	}, scrapekdl.CompileOptions{})
	if program != nil || !diagnostics.HasErrors() || err != nil {
		t.Fatalf("program=%#v diagnostics=%#v err=%v", program, diagnostics, err)
	}
}

func TestPublicExtractHTMLAndExecutionError(t *testing.T) {
	program, diagnostics := compileFile(t, context.Background(), "fixtures/valid/basic-http.kdl")
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
	program, diagnostics := compileFile(t, context.Background(), "fixtures/valid/basic-http.kdl")
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := program.Extract(ctx, map[string]any{"id": "42"}, scrapekdl.Options{})
	var executionError *scrapekdl.ExecutionError
	if !errors.As(err, &executionError) || executionError.Code != "E_EXECUTION_CANCELED" {
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
