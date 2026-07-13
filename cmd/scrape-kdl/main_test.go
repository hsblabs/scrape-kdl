package main

import (
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

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
	path, output, ok := parseCompileArgs([]string{"--emit-ir", "example.kdl", "--out", "result.json"})
	if !ok || path != "example.kdl" || output != "result.json" {
		t.Fatalf("parseCompileArgs success = %q, %q, %v", path, output, ok)
	}
	for _, args := range [][]string{
		nil,
		{"--help"},
		{"--out"},
		{"example.kdl", "--out", "one.json", "--out", "two.json"},
		{"one.kdl", "two.kdl"},
		{"--unknown", "example.kdl"},
	} {
		if _, _, ok := parseCompileArgs(args); ok {
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

func TestParseSession(t *testing.T) {
	if session, err := parseSession(nil, nil, false); err != nil || session != nil {
		t.Fatalf("absent session = %#v, %v", session, err)
	}
	if session, err := parseSession(nil, nil, true); err != nil || session == nil {
		t.Fatalf("explicit empty session = %#v, %v", session, err)
	}

	session, err := parseSession(
		[]string{" Accept-Language : ja ", "X-Test: one", "X-Test: two"},
		[]string{" session =a=b"},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(session.Headers, http.Header{"Accept-Language": {"ja"}, "X-Test": {"one", "two"}}) {
		t.Fatalf("headers = %#v", session.Headers)
	}
	if len(session.Cookies) != 1 || session.Cookies[0].Name != "session" || session.Cookies[0].Value != "a=b" {
		t.Fatalf("cookies = %#v", session.Cookies)
	}

	for _, test := range []struct {
		name    string
		headers []string
		cookies []string
	}{
		{name: "header separator", headers: []string{"missing"}},
		{name: "header name", headers: []string{": value"}},
		{name: "cookie separator", cookies: []string{"missing"}},
		{name: "cookie name", cookies: []string{" =value"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseSession(test.headers, test.cookies, false); err == nil {
				t.Fatal("invalid session value succeeded")
			}
		})
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
