//go:build e2e

package rodadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	page, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "html", "rod-browser-e2e.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	}))
	defer server.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "e2e.kdl")
	source, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "valid", "rod-browser-e2e.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Replace(string(source), "http://127.0.0.1:18080", server.URL, 1)
	if err := os.WriteFile(spec, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	program, diagnostics := scrapekdl.CompileFile(context.Background(), spec)
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
	actualJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "expected-output", "rod-browser-e2e.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("browser result = %#v, want %#v", actual, expected)
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
	failureProgram, failureDiagnostics := scrapekdl.CompileFile(context.Background(), spec)
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
