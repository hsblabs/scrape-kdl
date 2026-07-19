//go:build rodstub

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestRodstubCLIPropagatesInvocationToBrowserExtraction(t *testing.T) {
	rod.ResetObservations()
	spec := writeBrowserSpec(t, "https://93.184.216.34/{id}")
	session := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(session, []byte(`{"headers":{"X-Test":["one","two"]},"cookies":[{"name":"sid","value":"secret"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := runContext(context.Background(), []string{
		"--spec", spec,
		"--input", "id=42",
		"--session-file", session,
		"--timeout", "5s",
		"--user-agent", "rodstub-agent",
		"--json",
	}, commandIO{stdout: &stdout, stderr: &stderr})
	if code != exitSuccess {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	assertOneJSONDocument(t, stdout.String())
	var envelope struct {
		Result struct {
			Value map[string]any `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.Value["title"] != "stub title" {
		t.Fatalf("result = %#v", envelope.Result.Value)
	}
	observed := rod.CurrentObservations()
	if observed.NavigateURL != "https://93.184.216.34/42" || observed.UserAgent != "rodstub-agent" {
		t.Fatalf("observations = %#v", observed)
	}
	if !reflect.DeepEqual(observed.Headers, []string{"X-Test", "one", "X-Test", "two"}) {
		t.Fatalf("headers = %#v", observed.Headers)
	}
	if len(observed.Cookies) != 1 || observed.Cookies[0].Name != "sid" || observed.Cookies[0].Value != "secret" {
		t.Fatalf("cookies = %#v", observed.Cookies)
	}
	if !containsDuration(observed.Timeouts, 5*time.Second) {
		t.Fatalf("timeouts = %#v", observed.Timeouts)
	}
}

func TestRodstubCLIAppliesDefaultPublicInternetPolicy(t *testing.T) {
	rod.ResetObservations()
	spec := writeBrowserSpec(t, "http://127.0.0.1/{id}")
	var stdout, stderr strings.Builder
	code := runContext(context.Background(), []string{"--spec", spec, "--input", "id=42", "--json"}, commandIO{stdout: &stdout, stderr: &stderr})
	if code != exitProcessing {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	var envelope struct {
		Error cliError `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "E_URL_POLICY" || rod.CurrentObservations().NavigateURL != "" {
		t.Fatalf("error = %#v, observations = %#v", envelope.Error, rod.CurrentObservations())
	}
}

func TestRodstubCLISignalCancelsBrowserExtraction(t *testing.T) {
	rod.ResetObservations()
	started := rod.BlockNavigation()
	spec := writeBrowserSpec(t, "https://93.184.216.34/{id}")
	signals := make(chan os.Signal, 1)
	result := make(chan int, 1)
	var stdout, stderr strings.Builder
	go func() {
		result <- runWithSignalChannel(
			[]string{"--spec", spec, "--input", "id=42"},
			commandIO{stdout: &stdout, stderr: &stderr},
			signals,
		)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("browser navigation did not start")
	}
	signals <- syscall.SIGTERM
	select {
	case code := <-result:
		if code != exitSIGTERM {
			t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("signal did not stop extraction")
	}
}

func writeBrowserSpec(t *testing.T, target string) string {
	t.Helper()
	source := `extractor "rodstub" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="browser" url="` + target + `"
    session policy="optional"
  }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	path := filepath.Join(t.TempDir(), "extractor.kdl")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsDuration(values []time.Duration, want time.Duration) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
