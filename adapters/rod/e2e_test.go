//go:build e2e

package rodadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func newE2ELauncher() *launcher.Launcher {
	browserLauncher := launcher.New().Headless(true)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		// GitHub-hosted Linux runners do not provide a usable Chromium sandbox.
		// This is limited to the localhost-only E2E fixture and never affects the
		// adapter or browser instances supplied by library users.
		browserLauncher.NoSandbox(true)
	}
	return browserLauncher
}

func TestE2ELauncherDisablesSandboxOnlyOnGitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	if newE2ELauncher().Has(flags.NoSandbox) {
		t.Fatal("local E2E launcher unexpectedly disables the Chromium sandbox")
	}

	t.Setenv("GITHUB_ACTIONS", "true")
	if !newE2ELauncher().Has(flags.NoSandbox) {
		t.Fatal("GitHub Actions E2E launcher must disable the unavailable Chromium sandbox")
	}
}

func TestBrowserExtractionE2E(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body>
			<button id="load" onclick="document.querySelector('#value').textContent='ready'">load</button>
			<div id="value">pending</div>
			<ul><li><span class="id" data-id="1"></span><span class="label">A</span></li><li><span class="id" data-id="2"></span><span class="label">B</span></li></ul>
		</body></html>`))
	}))
	defer server.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "e2e.kdl")
	content := `extractor "rod-e2e" version="2026-07-15" language-version="2026-07-15" {
		source "html" {
			fetch mode="browser" url="` + server.URL + `"
			workflow {
				click "#load"
				wait-for "#value" state="visible"
			}
		}
		field "value" type="string" required=#true {
			select "#value" match="one"
			value "text"
		}
		field "title" type="string" required=#true {
			evaluate-js #"""
				() => document.title || "untitled"
				"""# scope="document" returns="string"
		}
		field "item_count" type="int" required=#true {
			evaluate-js "() => document.querySelectorAll('li').length" scope="document" returns="int"
		}
		collection "items" on-row-error="fail" {
			select "li"
			field "id" type="string" required=#true { select ".id" match="one"; value "attr" name="data-id" }
			field "text" type="string" required=#true { select ".label" match="one"; value "text" }
			field "dataset" type="object" required=#true {
				select ".id" match="one"
				evaluate-js #"""
					(element) => ({ id: element.dataset.id })
					"""# scope="current" returns="object"
			}
		}
	}`
	if err := os.WriteFile(spec, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	program, diagnostics := scrapekdl.CompileFile(spec)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics: %+v", diagnostics)
	}

	controlURL := newE2ELauncher().MustLaunch()
	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()
	adapter, err := NewBrowser(browser)
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()

	result, err := program.Extract(context.Background(), nil, scrapekdl.Options{
		Browser:         adapter,
		AllowJavaScript: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["value"] != "ready" {
		t.Fatalf("value = %#v", result.Value["value"])
	}
	if result.Value["title"] != "untitled" {
		t.Fatalf("title = %#v", result.Value["title"])
	}
	if result.Value["item_count"] != int64(2) {
		t.Fatalf("item_count = %#v (%T)", result.Value["item_count"], result.Value["item_count"])
	}
	items, ok := result.Value["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v", result.Value["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["id"] != "1" || first["text"] != "A" {
		t.Fatalf("items[0] = %#v", items[0])
	}
	dataset, ok := first["dataset"].(map[string]any)
	if !ok || dataset["id"] != "1" {
		t.Fatalf("items[0].dataset = %#v", first["dataset"])
	}

	failureContent := `extractor "rod-js-failure" version="2026-07-15" language-version="2026-07-15" {
		source "html" { fetch mode="browser" url="` + server.URL + `" }
		field "failure" type="string" required=#true {
			evaluate-js #"""
				() => { throw new Error("expected test failure") }
				"""# scope="document" returns="string"
		}
	}`
	if err := os.WriteFile(spec, []byte(failureContent), 0o600); err != nil {
		t.Fatal(err)
	}
	failureProgram, failureDiagnostics := scrapekdl.CompileFile(spec)
	if failureDiagnostics.HasErrors() {
		t.Fatalf("failure compile diagnostics: %+v", failureDiagnostics)
	}
	_, err = failureProgram.Extract(context.Background(), nil, scrapekdl.Options{
		Browser:         adapter,
		AllowJavaScript: true,
	})
	var executionError *scrapekdl.ExecutionError
	if !errors.As(err, &executionError) || executionError.Code != "E_JAVASCRIPT_EVALUATION" || executionError.Path != "output.failure" {
		t.Fatalf("JavaScript failure = %v", err)
	}
}
