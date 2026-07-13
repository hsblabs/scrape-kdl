package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/compiler"
)

func compileTestSpec(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extractor.kdl")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExecuteHTTP(t *testing.T) {
	var requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestURI = request.RequestURI
		if request.Header.Get("X-Test") != "yes" {
			t.Errorf("session header missing")
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(`<!doctype html><html><body>
<h1>  Example   Title </h1>
<ul class="items">
<li><span class="value">1</span><a href="/horse/alpha">Alpha</a><span class="sex">牡</span></li>
<li><span class="value">999</span><a href="/horse/broken">Broken</a><span class="sex">牝</span></li>
<li><span class="value">2</span><a href="/horse/beta">Beta</a><span class="sex">セ</span></li>
</ul></body></html>`))
	}))
	defer server.Close()

	spec := fmt.Sprintf(`extractor "http-runtime" version=1 {
  source "html" {
    fetch mode="http" url=%q
    session policy="optional"
  }
  input "id" type="string" required=#true

  transform "horse_id" input="string" output="string?" {
    pipeline {
      apply "regex-capture" pattern="/horse/([^/]+)" group=1
    }
  }
  transform "sex_name" input="string" output="string" {
    match {
      case "牡" "male"
      case "牝" "female"
      case "セ" "gelding"
      default "unknown"
    }
  }

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }
  field "missing" type="string" required=#false {
    select ".not-found" match="first"
    value "text"
  }
  collection "items" min-items=1 on-row-error="skip" {
    select "ul.items > li"
    field "number" type="u8" required=#true {
      select ".value"
      value "text"
      apply "trim"
      apply "parse-int" as="u8"
    }
    field "horse_id" type="string?" required=#false {
      select "a"
      value "attr" name="href"
      apply "horse_id"
    }
    field "sex" type="string" required=#true {
      select ".sex"
      value "text"
      apply "sex_name"
    }
  }
}`, server.URL+`/item/{id}`)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		diagnostics.WriteText(os.Stderr)
		t.Fatal("compile failed")
	}
	result, err := Execute(context.Background(), extractor, map[string]any{"id": "a b"}, Options{
		Session: &Session{Headers: http.Header{"X-Test": []string{"yes"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestURI != "/item/a%20b" {
		t.Fatalf("request URI = %q", requestURI)
	}
	if result.Value["title"] != "Example Title" {
		t.Fatalf("title = %#v", result.Value["title"])
	}
	if result.Value["missing"] != nil {
		t.Fatalf("missing = %#v", result.Value["missing"])
	}
	items, ok := result.Value["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v", result.Value["items"])
	}
	first := items[0].(map[string]any)
	if first["number"] != uint8(1) || first["horse_id"] != "alpha" || first["sex"] != "male" {
		t.Fatalf("first = %#v", first)
	}
	if !result.Partial || len(result.Warnings) != 2 || result.Warnings[0].Code != "W_ROW_SKIPPED" || result.Warnings[1].Code != "W_PARTIAL_EXTRACTION" {
		t.Fatalf("recovery = partial:%v warnings:%#v", result.Partial, result.Warnings)
	}
}

func TestExecuteFieldWarningAndExternalTransform(t *testing.T) {
	spec := `extractor "external" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid" }
  transform "decorate" input="string" output="string" { external symbol="decorate" }
  field "bad" type="u8" required=#false {
    select ".bad"
    value "text"
    apply "parse-int" as="u8"
    on-error "warn"
  }
  field "name" type="string" required=#true {
    select ".name"
    value "text"
    apply "decorate"
  }
}`
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	result, err := ExecuteHTML(context.Background(), extractor, `<div class="bad">x</div><div class="name">A</div>`, Options{
		ExternalTransforms: map[string]ExternalTransform{
			"decorate": func(_ context.Context, input any) (any, error) { return "[" + input.(string) + "]", nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["bad"] != nil || result.Value["name"] != "[A]" || !result.Partial || len(result.Warnings) != 2 {
		t.Fatalf("result = %#v", result)
	}
	if result.Warnings[0].Code != "W_ERROR_RECOVERED" || result.Warnings[1].Code != "W_PARTIAL_EXTRACTION" {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func TestExecuteRejectsMissingExternalBeforeFetch(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = true
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "external" version=1 {
  source "html" { fetch mode="http" url=%q }
  transform "x" input="string" output="string" { external symbol="missing" }
  field "x" type="string" required=#true {
    select "h1"
    value "text"
    apply "x"
  }
}`, server.URL)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		diagnostics.WriteText(os.Stderr)
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{})
	if err == nil || !strings.Contains(err.Error(), "E_EXTERNAL_TRANSFORM_MISSING") {
		t.Fatalf("error = %v", err)
	}
	if requested {
		t.Fatal("network request occurred before capability validation")
	}
}

func TestDecodeWindows1252(t *testing.T) {
	decoded, err := decodeHTML([]byte{'c', 'a', 'f', 0xe9, ' ', 0x80}, "text/html; charset=windows-1252")
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "café €" {
		t.Fatalf("decoded = %q", decoded)
	}
}

func TestSessionNoneIsIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Secret") != "" {
			t.Errorf("session header must be ignored for policy none")
		}
		_, _ = writer.Write([]byte(`<h1>ok</h1>`))
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "session-none" version=1 {
  source "html" {
    fetch mode="http" url=%q
    session policy="none"
  }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{Session: &Session{Headers: http.Header{"X-Secret": []string{"value"}}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequiredInputFailsBeforeFetch(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = true
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "required-input" version=1 {
  source "html" { fetch mode="http" url=%q }
  input "id" type="string" required=#true
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL+`/{id}`)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{})
	if err == nil || !strings.Contains(err.Error(), "E_INPUT_REQUIRED") {
		t.Fatalf("error = %v", err)
	}
	if requested {
		t.Fatal("network request occurred before input validation")
	}
}

func TestResponseBodyLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`<h1>too large</h1>`))
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "body-limit" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{MaxResponseBytes: 4})
	if err == nil || !strings.Contains(err.Error(), "E_HTTP_BODY_TOO_LARGE") {
		t.Fatalf("error = %v", err)
	}
}

func TestCustomCharsetDecoder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=shift_jis")
		_, _ = writer.Write([]byte("ignored"))
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "charset" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	called := false
	result, err := Execute(context.Background(), extractor, nil, Options{CharsetDecoder: func(body []byte, charset string) (string, error) {
		called = true
		if charset != "shift_jis" {
			t.Fatalf("charset = %q", charset)
		}
		return `<h1>日本語</h1>`, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !called || result.Value["title"] != "日本語" {
		t.Fatalf("result = %#v called=%v", result, called)
	}
}

func TestExecuteURLPolicyRejectsBeforeFetch(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = true
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "policy" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`, server.URL)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{
		URLPolicy: func(context.Context, *url.URL) error { return errors.New("blocked") },
	})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" {
		t.Fatalf("error = %v", err)
	}
	if requested {
		t.Fatal("network request occurred before URL policy validation")
	}
}

func TestExecuteURLPolicyChecksRedirects(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, "/blocked", http.StatusFound)
			return
		}
		_, _ = writer.Write([]byte(`<h1>should-not-run</h1>`))
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "policy-redirect" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`, server.URL+"/start")
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{
		URLPolicy: func(_ context.Context, target *url.URL) error {
			if target.Path == "/blocked" {
				return errors.New("redirect blocked")
			}
			return nil
		},
	})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(requests, []string{"/start"}) {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestExecuteHTTPTimeoutUsesStableCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	spec := fmt.Sprintf(`extractor "timeout" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`, server.URL)
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	_, err := Execute(context.Background(), extractor, nil, Options{RequestTimeout: 10 * time.Millisecond})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_TIMEOUT" {
		t.Fatalf("error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestExecuteHTTPPreservesParentCancellation(t *testing.T) {
	spec := `extractor "canceled" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	path := compileTestSpec(t, spec)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	transportCalled := false
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		transportCalled = true
		return nil, request.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Execute(ctx, extractor, nil, Options{HTTPClient: client})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_HTTP_FETCH" {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error does not preserve context cancellation: %v", err)
	}
	if transportCalled {
		t.Fatal("HTTP transport was called after parent context cancellation")
	}
}
