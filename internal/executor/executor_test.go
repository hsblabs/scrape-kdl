package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/compiler"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
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

func TestExecuteHTMLFieldRecoveryPolicies(t *testing.T) {
	path := compileTestSpec(t, `extractor "http-recovery" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "nulled" type="u8" required=#false {
    select ".bad"; value "text"; apply "parse-int" as="u8"; on-error "null"
  }
  field "warned" type="u8" required=#false {
    select ".bad"; value "text"; apply "parse-int" as="u8"; on-error "warn"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	result, err := ExecuteHTML(context.Background(), extractor, `<span class="bad">invalid</span>`, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["nulled"] != nil || result.Value["warned"] != nil || !result.Partial {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Warnings) != 2 || result.Warnings[0].Code != "W_ERROR_RECOVERED" || result.Warnings[0].Path != "output.warned" || result.Warnings[1].Code != "W_PARTIAL_EXTRACTION" {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func TestExecuteHTMLFieldRecoveryFail(t *testing.T) {
	path := compileTestSpec(t, `extractor "http-recovery-fail" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "value" type="u8" required=#true {
    select ".bad"; value "text"; apply "parse-int" as="u8"; on-error "fail"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	_, err := ExecuteHTML(context.Background(), extractor, `<span class="bad">invalid</span>`, Options{})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_TRANSFORM" || execution.Path != "output.value" || execution.Cause == nil {
		t.Fatalf("error = %#v", err)
	}
}

func TestExecuteHTMLRequiredMissingIsNotRecovered(t *testing.T) {
	path := compileTestSpec(t, `extractor "http-required-missing" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "value" type="string" required=#true {
    select ".missing" match="first"; value "text"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	_, err := ExecuteHTML(context.Background(), extractor, `<span>present</span>`, Options{})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_REQUIRED_VALUE_MISSING" || execution.Path != "output.value" {
		t.Fatalf("error = %#v", err)
	}
}

func TestExecuteHTMLNormalizesNumericFieldDefaults(t *testing.T) {
	path := compileTestSpec(t, `extractor "numeric-defaults" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "missing_count" type="int" required=#false default=7 {
    select ".missing"; value "text"; apply "parse-int" as="int"; on-error "default"
  }
  field "recovered_count" type="int" required=#false default=8 {
    select ".bad"; value "text"; apply "parse-int" as="int"; on-error "default"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	result, err := ExecuteHTML(context.Background(), extractor, `<span class="bad">invalid</span>`, Options{})
	if err != nil || result.Value["missing_count"] != int64(7) || result.Value["recovered_count"] != int64(8) || !result.Partial {
		t.Fatalf("result = %#v, error = %v", result, err)
	}

	field := extractor.Output.Members[1].(ir.Field)
	outOfRange := json.RawMessage(`9223372036854775808`)
	field.Default = &outOfRange
	extractor.Output.Members[1] = field
	_, err = ExecuteHTML(context.Background(), extractor, `<span class="bad">invalid</span>`, Options{})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "output.recovered_count" {
		t.Fatalf("malformed default error = %#v", err)
	}
}

func TestExecuteNormalizesNumericTransformLiterals(t *testing.T) {
	path := compileTestSpec(t, `extractor "numeric-literals" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  transform "maybe_count" input="string" output="int?" {
    match {
      case "one" 1
      default #null
    }
  }
  field "matched" type="int" required=#true {
    select ".matched"
    value "text"
    apply "maybe_count"
    apply "coalesce" value=3
  }
  field "defaulted" type="int" required=#true {
    select ".defaulted"
    value "text"
    apply "maybe_count"
    apply "coalesce" value=3
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	html := `<div class="matched">one</div><div class="defaulted">other</div>`
	result, err := ExecuteHTML(context.Background(), extractor, html, Options{})
	if err != nil || result.Value["matched"] != int64(1) || result.Value["defaulted"] != int64(3) {
		t.Fatalf("result = %#v, error = %v", result, err)
	}

	match := extractor.Transforms[0].(ir.MatchTransform)
	match.Cases[0].Then = json.RawMessage(`"invalid"`)
	extractor.Transforms[0] = match
	_, err = ExecuteHTML(context.Background(), extractor, html, Options{})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_TRANSFORM" || execution.Path != "maybe_count" || !strings.Contains(execution.Message, "not assignable to int?") {
		t.Fatalf("invalid match result error = %v", err)
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

func TestExecuteHTTPSessionConstructionIsDeterministic(t *testing.T) {
	path := compileTestSpec(t, `extractor "session-order" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/"; session policy="optional" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	wantHeaders := []string{"upper", "lower-one", "lower-two"}
	wantCookies := []string{"first", "second"}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Values("X-Order"); !reflect.DeepEqual(got, wantHeaders) {
			t.Fatalf("header value order = %v", got)
		}
		cookies := request.Cookies()
		gotCookies := make([]string, 0, len(cookies))
		for _, cookie := range cookies {
			if cookie.Name == "duplicate" {
				gotCookies = append(gotCookies, cookie.Value)
			}
		}
		if !reflect.DeepEqual(gotCookies, wantCookies) {
			t.Fatalf("cookie value order = %v", gotCookies)
		}
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`<h1>ok</h1>`)), Request: request,
		}, nil
	})}
	session := &Session{
		Headers: http.Header{
			"x-order": []string{"lower-one", "lower-two"},
			"X-Order": []string{"upper"},
		},
		Cookies: []*http.Cookie{
			nil,
			{Name: "duplicate", Value: "first"},
			{Name: "duplicate", Value: "second"},
		},
	}
	for range 100 {
		result, err := Execute(context.Background(), extractor, nil, Options{HTTPClient: client, Session: session})
		if err != nil || result.Value["title"] != "ok" {
			t.Fatalf("result = %#v, error = %v", result, err)
		}
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

func TestExecuteHTTPPreflightRejectsBeforeTransport(t *testing.T) {
	const spec = `extractor "http-preflight" version=1 {
  source "html" {
    fetch mode="http" url="https://example.invalid/{id}"
    session policy="required"
  }
  input "id" type="int" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	tests := []struct {
		name     string
		mutate   func(*ir.Extractor)
		inputs   map[string]any
		session  *Session
		wantCode string
	}{
		{name: "success", inputs: map[string]any{"id": int64(1)}, session: &Session{}},
		{
			name: "external transform",
			mutate: func(extractor *ir.Extractor) {
				extractor.Transforms = append(extractor.Transforms, ir.ExternalTransform{
					Kind: "external", TransformBase: ir.TransformBase{SymbolID: "external", Name: "external"}, Symbol: "missing",
				})
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_EXTERNAL_TRANSFORM_MISSING",
		},
		{
			name: "invalid selector",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Selection.Selector = "["
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_SELECTOR_INVALID",
		},
		{
			name: "browser value source",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Selection = nil
				field.ValueSource = ir.JavaScriptValueSource{Kind: "javascript", Scope: "document", Source: `() => "value"`, Returns: field.SuccessfulType}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_BROWSER_RUNTIME_MISSING",
		},
		{
			name: "unknown value source",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.ValueSource = nil
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown builtin transform",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{Target: ir.BuiltinTarget{Kind: "builtin", Name: "missing"}}}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_TRANSFORM",
		},
		{
			name: "missing declared transform",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{Target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:missing"}}}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_TRANSFORM_MISSING",
		},
		{
			name: "duplicate transform argument",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{
					Target: ir.BuiltinTarget{Kind: "builtin", Name: "prepend"},
					NamedArguments: []ir.NamedArgument{
						{Name: "value", Value: json.RawMessage(`"first"`)},
						{Name: "value", Value: json.RawMessage(`"second"`)},
					},
				}}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_TRANSFORM",
		},
		{
			name: "malformed transform argument",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{
					Target:              ir.BuiltinTarget{Kind: "builtin", Name: "assert-enum"},
					PositionalArguments: []json.RawMessage{json.RawMessage(`not-json`)},
				}}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_TRANSFORM",
		},
		{
			name: "field transform input discontinuity",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{
					Target: ir.BuiltinTarget{Kind: "builtin", Name: "trim"},
					Input:  typesys.Primitive("bool"), Output: typesys.Primitive("string"),
				}}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "malformed match literal",
			mutate: func(extractor *ir.Extractor) {
				stringType := typesys.Primitive("string")
				extractor.Transforms = append(extractor.Transforms, ir.MatchTransform{
					Kind:          "match",
					TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: stringType},
					Default:       json.RawMessage(`not-json`),
				})
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_TRANSFORM",
		},
		{
			name: "unknown transform argument",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{
					Target:         ir.BuiltinTarget{Kind: "builtin", Name: "trim"},
					NamedArguments: []ir.NamedArgument{{Name: "unknown", Value: json.RawMessage(`true`)}},
				}}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_TRANSFORM",
		},
		{
			name: "duplicate transform symbol",
			mutate: func(extractor *ir.Extractor) {
				base := ir.TransformBase{SymbolID: "transform:duplicate", Name: "duplicate"}
				extractor.Transforms = append(extractor.Transforms,
					ir.PipelineTransform{Kind: "pipeline", TransformBase: base},
					ir.PipelineTransform{Kind: "pipeline", TransformBase: base},
				)
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "duplicate input declaration",
			mutate: func(extractor *ir.Extractor) {
				extractor.Inputs = append(extractor.Inputs, extractor.Inputs[0])
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "malformed hidden input default",
			mutate: func(extractor *ir.Extractor) {
				value := json.RawMessage(`not-json`)
				extractor.Inputs[0].Required = false
				extractor.Inputs[0].Default = &value
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_INPUT_DEFAULT",
		},
		{
			name: "unknown source kind",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.Kind = "json"
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown session policy",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.SessionPolicy = "ambient"
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "invalid URL literal segment kind",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.Fetch.URLTemplate.Segments[0] = ir.LiteralTemplateSegment{Kind: "text", Value: "https://example.invalid/"}
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "duplicate output name",
			mutate: func(extractor *ir.Extractor) {
				first := extractor.Output.Members[0].(ir.Field)
				second := first
				second.ID = "output.other"
				extractor.Output.Members = append(extractor.Output.Members, second)
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "duplicate output ID",
			mutate: func(extractor *ir.Extractor) {
				first := extractor.Output.Members[0].(ir.Field)
				second := first
				second.Name = "other"
				extractor.Output.Members = append(extractor.Output.Members, second)
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "negative collection minimum",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.ID, field.Name = "output.rows[].title", "title"
				extractor.Output.Members = append(extractor.Output.Members, ir.Collection{
					Kind: "collection", ID: "output.rows", Name: "rows", Selector: "h1", MinItems: -1,
					OnRowError: "fail", Row: ir.OutputObject{Kind: "object", Members: []ir.OutputMember{field}},
				})
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown collection row policy",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.ID, field.Name = "output.rows[].title", "title"
				extractor.Output.Members = append(extractor.Output.Members, ir.Collection{
					Kind: "collection", ID: "output.rows", Name: "rows", Selector: "h1", OnRowError: "ignore",
					Row: ir.OutputObject{Kind: "object", Members: []ir.OutputMember{field}},
				})
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "empty collection row schema",
			mutate: func(extractor *ir.Extractor) {
				extractor.Output.Members = append(extractor.Output.Members, ir.Collection{
					Kind: "collection", ID: "output.rows", Name: "rows", Selector: "h1", OnRowError: "fail",
					Row: ir.OutputObject{Kind: "object"},
				})
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown field recovery policy",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.OnError = "ignore"
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "required field default",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				value := json.RawMessage(`"fallback"`)
				field.Default = &value
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "malformed field default",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				value := json.RawMessage(`not-json`)
				field.Required = false
				field.Default = &value
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "default policy without default",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Required = false
				field.OnError = "default"
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "warn policy with non-nullable output",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.OnError = "warn"
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown selection match mode",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Selection.Match = "any"
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "malformed field successful type",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.SuccessfulType = typesys.Type{Kind: typesys.KindArray}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "malformed field effective type",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.EffectiveType = typesys.Type{Kind: typesys.KindNullable}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "text source without selection",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.Selection = nil
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{
			name: "attribute source without name",
			mutate: func(extractor *ir.Extractor) {
				field := extractor.Output.Members[0].(ir.Field)
				field.ValueSource = ir.AttributeValueSource{Kind: "attribute"}
				extractor.Output.Members[0] = field
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_IR_INVALID",
		},
		{name: "required input", session: &Session{}, wantCode: "E_INPUT_REQUIRED"},
		{name: "input type", inputs: map[string]any{"id": "wrong"}, session: &Session{}, wantCode: "E_INPUT_TYPE"},
		{name: "required session", inputs: map[string]any{"id": int64(1)}, wantCode: "E_SESSION_REQUIRED"},
		{
			name: "expanded URL",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.Fetch.URLTemplate = ir.Template{Segments: []ir.TemplateSegment{
					ir.LiteralTemplateSegment{Kind: "literal", Value: "ftp://example.invalid/"},
				}}
			},
			inputs: map[string]any{"id": int64(1)}, session: &Session{}, wantCode: "E_URL_INVALID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := compileTestSpec(t, spec)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}
			if tt.mutate != nil {
				tt.mutate(extractor)
			}
			transportCalls := 0
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				transportCalls++
				return &http.Response{
					StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
					Body: io.NopCloser(strings.NewReader(`<h1>ready</h1>`)), Request: request,
				}, nil
			})}
			result, err := Execute(context.Background(), extractor, tt.inputs, Options{HTTPClient: client, Session: tt.session})
			if tt.wantCode == "" {
				if err != nil || result.Value["title"] != "ready" || transportCalls != 1 {
					t.Fatalf("result = %#v, error = %v, transport calls = %d", result, err, transportCalls)
				}
				return
			}
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode {
				t.Fatalf("error = %#v", err)
			}
			if transportCalls != 0 {
				t.Fatalf("transport called %d times before %s", transportCalls, tt.wantCode)
			}
		})
	}
}

func TestPreflightSourceStructure(t *testing.T) {
	valid := ir.Source{
		Kind: "html", SessionPolicy: "optional",
		Fetch: ir.Fetch{URLTemplate: ir.Template{Segments: []ir.TemplateSegment{
			ir.LiteralTemplateSegment{Kind: "literal", Value: "https://example.invalid/"},
			ir.InputTemplateSegment{Kind: "input", Name: "id"},
		}}},
	}
	if err := preflightSourceStructure(valid); err != nil {
		t.Fatalf("valid source preflight error = %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ir.Source)
	}{
		{name: "source kind", mutate: func(source *ir.Source) { source.Kind = "json" }},
		{name: "session policy", mutate: func(source *ir.Source) { source.SessionPolicy = "ambient" }},
		{name: "literal discriminator", mutate: func(source *ir.Source) {
			source.Fetch.URLTemplate.Segments[0] = ir.LiteralTemplateSegment{Kind: "text"}
		}},
		{name: "input discriminator", mutate: func(source *ir.Source) {
			source.Fetch.URLTemplate.Segments[1] = ir.InputTemplateSegment{Kind: "parameter", Name: "id"}
		}},
		{name: "unknown segment", mutate: func(source *ir.Source) { source.Fetch.URLTemplate.Segments[1] = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := valid
			source.Fetch.URLTemplate.Segments = append([]ir.TemplateSegment(nil), valid.Fetch.URLTemplate.Segments...)
			tt.mutate(&source)
			var execution *ExecutionError
			if err := preflightSourceStructure(source); !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" {
				t.Fatalf("preflight error = %#v", err)
			}
		})
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

func cookieNames(cookies []*http.Cookie) map[string]bool {
	present := make(map[string]bool, len(cookies))
	for _, cookie := range cookies {
		present[cookie.Name] = true
	}
	return present
}

type trackingBody struct {
	reader io.Reader
	closed bool
}

func (body *trackingBody) Read(buffer []byte) (int, error) { return body.reader.Read(buffer) }
func (body *trackingBody) Close() error {
	body.closed = true
	return nil
}

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

type contextReader struct{ ctx context.Context }

func (reader contextReader) Read([]byte) (int, error) {
	<-reader.ctx.Done()
	return 0, reader.ctx.Err()
}

func compileHTTPRuntimeSpec(t *testing.T, target string) *ir.Extractor {
	t.Helper()
	path := compileTestSpec(t, fmt.Sprintf(`extractor "http-cleanup" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`, target))
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	return extractor
}

func TestExecuteHTTPClosesResponseBodies(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		status     string
		reader     io.Reader
		maxBody    int64
		wantCode   string
	}{
		{name: "success", statusCode: http.StatusOK, status: "200 OK", reader: strings.NewReader(`<h1>ok</h1>`)},
		{name: "status error", statusCode: http.StatusInternalServerError, status: "500 Internal Server Error", reader: strings.NewReader("error"), wantCode: "E_HTTP_STATUS"},
		{name: "read error", statusCode: http.StatusOK, status: "200 OK", reader: errorReader{err: io.ErrUnexpectedEOF}, wantCode: "E_HTTP_READ"},
		{name: "body too large", statusCode: http.StatusOK, status: "200 OK", reader: strings.NewReader("12345"), maxBody: 4, wantCode: "E_HTTP_BODY_TOO_LARGE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackingBody{reader: tt.reader}
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.statusCode, Status: tt.status, Header: make(http.Header), Body: body, Request: request}, nil
			})}
			result, err := Execute(context.Background(), compileHTTPRuntimeSpec(t, "https://example.invalid/"), nil, Options{HTTPClient: client, MaxResponseBytes: tt.maxBody})
			if !body.closed {
				t.Fatal("response body was not closed")
			}
			if tt.wantCode == "" {
				if err != nil || result.Value["title"] != "ok" {
					t.Fatalf("result = %#v, error = %v", result, err)
				}
				return
			}
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestExecuteHTTPClosesBodyAfterReadCancellation(t *testing.T) {
	var body *trackingBody
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body = &trackingBody{reader: contextReader{ctx: request.Context()}}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: body, Request: request}, nil
	})}
	_, err := Execute(context.Background(), compileHTTPRuntimeSpec(t, "https://example.invalid/"), nil, Options{HTTPClient: client, RequestTimeout: 10 * time.Millisecond})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_HTTP_READ" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %#v", err)
	}
	if body == nil || !body.closed {
		t.Fatal("canceled response body was not closed")
	}
}

func TestExecuteHTTPRedirectPolicyOrderAndBodyCleanup(t *testing.T) {
	var bodies []*trackingBody
	var events []string
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			events = append(events, "request:"+request.URL.Path)
			if request.URL.Path == "/start" {
				body := &trackingBody{reader: strings.NewReader("")}
				bodies = append(bodies, body)
				return &http.Response{
					StatusCode: http.StatusFound, Status: "302 Found", Header: http.Header{"Location": []string{"/end"}}, Body: body, Request: request,
				}, nil
			}
			body := &trackingBody{reader: strings.NewReader(`<h1>redirected</h1>`)}
			bodies = append(bodies, body)
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: body, Request: request}, nil
		}),
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			events = append(events, "client-policy:"+request.URL.Path)
			return nil
		},
	}
	result, err := Execute(context.Background(), compileHTTPRuntimeSpec(t, "https://example.invalid/start"), nil, Options{
		HTTPClient: client,
		URLPolicy: func(_ context.Context, target *url.URL) error {
			events = append(events, "url-policy:"+target.Path)
			return nil
		},
	})
	if err != nil || result.Value["title"] != "redirected" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	wantEvents := []string{"url-policy:/start", "request:/start", "url-policy:/end", "client-policy:/end", "request:/end"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %v, want %v", events, wantEvents)
	}
	if len(bodies) != 2 || !bodies[0].closed || !bodies[1].closed {
		t.Fatalf("response bodies = %#v", bodies)
	}
}

func TestExecuteHTTPRedirectRejectionClosesBodyBeforeClientPolicy(t *testing.T) {
	body := &trackingBody{reader: strings.NewReader("")}
	clientPolicyCalled := false
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound, Status: "302 Found", Header: http.Header{"Location": []string{"/blocked"}}, Body: body, Request: request,
			}, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			clientPolicyCalled = true
			return nil
		},
	}
	blocked := errors.New("redirect blocked")
	_, err := Execute(context.Background(), compileHTTPRuntimeSpec(t, "https://example.invalid/start"), nil, Options{
		HTTPClient: client,
		URLPolicy: func(_ context.Context, target *url.URL) error {
			if target.Path == "/blocked" {
				return blocked
			}
			return nil
		},
	})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" || !errors.Is(err, blocked) {
		t.Fatalf("error = %#v", err)
	}
	if !body.closed || clientPolicyCalled {
		t.Fatalf("body closed = %v, client policy called = %v", body.closed, clientPolicyCalled)
	}
}

func TestExecuteHTTPSessionHeadersFollowRedirectSecurityRules(t *testing.T) {
	type observation struct {
		host          string
		authorization bool
		cookie        bool
		trace         bool
	}
	tests := []struct {
		name          string
		destination   string
		wantSensitive bool
	}{
		{name: "same host", destination: "https://example.invalid/end", wantSensitive: true},
		{name: "subdomain", destination: "https://sub.example.invalid/end", wantSensitive: true},
		{name: "different domain", destination: "https://other.invalid/end", wantSensitive: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var observations []observation
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				observations = append(observations, observation{
					host: request.URL.Host, authorization: request.Header.Get("Authorization") != "",
					cookie: request.Header.Get("Cookie") != "", trace: request.Header.Get("X-Trace") != "",
				})
				if request.URL.Path == "/start" {
					return &http.Response{
						StatusCode: http.StatusFound, Status: "302 Found", Header: http.Header{"Location": []string{tt.destination}},
						Body: io.NopCloser(strings.NewReader("")), Request: request,
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
					Body: io.NopCloser(strings.NewReader(`<h1>redirected</h1>`)), Request: request,
				}, nil
			})}
			path := compileTestSpec(t, `extractor "redirect-session" version=1 {
  source "html" {
    fetch mode="http" url="https://example.invalid/start"
    session policy="optional"
  }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}
			result, err := Execute(context.Background(), extractor, nil, Options{
				HTTPClient: client,
				Session: &Session{
					Headers: http.Header{"Authorization": []string{"Bearer test-value"}, "X-Trace": []string{"test-value"}},
					Cookies: []*http.Cookie{{Name: "session", Value: "test-value"}},
				},
			})
			if err != nil || result.Value["title"] != "redirected" {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
			if len(observations) != 2 {
				t.Fatalf("request count = %d", len(observations))
			}
			initial, redirected := observations[0], observations[1]
			if !initial.authorization || !initial.cookie || !initial.trace || redirected.authorization != tt.wantSensitive || redirected.cookie != tt.wantSensitive || !redirected.trace {
				t.Fatalf("header presence initial=%+v redirected=%+v", initial, redirected)
			}
		})
	}
}

func TestExecuteHTTPCookieJarAppliesRedirectScope(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	root := &url.URL{Scheme: "https", Host: "example.invalid", Path: "/"}
	subdomain := &url.URL{Scheme: "https", Host: "sub.example.invalid", Path: "/"}
	jar.SetCookies(root, []*http.Cookie{
		{Name: "host", Value: "test-value", Path: "/"},
		{Name: "domain", Value: "test-value", Domain: "example.invalid", Path: "/"},
	})
	jar.SetCookies(subdomain, []*http.Cookie{{Name: "subdomain", Value: "test-value", Path: "/"}})

	var observations []map[string]bool
	client := &http.Client{
		Jar: jar,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			observations = append(observations, cookieNames(request.Cookies()))
			if request.URL.Path == "/start" {
				return &http.Response{
					StatusCode: http.StatusFound, Status: "302 Found",
					Header: http.Header{"Location": []string{"https://sub.example.invalid/end"}},
					Body:   io.NopCloser(strings.NewReader("")), Request: request,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(`<h1>redirected</h1>`)), Request: request,
			}, nil
		}),
	}
	path := compileTestSpec(t, `extractor "redirect-jar" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/start" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	result, err := Execute(context.Background(), extractor, nil, Options{
		HTTPClient: client,
		URLPolicy:  func(context.Context, *url.URL) error { return nil },
	})
	if err != nil || result.Value["title"] != "redirected" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	want := []map[string]bool{
		{"host": true, "domain": true},
		{"domain": true, "subdomain": true},
	}
	if !reflect.DeepEqual(observations, want) {
		t.Fatalf("cookie names = %#v, want %#v", observations, want)
	}
}

func TestExecuteHTTPClientJarPersistsResponseCookies(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	root := &url.URL{Scheme: "https", Host: "example.invalid", Path: "/"}
	subdomain := &url.URL{Scheme: "https", Host: "sub.example.invalid", Path: "/"}
	var observations []map[string]bool
	client := &http.Client{
		Jar: jar,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			observations = append(observations, cookieNames(request.Cookies()))
			if request.URL.Path == "/start" {
				return &http.Response{
					StatusCode: http.StatusFound, Status: "302 Found",
					Header: http.Header{
						"Location":   []string{"https://sub.example.invalid/end"},
						"Set-Cookie": []string{"root-only=test-value; Path=/", "shared=test-value; Domain=example.invalid; Path=/"},
					},
					Body: io.NopCloser(strings.NewReader("")), Request: request,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK",
				Header: http.Header{"Set-Cookie": []string{"subdomain-only=test-value; Path=/"}},
				Body:   io.NopCloser(strings.NewReader(`<h1>redirected</h1>`)), Request: request,
			}, nil
		}),
	}
	path := compileTestSpec(t, `extractor "redirect-set-cookie" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/start" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	result, err := Execute(context.Background(), extractor, nil, Options{
		HTTPClient: client,
		URLPolicy:  func(context.Context, *url.URL) error { return nil },
	})
	if err != nil || result.Value["title"] != "redirected" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	wantRequests := []map[string]bool{{}, {"shared": true}}
	if !reflect.DeepEqual(observations, wantRequests) {
		t.Fatalf("request cookie names = %#v, want %#v", observations, wantRequests)
	}
	wantRoot := map[string]bool{"root-only": true, "shared": true}
	wantSubdomain := map[string]bool{"shared": true, "subdomain-only": true}
	if got := cookieNames(jar.Cookies(root)); !reflect.DeepEqual(got, wantRoot) {
		t.Fatalf("root cookie names = %#v, want %#v", got, wantRoot)
	}
	if got := cookieNames(jar.Cookies(subdomain)); !reflect.DeepEqual(got, wantSubdomain) {
		t.Fatalf("subdomain cookie names = %#v, want %#v", got, wantSubdomain)
	}
}

func TestExecuteHTTPCustomRedirectCanStripSessionHeaders(t *testing.T) {
	type observation struct {
		authorization bool
		cookie        bool
		trace         bool
	}
	var observations []observation
	redirectCalls := 0
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			observations = append(observations, observation{
				authorization: request.Header.Get("Authorization") != "",
				cookie:        request.Header.Get("Cookie") != "",
				trace:         request.Header.Get("X-Trace") != "",
			})
			if request.URL.Path == "/start" {
				return &http.Response{
					StatusCode: http.StatusFound, Status: "302 Found", Header: http.Header{"Location": []string{"/end"}},
					Body: io.NopCloser(strings.NewReader("")), Request: request,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(`<h1>redirected</h1>`)), Request: request,
			}, nil
		}),
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			redirectCalls++
			request.Header.Del("Authorization")
			request.Header.Del("Cookie")
			request.Header.Del("X-Trace")
			return nil
		},
	}
	path := compileTestSpec(t, `extractor "redirect-mutation" version=1 {
  source "html" { fetch mode="http" url="https://example.invalid/start"; session policy="optional" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	policyCalls := 0
	result, err := Execute(context.Background(), extractor, nil, Options{
		HTTPClient: client,
		Session: &Session{
			Headers: http.Header{"Authorization": []string{"Bearer test-value"}, "X-Trace": []string{"test-value"}},
			Cookies: []*http.Cookie{{Name: "session", Value: "test-value"}},
		},
		URLPolicy: func(context.Context, *url.URL) error {
			policyCalls++
			return nil
		},
	})
	if err != nil || result.Value["title"] != "redirected" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	want := []observation{{authorization: true, cookie: true, trace: true}, {}}
	if !reflect.DeepEqual(observations, want) || redirectCalls != 1 || policyCalls != 2 {
		t.Fatalf("headers = %+v, redirect calls = %d, policy calls = %d", observations, redirectCalls, policyCalls)
	}
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
