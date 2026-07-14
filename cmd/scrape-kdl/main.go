package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/compiler"
	"github.com/hsblabs/scrape-kdl/internal/executor"
	"github.com/hsblabs/scrape-kdl/internal/ir"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:])
	case "compile":
		return runCompile(args[1:])
	case "extract":
		return runExtract(args[1:])
	case "version", "--version":
		fmt.Fprintf(os.Stdout, "scrape-kdl %s (commit=%s, built=%s)\n", version, commit, date)
		return 0
	case "help", "--help", "-h":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func runValidate(args []string) int {
	if hasHelpFlag(args) {
		usageValidate(os.Stdout)
		return 0
	}
	path, jsonOutput, ok := parseValidateArgs(args)
	if !ok {
		usageValidate(os.Stderr)
		return 2
	}
	diags := compiler.ValidateFile(path)
	if jsonOutput {
		if err := diags.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else if len(diags) > 0 {
		diags.WriteText(os.Stderr)
	}
	if diags.HasErrors() {
		return 1
	}
	if !jsonOutput {
		fmt.Printf("valid: %s\n", path)
	}
	return 0
}

func runCompile(args []string) int {
	if hasHelpFlag(args) {
		usageCompile(os.Stdout)
		return 0
	}
	path, outPath, ok := parseCompileArgs(args)
	if !ok {
		usageCompile(os.Stderr)
		return 2
	}
	result, diags := compiler.CompileFile(path)
	if len(diags) > 0 {
		diags.WriteText(os.Stderr)
	}
	if diags.HasErrors() || result == nil {
		return 1
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal IR:", err)
		return 1
	}
	data = append(data, '\n')
	if outPath != "" {
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write IR:", err)
			return 1
		}
		fmt.Printf("wrote: %s\n", outPath)
		return 0
	}
	_, _ = os.Stdout.Write(data)
	return 0
}

func parseValidateArgs(args []string) (path string, jsonOutput bool, ok bool) {
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "--help", "-h":
			return "", false, false
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", false, false
			}
			if path != "" {
				return "", false, false
			}
			path = arg
		}
	}
	return path, jsonOutput, path != ""
}

func parseCompileArgs(args []string) (path, outPath string, ok bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--emit-ir":
			// Compile emits IR by definition; accepted for specification parity.
		case "--out":
			i++
			if i >= len(args) || outPath != "" {
				return "", "", false
			}
			outPath = args[i]
		case "--help", "-h":
			return "", "", false
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				return "", "", false
			}
			if path != "" {
				return "", "", false
			}
			path = args[i]
		}
	}
	return path, outPath, path != ""
}

func usage(w *os.File) {
	fmt.Fprintln(w, "usage: scrape-kdl <validate|compile|extract|version> ...")
	usageValidate(w)
	usageCompile(w)
	usageExtract(w)
}
func usageValidate(w *os.File) { fmt.Fprintln(w, "  scrape-kdl validate <file.kdl> [--json]") }
func usageCompile(w *os.File) {
	fmt.Fprintln(w, "  scrape-kdl compile <file.kdl> [--emit-ir] [--out file.json]")
}

type repeatedFlag []string

func (values *repeatedFlag) String() string { return strings.Join(*values, ",") }
func (values *repeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func runExtract(args []string) int {
	if hasHelpFlag(args) {
		usageExtract(os.Stdout)
		return 0
	}
	path := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		path = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("extract", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var inputFlags repeatedFlag
	var headerFlags repeatedFlag
	var cookieFlags repeatedFlag
	var htmlPath string
	var outPath string
	var sessionPath string
	var requestTimeout time.Duration
	var maxBody int64
	var userAgent string
	var sessionProvided bool
	flags.Var(&inputFlags, "input", "runtime input as name=value (repeatable)")
	flags.Var(&headerFlags, "header", "deprecated: HTTP session header as Name: value; use --session-file")
	flags.Var(&cookieFlags, "cookie", "deprecated: HTTP session cookie as name=value; use --session-file")
	flags.StringVar(&sessionPath, "session-file", "", "read session JSON from a file, or - for standard input")
	flags.StringVar(&htmlPath, "html", "", "execute against an already-decoded HTML file instead of fetching")
	flags.StringVar(&outPath, "out", "", "write JSON result to a file")
	flags.DurationVar(&requestTimeout, "timeout", 30*time.Second, "HTTP request timeout")
	flags.Int64Var(&maxBody, "max-body", 32<<20, "maximum HTTP response body size in bytes")
	flags.StringVar(&userAgent, "user-agent", "scrape-kdl/0.1", "HTTP User-Agent")
	flags.BoolVar(&sessionProvided, "session", false, "mark an empty runtime session as supplied")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if path == "" {
		if flags.NArg() != 1 {
			usageExtract(os.Stderr)
			return 2
		}
		path = flags.Arg(0)
	} else if flags.NArg() != 0 {
		usageExtract(os.Stderr)
		return 2
	}
	program, diagnostics := compiler.CompileFile(path)
	if len(diagnostics) > 0 {
		diagnostics.WriteText(os.Stderr)
	}
	if diagnostics.HasErrors() || program == nil {
		return 1
	}
	inputs, err := parseRuntimeInputs(program.Inputs, inputFlags)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fileSession, err := readSessionFile(sessionPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	directSession, err := parseSession(headerFlags, cookieFlags, sessionProvided)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	session := mergeSessions(fileSession, directSession)
	options := executor.Options{
		Session: session, RequestTimeout: requestTimeout, MaxResponseBytes: maxBody, UserAgent: userAgent,
	}
	var result *executor.Result
	if htmlPath != "" {
		html, readErr := os.ReadFile(htmlPath)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "read HTML:", readErr)
			return 1
		}
		result, err = executor.ExecuteHTML(context.Background(), program, string(html), options)
	} else {
		result, err = executor.Execute(context.Background(), program, inputs, options)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal result:", err)
		return 1
	}
	data = append(data, '\n')
	if outPath != "" {
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write result:", err)
			return 1
		}
		fmt.Printf("wrote: %s\n", outPath)
		return 0
	}
	_, _ = os.Stdout.Write(data)
	return 0
}

func parseRuntimeInputs(definitions []ir.Input, values []string) (map[string]any, error) {
	definitionByName := make(map[string]ir.Input, len(definitions))
	for _, definition := range definitions {
		definitionByName[definition.Name] = definition
	}
	result := make(map[string]any, len(values))
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --input %q; expected name=value", raw)
		}
		definition, exists := definitionByName[name]
		if !exists {
			return nil, fmt.Errorf("unknown input %q", name)
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("duplicate input %q", name)
		}
		parsed, err := parseCLIInputValue(definition.Type, value)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", name, err)
		}
		result[name] = parsed
	}
	return result, nil
}

func parseCLIInputValue(typeName, value string) (any, error) {
	switch typeName {
	case "string":
		return value, nil
	case "bool":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("expected bool: %w", err)
		}
		return parsed, nil
	case "int":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer: %w", err)
		}
		return parsed, nil
	case "float":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("expected finite float")
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported input type %q", typeName)
	}
}

func parseSession(headers, cookies []string, explicit bool) (*executor.Session, error) {
	if !explicit && len(headers) == 0 && len(cookies) == 0 {
		return nil, nil
	}
	session := &executor.Session{Headers: make(http.Header)}
	for _, raw := range headers {
		name, value, ok := strings.Cut(raw, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid --header; expected Name: value")
		}
		session.Headers.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	for _, raw := range cookies {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid --cookie; expected name=value")
		}
		session.Cookies = append(session.Cookies, &http.Cookie{Name: strings.TrimSpace(name), Value: value})
	}
	return session, nil
}

type sessionDocument struct {
	Headers map[string][]string `json:"headers"`
	Cookies []sessionCookie     `json:"cookies"`
}

type sessionCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func readSessionFile(path string) (*executor.Session, error) {
	if path == "" {
		return nil, nil
	}
	if path == "-" {
		session, err := decodeSessionDocument(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read session from standard input: %w", err)
		}
		return session, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	defer file.Close()
	session, err := decodeSessionDocument(file)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	return session, nil
}

func decodeSessionDocument(reader io.Reader) (*executor.Session, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var document sessionDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode JSON: multiple values")
		}
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	session := &executor.Session{Headers: make(http.Header)}
	for name, values := range document.Headers {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("header name must be non-empty")
		}
		for _, value := range values {
			session.Headers.Add(name, value)
		}
	}
	for _, cookie := range document.Cookies {
		if strings.TrimSpace(cookie.Name) == "" {
			return nil, fmt.Errorf("cookie name must be non-empty")
		}
		session.Cookies = append(session.Cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return session, nil
}

func mergeSessions(base, extra *executor.Session) *executor.Session {
	if base == nil {
		return extra
	}
	if extra == nil {
		return base
	}
	for name, values := range extra.Headers {
		for _, value := range values {
			base.Headers.Add(name, value)
		}
	}
	base.Cookies = append(base.Cookies, extra.Cookies...)
	return base
}

func usageExtract(w *os.File) {
	fmt.Fprintln(w, "  scrape-kdl extract <file.kdl> [--input name=value] [--html file.html] [--session-file session.json] [--out result.json]")
}
