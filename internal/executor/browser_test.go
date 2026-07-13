package executor

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/compiler"
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
