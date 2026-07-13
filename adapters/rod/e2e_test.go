//go:build e2e

package rodadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	scrapekdl "github.com/hsblabs/scrape-kdl"
)

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
	content := `extractor "rod-e2e" version=1 {
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
		collection "items" on-row-error="fail" {
			select "li"
			field "id" type="string" required=#true { select ".id" match="one"; value "attr" name="data-id" }
			field "text" type="string" required=#true { select ".label" match="one"; value "text" }
		}
	}`
	if err := os.WriteFile(spec, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	program, diagnostics := scrapekdl.CompileFile(spec)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics: %+v", diagnostics)
	}

	controlURL := launcher.New().Headless(true).MustLaunch()
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
	items, ok := result.Value["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v", result.Value["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["id"] != "1" || first["text"] != "A" {
		t.Fatalf("items[0] = %#v", items[0])
	}
}
