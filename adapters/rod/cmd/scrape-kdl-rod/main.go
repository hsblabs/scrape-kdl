package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	scrapekdl "github.com/hsblabs/scrape-kdl"
	rodadapter "github.com/hsblabs/scrape-kdl/adapters/rod"
	"github.com/hsblabs/scrape-kdl/internal/clisupport"
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

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if clisupport.HasPlaintextSecretFlag(args) {
		fmt.Fprintln(stderr, clisupport.PlaintextSecretFlagMessage)
		return exitUsage
	}
	flags := flag.NewFlagSet("scrape-kdl-rod", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var inputFlags clisupport.RepeatedFlag
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
	inputs, err := clisupport.ParseRuntimeInputs(declarations, inputFlags)
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

// inputDeclarations reads input declarations from the stable Validated IR
// JSON contract; ProgramMetadata does not expose them.
func inputDeclarations(program *scrapekdl.Program) ([]clisupport.InputDeclaration, error) {
	raw, err := program.IRJSON()
	if err != nil {
		return nil, fmt.Errorf("read program IR: %w", err)
	}
	var document struct {
		Inputs []clisupport.InputDeclaration `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("decode program IR: %w", err)
	}
	return document.Inputs, nil
}

func readSessionFile(path string, stdin io.Reader) (*scrapekdl.Session, error) {
	if path == "" {
		return nil, nil
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	headers, cookies, err := clisupport.DecodeSessionDocument(data)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	return &scrapekdl.Session{Headers: headers, Cookies: cookies}, nil
}
