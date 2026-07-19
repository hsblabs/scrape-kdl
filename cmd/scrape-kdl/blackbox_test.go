package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type processResult struct {
	code   int
	stdout string
	stderr string
}

func TestCLIBlackBoxContract(t *testing.T) {
	binary := buildCLI(t)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(root, "fixtures", "valid", "basic-http.kdl")
	invalid := filepath.Join(root, "fixtures", "invalid", "duplicate-property.kdl")
	htmlPath := filepath.Join(root, "fixtures", "html", "basic-http.html")
	partialSource := filepath.Join(root, "examples", "partial-result", "extractor.kdl")
	partialHTML := filepath.Join(root, "examples", "partial-result", "page.html")
	source, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("help", func(t *testing.T) {
		for _, args := range [][]string{
			{"--help"}, {"unknown", "--help"}, {"help", "validate"}, {"help", "compile"}, {"help", "extract"}, {"help", "version"},
			{"validate", "--help"}, {"compile", "--help"}, {"extract", "--help"}, {"version", "--help"},
		} {
			result := runCLI(t, binary, nil, args...)
			if result.code != exitSuccess || result.stdout == "" || result.stderr != "" {
				t.Fatalf("%q: %#v", args, result)
			}
		}
	})

	t.Run("JSON success and failure", func(t *testing.T) {
		for _, test := range []struct {
			args []string
			code int
		}{
			{args: []string{"validate", valid, "--json"}, code: exitSuccess},
			{args: []string{"compile", valid, "--json"}, code: exitSuccess},
			{args: []string{"extract", valid, "--html", htmlPath, "--json"}, code: exitSuccess},
			{args: []string{"version", "--json"}, code: exitSuccess},
			{args: []string{"validate", invalid, "--json"}, code: exitProcessing},
			{args: []string{"compile", invalid, "--json"}, code: exitProcessing},
			{args: []string{"extract", valid, "--html", filepath.Join(t.TempDir(), "missing.html"), "--json"}, code: exitProcessing},
			{args: []string{"extract", valid, "--html", htmlPath, "--input", "unknown=value", "--json"}, code: exitUsage},
		} {
			result := runCLI(t, binary, nil, test.args...)
			if result.code != test.code {
				t.Fatalf("%q code = %d, stderr = %q", test.args, result.code, result.stderr)
			}
			assertOneJSONDocument(t, result.stdout)
			assertNonTTYOutput(t, result.stdout, result.stderr)
		}
	})

	t.Run("explicit streams", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			stdin []byte
			args  []string
		}{
			{name: "validate source", stdin: source, args: []string{"validate", "-", "--json"}},
			{name: "compile source", stdin: source, args: []string{"compile", "-", "--json"}},
			{name: "compile stdout", args: []string{"compile", valid, "--out", "-"}},
			{name: "extract HTML", stdin: html, args: []string{"extract", valid, "--html", "-", "--json"}},
			{name: "extract stdout", args: []string{"extract", valid, "--html", htmlPath, "-o", "-"}},
		} {
			t.Run(test.name, func(t *testing.T) {
				result := runCLI(t, binary, test.stdin, test.args...)
				if result.code != exitSuccess {
					t.Fatalf("result = %#v", result)
				}
				assertOneJSONDocument(t, result.stdout)
			})
		}
		result := runCLI(t, binary, source, "extract", "-", "--html", "-", "--json")
		if result.code != exitUsage || !strings.Contains(result.stderr, "standard input can supply only one") {
			t.Fatalf("stdin conflict = %#v", result)
		}
		assertOneJSONDocument(t, result.stdout)
	})

	t.Run("warnings stay off stdout", func(t *testing.T) {
		result := runCLI(t, binary, nil, "extract", partialSource, "--html", partialHTML, "--json")
		if result.code != exitSuccess || !strings.Contains(result.stderr, "W_ERROR_RECOVERED at output.status") || !strings.Contains(result.stderr, "W_ROW_SKIPPED at output.items") {
			t.Fatalf("partial result = %#v", result)
		}
		assertOneJSONDocument(t, result.stdout)
	})

	t.Run("exit classes and removed secret flags", func(t *testing.T) {
		if result := runCLI(t, binary, nil); result.code != exitUsage || result.stdout != "" {
			t.Fatalf("missing command = %#v", result)
		}
		if result := runCLI(t, binary, nil, "compile", invalid); result.code != exitProcessing {
			t.Fatalf("processing failure = %#v", result)
		}
		result := runCLI(t, binary, nil, "extract", valid, "--header", "Authorization: DO-NOT-ECHO")
		if result.code != exitUsage || !strings.Contains(result.stderr, "--session-file") || strings.Contains(result.stderr, "Authorization") || strings.Contains(result.stderr, "DO-NOT-ECHO") {
			t.Fatalf("removed secret flag = %#v", result)
		}
	})

	t.Run("binary smoke is non-interactive", func(t *testing.T) {
		result := runCLI(t, binary, nil, "version")
		if result.code != exitSuccess || !strings.HasPrefix(result.stdout, "scrape-kdl dev") || result.stderr != "" {
			t.Fatalf("version = %#v", result)
		}
		assertNonTTYOutput(t, result.stdout, result.stderr)
	})

	t.Run("signals", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			signal os.Signal
			code   int
		}{
			{name: "SIGINT", signal: os.Interrupt, code: exitSIGINT},
			{name: "SIGTERM", signal: syscall.SIGTERM, code: exitSIGTERM},
		} {
			t.Run(test.name, func(t *testing.T) { assertSignalExit(t, binary, test.signal, test.code) })
		}
	})
}

func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "scrape-kdl")
	command := exec.Command("go", "build", "-o", binary, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func runCLI(t *testing.T, binary string, stdin []byte, args ...string) processResult {
	t.Helper()
	command := exec.Command(binary, args...)
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := exitSuccess
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return processResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
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

func assertNonTTYOutput(t *testing.T, outputs ...string) {
	t.Helper()
	for _, output := range outputs {
		if strings.Contains(output, "\x1b[") || strings.Contains(output, "\r") || strings.Contains(output, "[y/N]") {
			t.Fatalf("non-TTY output contains formatting or a prompt: %q", output)
		}
	}
}

func assertSignalExit(t *testing.T, binary string, interrupt os.Signal, wantCode int) {
	t.Helper()
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started <- struct{}{}
		select {
		case <-request.Context().Done():
		case <-time.After(10 * time.Second):
			response.WriteHeader(http.StatusGatewayTimeout)
		}
	}))
	defer server.Close()
	source := `extractor "signal" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="` + server.URL + `" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`
	path := filepath.Join(t.TempDir(), "signal.kdl")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "extract", path, "--allow-private-hosts")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("CLI did not start its HTTP request")
	}
	if err := command.Process.Signal(interrupt); err != nil {
		t.Fatal(err)
	}
	err := command.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != wantCode {
		t.Fatalf("signal exit = %v code %d, stdout %q, stderr %q", err, exitCode(exit), stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("signal stdout = %q", stdout.String())
	}
}

func exitCode(exit *exec.ExitError) int {
	if exit == nil {
		return exitSuccess
	}
	return exit.ExitCode()
}
