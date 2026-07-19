package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	scrapekdl "github.com/hsblabs/scrape-kdl"
	rodadapter "github.com/hsblabs/scrape-kdl/adapters/rod"
)

const (
	exitSuccess    = 0
	exitProcessing = 1
	exitUsage      = 2
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type repeatedFlag []string

func (values *repeatedFlag) String() string         { return strings.Join(*values, ",") }
func (values *repeatedFlag) Set(value string) error { *values = append(*values, value); return nil }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, arg := range args {
		if arg == "--header" || strings.HasPrefix(arg, "--header=") || arg == "--cookie" || strings.HasPrefix(arg, "--cookie=") {
			fmt.Fprintln(stderr, "--header and --cookie are not accepted; put secrets in --session-file FILE or use --session-file -")
			return exitUsage
		}
	}
	flags := flag.NewFlagSet("scrape-kdl-rod", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var inputFlags repeatedFlag
	spec := flags.String("spec", "", "path to an extractor KDL file")
	showVersion := flags.Bool("version", false, "print version and exit")
	allowJS := flags.Bool("allow-js", false, "allow JavaScript from the trusted spec")
	headless := flags.Bool("headless", true, "run Chromium headlessly")
	allowPrivateHosts := flags.Bool("allow-private-hosts", false, "allow navigation to loopback, private, and link-local addresses")
	sessionPath := flags.String("session-file", "", "read session JSON from a file, or - for standard input")
	requestTimeout := flags.Duration("timeout", 30*time.Second, "navigation timeout")
	userAgent := flags.String("user-agent", "scrape-kdl/0.5", "browser User-Agent override")
	jsonOutput := flags.Bool("json", false, "emit exactly one JSON document on standard output")
	outPath := flags.String("out", "", "write the result to a file, or - for standard output")
	flags.Var(&inputFlags, "input", "runtime input as name=value (repeatable)")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *showVersion {
		fmt.Fprintf(stdout, "scrape-kdl-rod %s (commit=%s, built=%s)\n", version, commit, date)
		return exitSuccess
	}
	if *spec == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: scrape-kdl-rod -spec extractor.kdl [--input name=value ...] [options]")
		return exitUsage
	}

	program, diagnostics := scrapekdl.CompileFile(context.Background(), *spec)
	if diagnostics.HasErrors() {
		_ = json.NewEncoder(stderr).Encode(diagnostics)
		return exitProcessing
	}
	declarations, err := inputDeclarations(program)
	if err != nil {
		return fail(stderr, stdout, *jsonOutput, err)
	}
	inputs, err := parseRuntimeInputs(declarations, inputFlags)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	session, err := readSessionFile(*sessionPath, stdin)
	if err != nil {
		return fail(stderr, stdout, *jsonOutput, err)
	}

	options := scrapekdl.Options{
		AllowJavaScript: *allowJS,
		Session:         session,
		RequestTimeout:  *requestTimeout,
		UserAgent:       *userAgent,
	}
	if !*allowPrivateHosts {
		options.URLPolicy = scrapekdl.PublicInternetURLPolicy()
	}

	controlURL, err := launcher.New().Headless(*headless).Launch()
	if err != nil {
		return fail(stderr, stdout, *jsonOutput, fmt.Errorf("launch browser: %w", err))
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return fail(stderr, stdout, *jsonOutput, fmt.Errorf("connect browser: %w", err))
	}
	defer func() { _ = browser.Close() }()
	adapter, err := rodadapter.NewBrowser(browser)
	if err != nil {
		return fail(stderr, stdout, *jsonOutput, err)
	}
	defer adapter.Close()
	options.Browser = adapter

	result, err := program.Extract(context.Background(), inputs, options)
	if err != nil {
		return fail(stderr, stdout, *jsonOutput, err)
	}
	for _, warning := range result.Warnings {
		path := ""
		if warning.Path != "" {
			path = " at " + warning.Path
		}
		fmt.Fprintf(stderr, "%s%s: %s\n", warning.Code, path, warning.Message)
	}
	if *jsonOutput {
		if err := writeJSON(stdout, struct {
			OK     bool              `json:"ok"`
			Result *scrapekdl.Result `json:"result"`
		}{OK: true, Result: result}); err != nil {
			fmt.Fprintln(stderr, "marshal result:", err)
			return exitProcessing
		}
		return exitSuccess
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "marshal result:", err)
		return exitProcessing
	}
	data = append(data, '\n')
	if *outPath == "" || *outPath == "-" {
		if _, err := stdout.Write(data); err != nil {
			return exitProcessing
		}
		return exitSuccess
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		fmt.Fprintln(stderr, "write result:", err)
		return exitProcessing
	}
	fmt.Fprintf(stderr, "wrote: %s\n", *outPath)
	return exitSuccess
}

type cliError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

func fail(stderr, stdout io.Writer, jsonOutput bool, err error) int {
	fmt.Fprintln(stderr, err)
	if jsonOutput {
		failure := cliError{Message: err.Error()}
		var execution *scrapekdl.ExecutionError
		if errors.As(err, &execution) {
			failure = cliError{Code: execution.Code, Message: execution.Message, Path: execution.Path}
		}
		if writeErr := writeJSON(stdout, struct {
			OK    bool     `json:"ok"`
			Error cliError `json:"error"`
		}{OK: false, Error: failure}); writeErr != nil {
			return exitProcessing
		}
	}
	return exitProcessing
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

type inputDeclaration struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// inputDeclarations reads input declarations from the stable Validated IR
// JSON contract; ProgramMetadata does not expose them.
func inputDeclarations(program *scrapekdl.Program) ([]inputDeclaration, error) {
	raw, err := program.IRJSON()
	if err != nil {
		return nil, fmt.Errorf("read program IR: %w", err)
	}
	var document struct {
		Inputs []inputDeclaration `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("decode program IR: %w", err)
	}
	return document.Inputs, nil
}

func parseRuntimeInputs(declarations []inputDeclaration, values []string) (map[string]any, error) {
	byName := make(map[string]inputDeclaration, len(declarations))
	for _, declaration := range declarations {
		byName[declaration.Name] = declaration
	}
	result := make(map[string]any, len(values))
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --input %q; expected name=value", raw)
		}
		declaration, exists := byName[name]
		if !exists {
			return nil, fmt.Errorf("unknown input %q", name)
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("duplicate input %q", name)
		}
		parsed, err := parseInputValue(declaration.Type, value)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", name, err)
		}
		result[name] = parsed
	}
	return result, nil
}

func parseInputValue(typeName, value string) (any, error) {
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

type sessionDocument struct {
	Headers map[string][]string `json:"headers"`
	Cookies []sessionCookie     `json:"cookies"`
}

type sessionCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func readSessionFile(path string, stdin io.Reader) (*scrapekdl.Session, error) {
	if path == "" {
		return nil, nil
	}
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("read session file: %w", err)
		}
		defer file.Close()
		reader = file
	}
	session, err := decodeSessionDocument(reader)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	return session, nil
}

func decodeSessionDocument(reader io.Reader) (*scrapekdl.Session, error) {
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
	session := &scrapekdl.Session{Headers: make(http.Header)}
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
