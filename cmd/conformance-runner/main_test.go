package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/conformance"
)

func TestHelpUsesStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--suite", "missing", "--help"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Usage:") || stderr.Len() != 0 {
		t.Fatalf("help = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestListAndRunEmitJSON(t *testing.T) {
	manifest := filepath.Join("..", "..", "conformance", "manifest.json")
	for _, args := range [][]string{
		{"--manifest", manifest, "--suite", "typescript-core", "--list"},
		{"--manifest", manifest, "--suite", "runtime"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("run(%v) = code %d, stderr %q", args, code, stderr.String())
		}
		var value map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
			t.Fatalf("run(%v) stdout is not JSON: %v\n%s", args, err, stdout.String())
		}
		if value["cases"] == nil {
			t.Fatalf("run(%v) has no cases: %v", args, value)
		}
	}
}

func TestUsageErrorsUseStderr(t *testing.T) {
	tests := [][]string{
		{"unexpected"},
		{"--implementation", "typescript"},
		{"--suite", "missing", "--manifest", filepath.Join("..", "..", "conformance", "manifest.json")},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("run(%v) = code %d, stdout %q, stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunResultShape(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"--manifest", filepath.Join("..", "..", "conformance", "manifest.json"),
		"--suite", "runtime",
	}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("run = code %d, stderr %q", code, stderr.String())
	}
	var result conformance.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || result.Implementation != "go" || len(result.Cases) != 1 {
		t.Fatalf("result = %#v", result)
	}
}
