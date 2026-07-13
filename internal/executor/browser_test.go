package executor

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/compiler"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

type fakeElement struct{ id string }
type fakeBrowser struct {
	calls []string
	js    any
}

func (f *fakeBrowser) Navigate(_ context.Context, url string, _ BrowserNavigateOptions) error {
	f.calls = append(f.calls, "navigate:"+url)
	return nil
}
func (f *fakeBrowser) WaitFor(_ context.Context, s, state string, _ time.Duration) error {
	f.calls = append(f.calls, "wait:"+s+":"+state)
	return nil
}
func (f *fakeBrowser) Click(_ context.Context, s string, _ time.Duration) error {
	f.calls = append(f.calls, "click:"+s)
	return nil
}
func (f *fakeBrowser) Fill(_ context.Context, s, v string, _ time.Duration) error {
	f.calls = append(f.calls, "fill:"+s+":"+v)
	return nil
}
func (f *fakeBrowser) Press(_ context.Context, s, k string, _ time.Duration) error {
	f.calls = append(f.calls, "press:"+s+":"+k)
	return nil
}
func (f *fakeBrowser) Scroll(_ context.Context, x, y float64) error {
	f.calls = append(f.calls, "scroll")
	return nil
}
func (f *fakeBrowser) WaitForNetworkIdle(_ context.Context, _, _ time.Duration) error {
	f.calls = append(f.calls, "idle")
	return nil
}
func (f *fakeBrowser) Evaluate(_ context.Context, source string, options BrowserEvaluateOptions) (any, error) {
	f.calls = append(f.calls, "js")
	if source == "bad" {
		return nil, errors.New("bad js")
	}
	if options.Scope != nil {
		return map[string]any{"row": "ok"}, nil
	}
	return f.js, nil
}
func (f *fakeBrowser) QueryAll(_ context.Context, scope BrowserElement, selector string) ([]BrowserElement, error) {
	switch selector {
	case "h1", ".race-detail":
		return []BrowserElement{fakeElement{"root"}}, nil
	case "table.entries tbody tr":
		return []BrowserElement{fakeElement{"r1"}, fakeElement{"r2"}}, nil
	case ".number", ".horse-name a", ".sex":
		return []BrowserElement{fakeElement{selector}}, nil
	default:
		return nil, nil
	}
}
func (f *fakeBrowser) Text(_ context.Context, e BrowserElement) (string, error) {
	id := e.(fakeElement).id
	switch id {
	case ".number":
		return "1", nil
	case ".horse-name a":
		return "Horse", nil
	case ".sex":
		return "牡", nil
	default:
		return "Title", nil
	}
}
func (f *fakeBrowser) HTML(_ context.Context, _ BrowserElement) (string, error) {
	return "<b>x</b>", nil
}
func (f *fakeBrowser) Attribute(_ context.Context, e BrowserElement, name string) (string, bool, error) {
	if name == "href" {
		return "/horse/123/", true, nil
	}
	return "", false, nil
}

func TestExecuteBrowser(t *testing.T) {
	extractor, diags := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diags.HasErrors() {
		t.Fatalf("compile: %v", diags)
	}
	browser := &fakeBrowser{js: map[string]any{"id": "race"}}
	result, err := Execute(context.Background(), extractor, map[string]any{"race_id": "42"}, Options{Browser: browser, AllowJavaScript: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Partial {
		t.Fatalf("unexpected partial: %#v", result)
	}
	if got := result.Value["title"]; got != "Title" {
		t.Fatalf("title=%v", got)
	}
	entries := result.Value["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}
	if !reflect.DeepEqual(browser.calls[:2], []string{"navigate:https://example.com/race/42", "wait:.race-detail:visible"}) {
		t.Fatalf("calls=%v", browser.calls)
	}
}

func TestExecuteBrowserRequiresJavaScriptOptIn(t *testing.T) {
	extractor, _ := compiler.CompileFile("../../fixtures/valid/browser-js.kdl")
	_, err := Execute(context.Background(), extractor, map[string]any{}, Options{Browser: &fakeBrowser{}})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_JAVASCRIPT_DISABLED" {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteBrowserPreflightsExternalTransformsBeforeAcquire(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-external" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  transform "decorate" input="string" output="string" { external symbol="decorate" }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
    apply "decorate"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	browser := &leasedFakeBrowser{}
	_, err := Execute(context.Background(), extractor, nil, Options{Browser: browser})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_EXTERNAL_TRANSFORM_MISSING" {
		t.Fatalf("error = %#v", err)
	}
	if browser.acquired != 0 || browser.released != 0 || len(browser.calls) != 0 {
		t.Fatalf("browser used before preflight: acquired=%d released=%d calls=%v", browser.acquired, browser.released, browser.calls)
	}

	result, err := Execute(context.Background(), extractor, nil, Options{
		Browser: browser,
		ExternalTransforms: map[string]ExternalTransform{
			"decorate": func(_ context.Context, input any) (any, error) { return "[" + input.(string) + "]", nil },
		},
	})
	if err != nil || result.Value["title"] != "[Title]" || browser.acquired != 1 || browser.released != 1 {
		t.Fatalf("result = %#v, error = %v, acquired=%d released=%d", result, err, browser.acquired, browser.released)
	}
}

func TestExecuteBrowserPreflightsInputsAndSessionBeforeAcquire(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-preflight" version=1 {
  source "html" {
    fetch mode="browser" url="https://example.invalid/{id}"
    session policy="required"
  }
  input "id" type="int" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	tests := []struct {
		name     string
		inputs   map[string]any
		session  *Session
		wantCode string
	}{
		{name: "required input", session: &Session{}, wantCode: "E_INPUT_REQUIRED"},
		{name: "input type", inputs: map[string]any{"id": "wrong"}, session: &Session{}, wantCode: "E_INPUT_TYPE"},
		{name: "required session", inputs: map[string]any{"id": int64(1)}, wantCode: "E_SESSION_REQUIRED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser := &leasedFakeBrowser{}
			_, err := Execute(context.Background(), extractor, tt.inputs, Options{Browser: browser, Session: tt.session})
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode {
				t.Fatalf("error = %#v", err)
			}
			if browser.acquired != 0 || browser.released != 0 || len(browser.calls) != 0 {
				t.Fatalf("browser used before preflight: acquired=%d released=%d calls=%v", browser.acquired, browser.released, browser.calls)
			}
		})
	}
}

func TestExecuteBrowserPreflightsMalformedOutputBeforeAcquire(t *testing.T) {
	const spec = `extractor "browser-output-preflight" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  collection "rows" { select ".rows"; field "value" type="string" required=#true { select ".value"; value "text" } }
}`
	tests := []struct {
		name     string
		mutate   func(*ir.Extractor)
		wantCode string
	}{
		{name: "success"},
		{
			name: "nested selector",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				field.Selection.Selector = "["
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_SELECTOR_INVALID",
		},
		{
			name: "unknown output member",
			mutate: func(extractor *ir.Extractor) {
				extractor.Output.Members = []ir.OutputMember{nil}
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown value source",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				field.ValueSource = nil
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "unknown builtin transform",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{Target: ir.BuiltinTarget{Kind: "builtin", Name: "missing"}}}
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_TRANSFORM",
		},
		{
			name: "missing declared transform",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{Target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:missing"}}}
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_TRANSFORM_MISSING",
		},
		{
			name: "malformed transform argument",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				field.Transforms = []ir.TransformCall{{
					Target:         ir.BuiltinTarget{Kind: "builtin", Name: "prepend"},
					NamedArguments: []ir.NamedArgument{{Name: "value", Value: json.RawMessage(`not-json`)}},
				}}
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_TRANSFORM",
		},
		{
			name: "malformed match literal",
			mutate: func(extractor *ir.Extractor) {
				stringType := typesys.Primitive("string")
				extractor.Transforms = append(extractor.Transforms, ir.MatchTransform{
					Kind:          "match",
					TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: stringType},
					Cases:         []ir.MatchCase{{When: json.RawMessage(`not-json`), Then: json.RawMessage(`"output"`)}},
					Default:       json.RawMessage(`"fallback"`),
				})
			},
			wantCode: "E_TRANSFORM",
		},
		{
			name: "duplicate nested output name",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				first := collection.Row.Members[0].(ir.Field)
				second := first
				second.ID = collection.ID + "[].other"
				collection.Row.Members = append(collection.Row.Members, second)
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "collection maximum below required minimum",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				maximum := 0
				collection.Required = true
				collection.MinItems = 0
				collection.MaxItems = &maximum
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "nested field default type mismatch",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				value := json.RawMessage(`1`)
				field.Required = false
				field.Default = &value
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "empty collection row schema",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				collection.Row.Members = nil
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "nested field selection match mode",
			mutate: func(extractor *ir.Extractor) {
				collection := extractor.Output.Members[0].(ir.Collection)
				field := collection.Row.Members[0].(ir.Field)
				field.Selection.Match = "last"
				collection.Row.Members[0] = field
				extractor.Output.Members[0] = collection
			},
			wantCode: "E_IR_INVALID",
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
			browser := &leasedFakeBrowser{}
			result, err := Execute(context.Background(), extractor, nil, Options{Browser: browser})
			if tt.wantCode == "" {
				if err != nil || !reflect.DeepEqual(result.Value["rows"], []any{}) || browser.acquired != 1 || browser.released != 1 {
					t.Fatalf("result=%#v error=%v acquired=%d released=%d", result, err, browser.acquired, browser.released)
				}
				return
			}
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode {
				t.Fatalf("error = %#v", err)
			}
			if browser.acquired != 0 || browser.released != 0 || len(browser.calls) != 0 {
				t.Fatalf("browser used before output preflight: acquired=%d released=%d calls=%v", browser.acquired, browser.released, browser.calls)
			}
		})
	}
}

func TestExecuteBrowserPreflightsMalformedWorkflowBeforeAcquire(t *testing.T) {
	const spec = `extractor "browser-workflow-preflight" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/"; workflow { wait-for "#ready" } }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	tests := []struct {
		name     string
		mutate   func(*ir.Extractor)
		wantCode string
	}{
		{name: "success"},
		{
			name: "selector",
			mutate: func(extractor *ir.Extractor) {
				step := extractor.Source.Workflow[0].(ir.WaitForStep)
				step.Selector = "["
				extractor.Source.Workflow[0] = step
			},
			wantCode: "E_SELECTOR_INVALID",
		},
		{
			name: "unknown step",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.Workflow[0] = nil
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "state",
			mutate: func(extractor *ir.Extractor) {
				step := extractor.Source.Workflow[0].(ir.WaitForStep)
				step.State = "moving"
				extractor.Source.Workflow[0] = step
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "timeout",
			mutate: func(extractor *ir.Extractor) {
				step := extractor.Source.Workflow[0].(ir.WaitForStep)
				value := 0
				step.TimeoutMS = &value
				extractor.Source.Workflow[0] = step
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "network idle",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.Workflow[0] = ir.NetworkIdleStep{Kind: "wait-for-network-idle", IdleMS: 0}
			},
			wantCode: "E_IR_INVALID",
		},
		{
			name: "scroll coordinates",
			mutate: func(extractor *ir.Extractor) {
				extractor.Source.Workflow[0] = ir.ScrollStep{Kind: "scroll", X: math.Inf(1), Y: 0}
			},
			wantCode: "E_IR_INVALID",
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
			browser := &leasedFakeBrowser{}
			result, err := Execute(context.Background(), extractor, nil, Options{Browser: browser})
			if tt.wantCode == "" {
				if err != nil || result.Value["title"] != "Title" || browser.acquired != 1 || browser.released != 1 {
					t.Fatalf("result=%#v error=%v acquired=%d released=%d", result, err, browser.acquired, browser.released)
				}
				return
			}
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || execution.Path != "source.workflow[0]" {
				t.Fatalf("error = %#v", err)
			}
			if browser.acquired != 0 || browser.released != 0 || len(browser.calls) != 0 {
				t.Fatalf("browser used before workflow preflight: acquired=%d released=%d calls=%v", browser.acquired, browser.released, browser.calls)
			}
		})
	}
}

type leasedFakeBrowser struct {
	fakeBrowser
	acquired   int
	released   int
	acquireErr error
}

func (f *leasedFakeBrowser) Acquire(context.Context) (func(), error) {
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	f.acquired++
	return func() { f.released++ }, nil
}

func TestExecuteBrowserUsesAdapterLease(t *testing.T) {
	extractor, diags := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diags.HasErrors() {
		t.Fatalf("compile: %v", diags)
	}
	browser := &leasedFakeBrowser{fakeBrowser: fakeBrowser{js: map[string]any{"id": "race"}}}
	_, err := Execute(context.Background(), extractor, map[string]any{"race_id": "42"}, Options{Browser: browser, AllowJavaScript: true})
	if err != nil {
		t.Fatal(err)
	}
	if browser.acquired != 1 || browser.released != 1 {
		t.Fatalf("lease counts acquired=%d released=%d", browser.acquired, browser.released)
	}
}

func TestExecuteBrowserReleasesLeaseOnNavigationFailure(t *testing.T) {
	extractor, diags := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diags.HasErrors() {
		t.Fatalf("compile: %v", diags)
	}
	browser := &failingNavigationBrowser{}
	_, err := Execute(context.Background(), extractor, map[string]any{"race_id": "42"}, Options{Browser: browser, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_BROWSER_NAVIGATE" {
		t.Fatalf("err=%v", err)
	}
	if browser.released != 1 {
		t.Fatalf("release count=%d", browser.released)
	}
}

type failingNavigationBrowser struct {
	fakeBrowser
	released int
}

func (f *failingNavigationBrowser) Acquire(context.Context) (func(), error) {
	return func() { f.released++ }, nil
}

func (f *failingNavigationBrowser) Navigate(context.Context, string, BrowserNavigateOptions) error {
	return errors.New("navigation failed")
}

type leaseTrackingBrowser struct {
	BrowserAdapter
	acquired   int
	released   int
	nilRelease bool
}

func (b *leaseTrackingBrowser) Acquire(context.Context) (func(), error) {
	b.acquired++
	if b.nilRelease {
		return nil, nil
	}
	return func() { b.released++ }, nil
}

func TestExecuteBrowserRejectsNilLeaseRelease(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-nil-release" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	base := &fakeBrowser{}
	browser := &leaseTrackingBrowser{BrowserAdapter: base, nilRelease: true}
	_, err := Execute(context.Background(), extractor, nil, Options{Browser: browser})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_BROWSER_ACQUIRE" {
		t.Fatalf("error = %#v", err)
	}
	if browser.acquired != 1 || browser.released != 0 || len(base.calls) != 0 {
		t.Fatalf("acquired=%d released=%d calls=%v", browser.acquired, browser.released, base.calls)
	}
}

func TestExecuteBrowserReleasesLeaseAfterPostNavigationFailures(t *testing.T) {
	cause := errors.New("adapter failed")
	tests := []struct {
		name     string
		source   string
		adapter  BrowserAdapter
		wantCode string
	}{
		{
			name: "workflow",
			source: `extractor "browser-workflow-release" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/"; workflow { wait-for "#ready" } }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`,
			adapter: &workflowFailureBrowser{operation: "wait-for", cause: cause}, wantCode: "E_BROWSER_WORKFLOW",
		},
		{
			name: "output query",
			source: `extractor "browser-query-release" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`,
			adapter: &readFailureBrowser{operation: "query", err: cause}, wantCode: "E_BROWSER_QUERY",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := compileTestSpec(t, tt.source)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}
			browser := &leaseTrackingBrowser{BrowserAdapter: tt.adapter}
			_, err := Execute(context.Background(), extractor, nil, Options{Browser: browser})
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || !errors.Is(err, cause) {
				t.Fatalf("error = %#v", err)
			}
			if browser.acquired != 1 || browser.released != 1 {
				t.Fatalf("acquired=%d released=%d", browser.acquired, browser.released)
			}
		})
	}
}

type rowRecoveryBrowser struct{ fakeBrowser }

func (f *rowRecoveryBrowser) QueryAll(_ context.Context, scope BrowserElement, selector string) ([]BrowserElement, error) {
	switch selector {
	case ".rows":
		return []BrowserElement{fakeElement{"good-row"}, fakeElement{"bad-row"}}, nil
	case ".value":
		if scope == (fakeElement{"bad-row"}) {
			return nil, nil
		}
		return []BrowserElement{fakeElement{"value"}}, nil
	default:
		return f.fakeBrowser.QueryAll(context.Background(), scope, selector)
	}
}

type readFailureBrowser struct {
	fakeBrowser
	operation string
	err       error
}

type workflowFailureBrowser struct {
	fakeBrowser
	operation string
	cause     error
}

func (f *workflowFailureBrowser) WaitFor(ctx context.Context, selector, state string, timeout time.Duration) error {
	if f.operation == "wait-for" {
		return f.cause
	}
	return f.fakeBrowser.WaitFor(ctx, selector, state, timeout)
}

func (f *workflowFailureBrowser) Click(ctx context.Context, selector string, timeout time.Duration) error {
	if f.operation == "click" {
		return f.cause
	}
	return f.fakeBrowser.Click(ctx, selector, timeout)
}

func (f *workflowFailureBrowser) Fill(ctx context.Context, selector, value string, timeout time.Duration) error {
	if f.operation == "fill" {
		return f.cause
	}
	return f.fakeBrowser.Fill(ctx, selector, value, timeout)
}

func (f *workflowFailureBrowser) Press(ctx context.Context, selector, key string, timeout time.Duration) error {
	if f.operation == "press" {
		return f.cause
	}
	return f.fakeBrowser.Press(ctx, selector, key, timeout)
}

func (f *workflowFailureBrowser) Scroll(ctx context.Context, x, y float64) error {
	if f.operation == "scroll" {
		return f.cause
	}
	return f.fakeBrowser.Scroll(ctx, x, y)
}

func (f *workflowFailureBrowser) WaitForNetworkIdle(ctx context.Context, idle, timeout time.Duration) error {
	if f.operation == "network-idle" {
		return f.cause
	}
	return f.fakeBrowser.WaitForNetworkIdle(ctx, idle, timeout)
}

func (f *workflowFailureBrowser) Evaluate(ctx context.Context, source string, options BrowserEvaluateOptions) (any, error) {
	if f.operation == "evaluate-js" {
		return nil, f.cause
	}
	return f.fakeBrowser.Evaluate(ctx, source, options)
}

func (f *readFailureBrowser) QueryAll(ctx context.Context, scope BrowserElement, selector string) ([]BrowserElement, error) {
	if f.operation == "query" {
		return nil, f.err
	}
	return f.fakeBrowser.QueryAll(ctx, scope, selector)
}

func (f *readFailureBrowser) Text(context.Context, BrowserElement) (string, error) {
	if f.operation == "text" {
		return "", f.err
	}
	return "Title", nil
}

func (f *readFailureBrowser) HTML(context.Context, BrowserElement) (string, error) {
	if f.operation == "html" {
		return "", f.err
	}
	return "<b>x</b>", nil
}

func (f *readFailureBrowser) Attribute(context.Context, BrowserElement, string) (string, bool, error) {
	if f.operation == "attribute" {
		return "", false, f.err
	}
	return "/horse/123/", true, nil
}

func TestExecuteBrowserAcquireFailure(t *testing.T) {
	extractor, diags := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diags.HasErrors() {
		t.Fatalf("compile: %v", diags)
	}
	browser := &leasedFakeBrowser{acquireErr: context.Canceled}
	_, err := Execute(context.Background(), extractor, map[string]any{"race_id": "42"}, Options{Browser: browser, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_BROWSER_ACQUIRE" {
		t.Fatalf("err=%v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire error does not preserve context cancellation: %v", err)
	}
}

func TestExecuteBrowserRejectsNonJSONJavaScriptResult(t *testing.T) {
	extractor, diags := compiler.CompileFile("../../fixtures/valid/browser-js.kdl")
	if diags.HasErrors() {
		t.Fatalf("compile: %v", diags)
	}
	browser := &fakeBrowser{js: make(chan int)}
	_, err := Execute(context.Background(), extractor, map[string]any{}, Options{Browser: browser, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_JAVASCRIPT_RESULT_TYPE" {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteBrowserValidatesJSONNumberResult(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-number" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "value" type="unknown" required=#true {
    evaluate-js "() => 1" scope="document" returns="unknown"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	tests := []struct {
		name    string
		value   json.Number
		wantErr bool
	}{
		{name: "finite", value: json.Number("1.25")},
		{name: "not a number", value: json.Number("NaN"), wantErr: true},
		{name: "infinite exponent", value: json.Number("1e1000"), wantErr: true},
		{name: "invalid JSON lexeme", value: json.Number("01"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{js: tt.value}, AllowJavaScript: true})
			if !tt.wantErr {
				if err != nil || result.Value["value"] != tt.value {
					t.Fatalf("result = %#v, error = %v", result, err)
				}
				return
			}
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != "E_JAVASCRIPT_RESULT_TYPE" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestExecuteBrowserValidatesDeclaredJavaScriptReturnType(t *testing.T) {
	tests := []struct {
		name    string
		returns string
		value   any
		want    any
		wantErr bool
	}{
		{name: "string", returns: "string", value: "ok", want: "ok"},
		{name: "string mismatch", returns: "string", value: true, wantErr: true},
		{name: "integer from browser number", returns: "int", value: float64(42), want: int64(42)},
		{name: "fractional integer", returns: "int", value: 1.5, wantErr: true},
		{name: "signed integer boundary", returns: "i8", value: float64(127), want: int8(127)},
		{name: "signed integer overflow", returns: "i8", value: float64(128), wantErr: true},
		{name: "unsigned integer", returns: "u8", value: json.Number("255"), want: uint8(255)},
		{name: "negative unsigned integer", returns: "u8", value: float64(-1), wantErr: true},
		{name: "maximum unsigned integer", returns: "u64", value: json.Number("18446744073709551615"), want: uint64(18446744073709551615)},
		{name: "float32", returns: "f32", value: 1.5, want: float32(1.5)},
		{name: "float32 overflow", returns: "f32", value: json.Number("3.5e38"), wantErr: true},
		{name: "nullable", returns: "int?", value: nil, want: nil},
		{name: "integer array", returns: "int[]", value: []any{float64(1), json.Number("2")}, want: []any{int64(1), int64(2)}},
		{name: "array element mismatch", returns: "int[]", value: []any{float64(1), "2"}, wantErr: true},
		{name: "object", returns: "object", value: map[string]any{"ok": true}, want: map[string]any{"ok": true}},
		{name: "object mismatch", returns: "object", value: []any{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := compileTestSpec(t, `extractor "browser-return" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "value" type="`+tt.returns+`" required=#true {
    evaluate-js "() => null" scope="document" returns="`+tt.returns+`"
  }
}`)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}

			result, err := Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{js: tt.value}, AllowJavaScript: true})
			if tt.wantErr {
				var execution *ExecutionError
				if !errors.As(err, &execution) || execution.Code != "E_JAVASCRIPT_RESULT_TYPE" {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute error = %v", err)
			}
			if got := result.Value["value"]; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("value = %#v (%T), want %#v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestExecuteBrowserMissingAndFieldRecovery(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-recovery" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "missing" type="string" required=#false {
    select ".missing" match="first"
    value "text"
  }
  field "missing_default" type="string" required=#false default="fallback" {
    select ".missing" match="first"
    value "text"
    on-error "default"
  }
  field "warned" type="string" required=#false {
    evaluate-js "bad" scope="document" returns="string"
    on-error "warn"
  }
  field "recovered_default" type="string" required=#false default="fallback" {
    evaluate-js "bad" scope="document" returns="string"
    on-error "default"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	result, err := Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{}, AllowJavaScript: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["missing"] != nil || result.Value["missing_default"] != "fallback" || result.Value["warned"] != nil || result.Value["recovered_default"] != "fallback" {
		t.Fatalf("value = %#v", result.Value)
	}
	if !result.Partial || len(result.Warnings) != 2 || result.Warnings[0].Code != "W_ERROR_RECOVERED" || result.Warnings[0].Path != "output.warned" || result.Warnings[1].Code != "W_PARTIAL_EXTRACTION" {
		t.Fatalf("recovery = partial:%v warnings:%#v", result.Partial, result.Warnings)
	}
}

func TestExecuteBrowserNormalizesNumericFieldDefaults(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-numeric-defaults" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "missing_count" type="int" required=#false default=7 {
    select ".missing"; value "text"; apply "parse-int" as="int"; on-error "default"
  }
  field "recovered_count" type="int" required=#false default=8 {
    evaluate-js "bad" scope="document" returns="string"; apply "parse-int" as="int"; on-error "default"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	result, err := Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{}, AllowJavaScript: true})
	if err != nil || result.Value["missing_count"] != int64(7) || result.Value["recovered_count"] != int64(8) || !result.Partial {
		t.Fatalf("result = %#v, error = %v", result, err)
	}

	field := extractor.Output.Members[1].(ir.Field)
	outOfRange := json.RawMessage(`9223372036854775808`)
	field.Default = &outOfRange
	extractor.Output.Members[1] = field
	_, err = Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{}, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "output.recovered_count" {
		t.Fatalf("malformed default error = %#v", err)
	}
}

func TestExecuteBrowserRequiredMissingIsNotRecovered(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-required-missing" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "missing" type="string" required=#true {
    select ".missing" match="first"
    value "text"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	_, err := Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{}})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_REQUIRED_VALUE_MISSING" || execution.Path != "output.missing" {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteBrowserWrapsInvalidRecoveryDefault(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-invalid-default" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "value" type="string" required=#false default="fallback" {
    evaluate-js "bad" scope="document" returns="string"
    on-error "default"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	field := extractor.Output.Members[0].(ir.Field)
	invalid := json.RawMessage(`not-json`)
	field.Default = &invalid
	extractor.Output.Members[0] = field

	_, err := Execute(context.Background(), extractor, nil, Options{Browser: &fakeBrowser{}, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "output.value" || execution.Cause == nil {
		t.Fatalf("error = %#v", err)
	}
}

func TestExecuteBrowserCollectionRowRecovery(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-row-recovery" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  collection "rows" min-items=1 on-row-error="skip" {
    select ".rows"
    field "value" type="string" required=#true {
      select ".value" match="one"
      value "text"
    }
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	result, err := Execute(context.Background(), extractor, nil, Options{Browser: &rowRecoveryBrowser{}})
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := result.Value["rows"].([]any)
	if !ok || len(rows) != 1 || rows[0].(map[string]any)["value"] != "Title" {
		t.Fatalf("rows = %#v", result.Value["rows"])
	}
	if !result.Partial || len(result.Warnings) != 2 || result.Warnings[0].Code != "W_ROW_SKIPPED" || result.Warnings[0].Path != "output.rows" || result.Warnings[0].Row == nil || *result.Warnings[0].Row != 1 || result.Warnings[1].Code != "W_PARTIAL_EXTRACTION" {
		t.Fatalf("recovery = partial:%v warnings:%#v", result.Partial, result.Warnings)
	}
}

func TestExecuteBrowserCollectionCardinality(t *testing.T) {
	tests := []struct {
		name       string
		collection string
		browser    BrowserAdapter
		wantText   string
	}{
		{
			name: "required empty collection",
			collection: `collection "rows" required=#true {
    select ".absent"
    field "value" type="string" required=#true { select ".value"; value "text" }
  }`,
			browser:  &fakeBrowser{},
			wantText: "minimum is 1",
		},
		{
			name: "maximum exceeded",
			collection: `collection "rows" max-items=1 {
    select "table.entries tbody tr"
    field "value" type="string" required=#true { select ".number"; value "text" }
  }`,
			browser:  &fakeBrowser{},
			wantText: "maximum is 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := compileTestSpec(t, `extractor "browser-cardinality" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  `+tt.collection+`
}`)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}

			_, err := Execute(context.Background(), extractor, nil, Options{Browser: tt.browser})
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != "E_COLLECTION_CARDINALITY" || execution.Path != "output.rows" || !strings.Contains(execution.Message, tt.wantText) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestExecuteBrowserValueSourceReads(t *testing.T) {
	tests := []struct {
		name        string
		valueSource string
		want        string
	}{
		{name: "text", valueSource: `value "text"`, want: "Title"},
		{name: "HTML", valueSource: `value "html"`, want: "<b>x</b>"},
		{name: "attribute", valueSource: `value "attr" name="href"`, want: "/horse/123/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := compileTestSpec(t, `extractor "browser-read" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "value" type="string" required=#true {
    select "h1" match="one"
    `+tt.valueSource+`
  }
}`)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}

			result, err := Execute(context.Background(), extractor, nil, Options{Browser: &readFailureBrowser{}})
			if err != nil || result.Value["value"] != tt.want {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
		})
	}
}

func TestExecuteBrowserValueSourceFailures(t *testing.T) {
	tests := []struct {
		name        string
		valueSource string
		operation   string
		cause       error
		wantCode    string
	}{
		{name: "query", valueSource: `value "text"`, operation: "query", cause: errors.New("query failed"), wantCode: "E_BROWSER_QUERY"},
		{name: "query timeout", valueSource: `value "text"`, operation: "query", cause: context.DeadlineExceeded, wantCode: "E_TIMEOUT"},
		{name: "query cancellation", valueSource: `value "text"`, operation: "query", cause: context.Canceled, wantCode: "E_BROWSER_QUERY"},
		{name: "text", valueSource: `value "text"`, operation: "text", cause: errors.New("text failed"), wantCode: "E_BROWSER_READ"},
		{name: "HTML timeout", valueSource: `value "html"`, operation: "html", cause: context.DeadlineExceeded, wantCode: "E_TIMEOUT"},
		{name: "attribute", valueSource: `value "attr" name="href"`, operation: "attribute", cause: errors.New("attribute failed"), wantCode: "E_BROWSER_READ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := compileTestSpec(t, `extractor "browser-read-failure" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "value" type="string" required=#true {
    select "h1" match="one"
    `+tt.valueSource+`
  }
}`)
			extractor, diagnostics := compiler.CompileFile(path)
			if diagnostics.HasErrors() {
				t.Fatalf("compile diagnostics = %#v", diagnostics)
			}

			_, err := Execute(context.Background(), extractor, nil, Options{Browser: &readFailureBrowser{operation: tt.operation, err: tt.cause}})
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || execution.Path != "output.value" || !errors.Is(err, tt.cause) {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestExecuteBrowserRecoversQueryFailure(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-query-recovery" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "value" type="string" required=#false {
    select "h1" match="one"
    value "text"
    on-error "warn"
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	cause := errors.New("query failed")
	result, err := Execute(context.Background(), extractor, nil, Options{Browser: &readFailureBrowser{operation: "query", err: cause}})
	if err != nil || result.Value["value"] != nil || !result.Partial || len(result.Warnings) != 2 || result.Warnings[0].Code != "W_ERROR_RECOVERED" || !strings.Contains(result.Warnings[0].Message, cause.Error()) {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestExecuteBrowserCollectionQueryFailure(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-collection-query" version=1 {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  collection "rows" {
    select ".rows"
    field "value" type="string" required=#true { select ".value"; value "text" }
  }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	cause := context.Canceled
	_, err := Execute(context.Background(), extractor, nil, Options{Browser: &readFailureBrowser{operation: "query", err: cause}})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_BROWSER_QUERY" || execution.Path != "output.rows" || !errors.Is(err, cause) {
		t.Fatalf("error = %#v", err)
	}
}

func TestExecuteBrowserURLPolicyRejectsBeforeAcquireOrNavigate(t *testing.T) {
	extractor, diagnostics := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	browser := &leasedFakeBrowser{fakeBrowser: fakeBrowser{js: map[string]any{"id": "race"}}}
	_, err := Execute(context.Background(), extractor, map[string]any{"race_id": "42"}, Options{
		Browser:         browser,
		AllowJavaScript: true,
		URLPolicy: func(context.Context, *url.URL) error {
			return errors.New("blocked")
		},
	})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" {
		t.Fatalf("error = %v", err)
	}
	if browser.acquired != 0 || len(browser.calls) != 0 {
		t.Fatalf("browser used before policy: acquired=%d calls=%v", browser.acquired, browser.calls)
	}
}

type timeoutWorkflowBrowser struct{ fakeBrowser }

func (f *timeoutWorkflowBrowser) WaitFor(context.Context, string, string, time.Duration) error {
	return context.DeadlineExceeded
}

func TestExecuteBrowserWorkflowTimeoutUsesStableCode(t *testing.T) {
	extractor, diagnostics := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	browser := &timeoutWorkflowBrowser{fakeBrowser: fakeBrowser{js: map[string]any{"id": "race"}}}
	_, err := Execute(context.Background(), extractor, map[string]any{"race_id": "42"}, Options{Browser: browser, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_TIMEOUT" {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteBrowserWorkflowOperations(t *testing.T) {
	path := compileTestSpec(t, `extractor "browser-workflow" version=1 {
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
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
	extractor, diagnostics := compiler.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	browser := &workflowFailureBrowser{}
	result, err := Execute(context.Background(), extractor, nil, Options{Browser: browser, AllowJavaScript: true})
	if err != nil || result.Value["title"] != "Title" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	wantCalls := []string{
		"navigate:https://example.invalid/", "wait:#ready:attached", "click:button", "fill:input:value",
		"press:input:Enter", "scroll", "idle", "js",
	}
	if !reflect.DeepEqual(browser.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", browser.calls, wantCalls)
	}
}

func TestExecuteBrowserWorkflowOperationFailures(t *testing.T) {
	operations := []struct {
		name string
		step string
	}{
		{name: "wait-for", step: `wait-for "#ready"`},
		{name: "click", step: `click "button"`},
		{name: "fill", step: `fill "input" "value"`},
		{name: "press", step: `press "input" "Enter"`},
		{name: "scroll", step: `scroll 1.5 -2`},
		{name: "network-idle", step: `wait-for-network-idle idle-ms=250`},
		{name: "evaluate-js", step: `evaluate-js "() => null"`},
	}
	causes := []struct {
		name     string
		cause    error
		wantCode string
	}{
		{name: "failure", cause: errors.New("workflow failed"), wantCode: "E_BROWSER_WORKFLOW"},
		{name: "timeout", cause: context.DeadlineExceeded, wantCode: "E_TIMEOUT"},
		{name: "cancellation", cause: context.Canceled, wantCode: "E_BROWSER_WORKFLOW"},
	}
	for _, operation := range operations {
		for _, failure := range causes {
			t.Run(operation.name+"/"+failure.name, func(t *testing.T) {
				path := compileTestSpec(t, `extractor "browser-workflow-failure" version=1 {
  source "html" {
    fetch mode="browser" url="https://example.invalid/"
    workflow { `+operation.step+` }
  }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)
				extractor, diagnostics := compiler.CompileFile(path)
				if diagnostics.HasErrors() {
					t.Fatalf("compile diagnostics = %#v", diagnostics)
				}
				browser := &workflowFailureBrowser{operation: operation.name, cause: failure.cause}
				_, err := Execute(context.Background(), extractor, nil, Options{Browser: browser, AllowJavaScript: true})
				var execution *ExecutionError
				if !errors.As(err, &execution) || execution.Code != failure.wantCode || execution.Path != "source.workflow[0]" || !errors.Is(err, failure.cause) {
					t.Fatalf("error = %#v", err)
				}
			})
		}
	}
}

type canceledWorkflowBrowser struct{ leasedFakeBrowser }

func (f *canceledWorkflowBrowser) WaitFor(ctx context.Context, _ string, _ string, _ time.Duration) error {
	return ctx.Err()
}

func TestExecuteBrowserWorkflowPreservesParentCancellation(t *testing.T) {
	extractor, diagnostics := compiler.CompileFile("../../fixtures/valid/race-detail.kdl")
	if diagnostics.HasErrors() {
		t.Fatal("compile failed")
	}
	browser := &canceledWorkflowBrowser{leasedFakeBrowser: leasedFakeBrowser{fakeBrowser: fakeBrowser{js: map[string]any{"id": "race"}}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Execute(ctx, extractor, map[string]any{"race_id": "42"}, Options{Browser: browser, AllowJavaScript: true})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_BROWSER_WORKFLOW" {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("workflow error does not preserve context cancellation: %v", err)
	}
	if browser.released != 1 {
		t.Fatalf("release count = %d, want 1", browser.released)
	}
}
