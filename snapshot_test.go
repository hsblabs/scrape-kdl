package scrapekdl_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

type snapshotTransport struct {
	calls int
}

func (transport *snapshotTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls++
	return nil, errors.New("snapshot execution attempted HTTP transport")
}

func TestExtractSnapshotMatchesSharedFixtures(t *testing.T) {
	data, err := os.ReadFile("fixtures/snapshot/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string `json:"name"`
		Source   string `json:"source"`
		HTML     string `json:"html"`
		Expected *struct {
			Value    map[string]any      `json:"value"`
			Warnings []scrapekdl.Warning `json:"warnings"`
			Partial  bool                `json:"partial"`
		} `json:"expected"`
		ExternalTransform bool `json:"externalTransform"`
		Error             *struct {
			Code string `json:"code"`
			Path string `json:"path"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			program, diagnostics, err := scrapekdl.Compile(context.Background(), scrapekdl.Source{
				Path: testCase.Name + ".kdl", Data: []byte(testCase.Source),
			}, scrapekdl.CompileOptions{})
			if err != nil || diagnostics.HasErrors() || program == nil {
				t.Fatalf("compile = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
			}
			transport := &snapshotTransport{}
			policyCalls := 0
			options := scrapekdl.Options{
				HTTPClient: &http.Client{Transport: transport},
				URLPolicy: func(context.Context, *url.URL) error {
					policyCalls++
					return errors.New("snapshot execution attempted URL policy")
				},
			}
			if testCase.ExternalTransform {
				options.ExternalTransforms = map[string]scrapekdl.ExternalTransform{
					"decorate": func(_ context.Context, input any) (any, error) { return input.(string) + "!", nil },
				}
			}
			result, err := program.ExtractSnapshot(context.Background(), testCase.HTML, options)
			if transport.calls != 0 || policyCalls != 0 {
				t.Fatalf("snapshot performed acquisition: transport=%d policy=%d", transport.calls, policyCalls)
			}
			if testCase.Error != nil {
				var execution *scrapekdl.ExecutionError
				if result != nil || !errors.As(err, &execution) || execution.Code != testCase.Error.Code || execution.Path != testCase.Error.Path {
					t.Fatalf("snapshot error = result %#v, error %#v", result, err)
				}
				return
			}
			if err != nil || testCase.Expected == nil {
				t.Fatalf("snapshot result = %#v, error %v", result, err)
			}
			actualJSON, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			expectedJSON, err := json.Marshal(testCase.Expected)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actualJSON, expectedJSON) {
				t.Fatalf("snapshot result = %#v, expected %#v", result, testCase.Expected)
			}
		})
	}
}

func TestExtractSnapshotPreservesCancellationAndExtractHTMLMode(t *testing.T) {
	program, diagnostics, err := scrapekdl.Compile(context.Background(), scrapekdl.Source{
		Path: "browser.kdl",
		Data: []byte(`extractor "browser" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="browser" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`),
	}, scrapekdl.CompileOptions{})
	if err != nil || diagnostics.HasErrors() || program == nil {
		t.Fatalf("compile = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = program.ExtractSnapshot(ctx, "<h1>Saved</h1>", scrapekdl.Options{})
	var execution *scrapekdl.ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_EXECUTION_CANCELED" || !errors.Is(err, context.Canceled) {
		t.Fatalf("snapshot cancellation = %v", err)
	}
	_, err = program.ExtractHTML(context.Background(), "<h1>Saved</h1>", scrapekdl.Options{})
	if !errors.As(err, &execution) || execution.Code != "E_BROWSER_RUNTIME_MISSING" {
		t.Fatalf("ExtractHTML browser mode error = %v", err)
	}
}
