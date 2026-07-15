package main

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func captureRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		t.Fatal(err)
	}
	originalStdout, originalStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	defer func() {
		os.Stdout, os.Stderr = originalStdout, originalStderr
	}()

	code := run(args)
	if err := stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdoutReader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatal(err)
	}
	return code, string(stdout), string(stderr)
}

func TestRunTopLevelCommands(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "missing command", wantCode: 2, wantStderr: "usage: scrape-kdl"},
		{name: "help", args: []string{"--help"}, wantCode: 0, wantStdout: "USAGE"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "scrape-kdl dev"},
		{name: "unknown", args: []string{"missing"}, wantCode: 2, wantStderr: `unknown command "missing"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := captureRun(t, tt.args...)
			if code != tt.wantCode || !strings.Contains(stdout, tt.wantStdout) || !strings.Contains(stderr, tt.wantStderr) {
				t.Fatalf("run(%q) = code %d, stdout %q, stderr %q", tt.args, code, stdout, stderr)
			}
		})
	}
}

func TestRunSubcommandHelpSucceeds(t *testing.T) {
	for _, args := range [][]string{
		{"validate", "--help"},
		{"validate", "missing.kdl", "-h"},
		{"compile", "--help"},
		{"compile", "missing.kdl", "-h"},
		{"extract", "--help"},
		{"extract", "missing.kdl", "-h"},
	} {
		code, stdout, stderr := captureRun(t, args...)
		if code != 0 || !strings.Contains(stdout, "scrape-kdl "+args[0]) || stderr != "" {
			t.Fatalf("run(%q) = code %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestRunValidate(t *testing.T) {
	valid := filepath.Join("..", "..", "fixtures", "valid", "basic-http.kdl")
	invalid := filepath.Join("..", "..", "fixtures", "invalid", "duplicate-property.kdl")

	code, stdout, stderr := captureRun(t, "validate", valid)
	if code != 0 || stdout != "valid: "+valid+"\n" || stderr != "" {
		t.Fatalf("validate success = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = captureRun(t, "validate", valid, "--json")
	var validation struct {
		OK          bool  `json:"ok"`
		Diagnostics []any `json:"diagnostics"`
	}
	if code != 0 || json.Unmarshal([]byte(stdout), &validation) != nil || !validation.OK || len(validation.Diagnostics) != 0 || stderr != "" {
		t.Fatalf("validate JSON = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, _, stderr = captureRun(t, "validate", invalid)
	if code != 1 || !strings.Contains(stderr, "E_DUPLICATE_PROPERTY") {
		t.Fatalf("validate failure = code %d, stderr %q", code, stderr)
	}
	code, _, stderr = captureRun(t, "validate")
	if code != 2 || !strings.Contains(stderr, "scrape-kdl validate") {
		t.Fatalf("validate usage = code %d, stderr %q", code, stderr)
	}
}

func TestRunCompile(t *testing.T) {
	valid := filepath.Join("..", "..", "fixtures", "valid", "basic-http.kdl")
	invalid := filepath.Join("..", "..", "fixtures", "invalid", "duplicate-property.kdl")

	code, stdout, stderr := captureRun(t, "compile", valid)
	var compiled map[string]any
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &compiled) != nil || compiled["name"] != "basic-http" {
		t.Fatalf("compile success = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	outPath := filepath.Join(t.TempDir(), "program.json")
	code, stdout, stderr = captureRun(t, "compile", valid, "--out", outPath)
	if code != 0 || stdout != "" || stderr != "wrote: "+outPath+"\n" {
		t.Fatalf("compile output = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if data, err := os.ReadFile(outPath); err != nil || json.Unmarshal(data, &compiled) != nil {
		t.Fatalf("compiled file = %q, %v", data, err)
	}
	code, _, stderr = captureRun(t, "compile", invalid)
	if code != 1 || !strings.Contains(stderr, "E_DUPLICATE_PROPERTY") {
		t.Fatalf("compile failure = code %d, stderr %q", code, stderr)
	}
	code, _, stderr = captureRun(t, "compile", valid, "--out", t.TempDir())
	if code != 1 || !strings.Contains(stderr, "write IR:") {
		t.Fatalf("compile write failure = code %d, stderr %q", code, stderr)
	}
}

func TestRunExtractOffline(t *testing.T) {
	valid := filepath.Join("..", "..", "fixtures", "valid", "basic-http.kdl")
	html := filepath.Join("..", "..", "fixtures", "html", "basic-http.html")

	code, stdout, stderr := captureRun(t, "extract", valid, "--html", html, "--input", "id=offline")
	var result struct {
		Value map[string]any `json:"value"`
	}
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &result) != nil || result.Value["title"] != "Scraping KDL Runtime" {
		t.Fatalf("extract success = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	outPath := filepath.Join(t.TempDir(), "result.json")
	code, stdout, stderr = captureRun(t, "extract", "--html", html, "--out", outPath, valid)
	if code != 0 || stdout != "" || stderr != "wrote: "+outPath+"\n" {
		t.Fatalf("extract output = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if data, err := os.ReadFile(outPath); err != nil || json.Unmarshal(data, &result) != nil {
		t.Fatalf("result file = %q, %v", data, err)
	}
	code, _, stderr = captureRun(t, "extract", valid, "--html", filepath.Join(t.TempDir(), "missing.html"))
	if code != 1 || !strings.Contains(stderr, "read HTML:") {
		t.Fatalf("extract read failure = code %d, stderr %q", code, stderr)
	}
	code, _, stderr = captureRun(t, "extract", valid, "--html", html, "--input", "missing=value")
	if code != 2 || !strings.Contains(stderr, `unknown input "missing"`) {
		t.Fatalf("extract input failure = code %d, stderr %q", code, stderr)
	}
}

func TestParseValidateArgs(t *testing.T) {
	for _, args := range [][]string{{"example.kdl", "--json"}, {"--json", "example.kdl"}} {
		path, jsonOutput, ok := parseValidateArgs(args)
		if !ok || path != "example.kdl" || !jsonOutput {
			t.Fatalf("parseValidateArgs(%q) = %q, %v, %v", args, path, jsonOutput, ok)
		}
	}
	for _, args := range [][]string{nil, {"--help"}, {"--unknown", "example.kdl"}, {"one.kdl", "two.kdl"}} {
		if _, _, ok := parseValidateArgs(args); ok {
			t.Fatalf("parseValidateArgs(%q) succeeded", args)
		}
	}
}

func TestParseCompileArgs(t *testing.T) {
	path, output, jsonOutput, ok := parseCompileArgs([]string{"--emit-ir", "example.kdl", "--out", "result.json"})
	if !ok || path != "example.kdl" || output != "result.json" || jsonOutput {
		t.Fatalf("parseCompileArgs success = %q, %q, %v, %v", path, output, jsonOutput, ok)
	}
	for _, args := range [][]string{
		nil,
		{"--help"},
		{"--out"},
		{"example.kdl", "--out", "one.json", "--out", "two.json"},
		{"one.kdl", "two.kdl"},
		{"--unknown", "example.kdl"},
	} {
		if _, _, _, ok := parseCompileArgs(args); ok {
			t.Fatalf("parseCompileArgs(%q) succeeded", args)
		}
	}
}

func TestParseRuntimeInputs(t *testing.T) {
	definitions := []ir.Input{
		{Name: "name", Type: "string"},
		{Name: "enabled", Type: "bool"},
		{Name: "count", Type: "int"},
		{Name: "ratio", Type: "float"},
	}
	got, err := parseRuntimeInputs(definitions, []string{"name=a=b", "enabled=true", "count=-2", "ratio=1.5"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"name": "a=b", "enabled": true, "count": int64(-2), "ratio": 1.5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inputs = %#v, want %#v", got, want)
	}

	tests := []struct {
		name   string
		values []string
		match  string
	}{
		{name: "missing separator", values: []string{"name"}, match: "expected name=value"},
		{name: "empty name", values: []string{"=value"}, match: "expected name=value"},
		{name: "unknown", values: []string{"missing=value"}, match: `unknown input "missing"`},
		{name: "duplicate", values: []string{"name=one", "name=two"}, match: `duplicate input "name"`},
		{name: "invalid bool", values: []string{"enabled=maybe"}, match: "expected bool"},
		{name: "integer overflow", values: []string{"count=999999999999999999999"}, match: "expected integer"},
		{name: "non-finite float", values: []string{"ratio=NaN"}, match: "expected finite float"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRuntimeInputs(definitions, tt.values)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("error = %v, want substring %q", err, tt.match)
			}
		})
	}
}

func TestParseCLIInputValueRejectsUnsupportedAndInfiniteFloat(t *testing.T) {
	if _, err := parseCLIInputValue("object", "{}"); err == nil {
		t.Fatal("unsupported input type succeeded")
	}
	for _, value := range []string{"NaN", "+Inf", "-Inf"} {
		parsed, err := parseCLIInputValue("float", value)
		if err == nil || (parsed != nil && !math.IsNaN(parsed.(float64)) && !math.IsInf(parsed.(float64), 0)) {
			t.Fatalf("parseCLIInputValue(float, %q) = %#v, %v", value, parsed, err)
		}
	}
}

func TestDecodeSessionDocuments(t *testing.T) {
	session, err := decodeSessionDocument(strings.NewReader(`{
  "headers": {"Authorization": ["Bearer secret"], "X-Test": ["one", "two"]},
  "cookies": [{"name": "session", "value": "secret"}]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(session.Headers, http.Header{"Authorization": {"Bearer secret"}, "X-Test": {"one", "two"}}) {
		t.Fatalf("headers = %#v", session.Headers)
	}
	if len(session.Cookies) != 1 || session.Cookies[0].Name != "session" || session.Cookies[0].Value != "secret" {
		t.Fatalf("cookies = %#v", session.Cookies)
	}

	for _, document := range []string{
		``,
		`{"unknown": true}`,
		`{} {}`,
		`{"headers":{"": ["value"]}}`,
		`{"cookies":[{"name":"", "value":"secret"}]}`,
	} {
		if _, err := decodeSessionDocument(strings.NewReader(document)); err == nil {
			t.Fatalf("invalid session document succeeded: %q", document)
		}
	}
}

func TestReadSessionFile(t *testing.T) {
	cli := command{io: commandIO{stdin: strings.NewReader("")}}
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"headers":{},"cookies":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if session, err := cli.readSessionFile(path); err != nil || session == nil {
		t.Fatalf("readSessionFile() = %#v, %v", session, err)
	}
	if session, err := cli.readSessionFile(""); err != nil || session != nil {
		t.Fatalf("readSessionFile(empty) = %#v, %v", session, err)
	}
	if _, err := cli.readSessionFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("missing session file succeeded")
	}
}

func TestRepeatedFlagPreservesOrder(t *testing.T) {
	var values repeatedFlag
	if err := values.Set("first"); err != nil {
		t.Fatal(err)
	}
	if err := values.Set("second"); err != nil {
		t.Fatal(err)
	}
	if got := values.String(); got != "first,second" {
		t.Fatalf("String() = %q", got)
	}
}
