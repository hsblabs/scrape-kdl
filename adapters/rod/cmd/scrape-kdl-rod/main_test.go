package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
	"github.com/hsblabs/scrape-kdl/internal/clisupport"
)

func TestInputDeclarationsFromProgramIR(t *testing.T) {
	source := `extractor "decl" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="browser" url="https://example.invalid/{id}" }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	path := filepath.Join(t.TempDir(), "decl.kdl")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	program, diagnostics, err := scrapekdl.CompileFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	declarations, err := inputDeclarations(program)
	if err != nil {
		t.Fatal(err)
	}
	want := clisupport.InputDeclaration{Name: "id", Type: "string", Required: true}
	if len(declarations) != 1 || declarations[0] != want {
		t.Fatalf("declarations = %#v", declarations)
	}
}

func TestReadSessionFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	document := `{"headers":{"X-Test":["one","two"]},"cookies":[{"name":"sid","value":"abc"}]}`
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := readSessionFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := session.Headers.Values("X-Test"); len(got) != 2 || got[0] != "one" {
		t.Fatalf("headers = %#v", session.Headers)
	}
	if len(session.Cookies) != 1 || session.Cookies[0].Name != "sid" || session.Cookies[0].Value != "abc" {
		t.Fatalf("cookies = %#v", session.Cookies)
	}
	if session, err := readSessionFile("", nil); session != nil || err != nil {
		t.Fatalf("empty path = %v, %v", session, err)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"headers":{},"unknown":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSessionFile(bad, nil); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestRunRejectsPlaintextSecretFlags(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--header", "X: y", "--json"}, nil, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "--session-file") {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	assertOneJSONDocument(t, stdout.String())
}

func TestRunRequiresSpec(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run([]string{"--json"}, nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("code = %d", code)
	}
	assertOneJSONDocument(t, stdout.String())
}

func TestParseInvocationKeepsCLIContractPure(t *testing.T) {
	command, err := parseInvocation([]string{"--spec", "extractor.kdl", "--input", "id=42", "-o", "-", "--timeout", "5s"})
	if err != nil {
		t.Fatal(err)
	}
	if command.spec != "extractor.kdl" || command.outPath != "-" || command.requestTimeout.String() != "5s" || len(command.inputs) != 1 {
		t.Fatalf("invocation = %#v", command)
	}
	if _, err := parseInvocation([]string{"--spec", "extractor.kdl", "--json", "--out", "result.json"}); err == nil {
		t.Fatal("--json --out FILE succeeded")
	}
	help, err := parseInvocation([]string{"--unknown", "--help"})
	if err != nil || !help.showHelp {
		t.Fatalf("help invocation = %#v, %v", help, err)
	}
}

func TestRunJSONInputFailureDoesNotLaunchBrowser(t *testing.T) {
	source := `extractor "input-error" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="browser" url="https://93.184.216.34/{id}" }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	path := filepath.Join(t.TempDir(), "extractor.kdl")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := run([]string{"--spec", path, "--input", "missing=value", "--json"}, nil, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), `unknown input "missing"`) {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	assertOneJSONDocument(t, stdout.String())
}

func TestRunHelpAndVersionAreMachineSafe(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"--unknown", "--help"}} {
		var stdout, stderr strings.Builder
		if code := run(args, nil, &stdout, &stderr); code != exitSuccess || stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--version", "--json"}, nil, &stdout, &stderr); code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("version code=%d stderr=%q", code, stderr.String())
	}
	assertOneJSONDocument(t, stdout.String())
}

func assertOneJSONDocument(t *testing.T, output string) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(output))
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode JSON %q: %v", output, err)
	}
	if err := decoder.Decode(&value); err != io.EOF {
		t.Fatalf("stdout contains more than one JSON document: %q", output)
	}
}
