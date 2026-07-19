package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
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
	exitSIGINT     = 130
	exitSIGTERM    = 143
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type commandIO struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type invocation struct {
	spec              string
	inputs            []string
	sessionPath       string
	outPath           string
	requestTimeout    time.Duration
	userAgent         string
	allowJavaScript   bool
	headless          bool
	allowPrivateHosts bool
	jsonOutput        bool
	showHelp          bool
	showVersion       bool
}

func main() {
	streams := commandIO{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(runWithSignals(os.Args[1:], streams))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, commandIO{stdin: stdin, stdout: stdout, stderr: stderr})
}

func runWithSignals(args []string, streams commandIO) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	return runWithSignalChannel(args, streams, signals)
}

func runWithSignalChannel(args []string, streams commandIO, signals <-chan os.Signal) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan os.Signal, 1)
	done := make(chan struct{})
	go func() {
		select {
		case received := <-signals:
			observed <- received
			cancel()
		case <-done:
		}
	}()
	code := runContext(ctx, args, streams)
	close(done)
	select {
	case received := <-observed:
		if received == syscall.SIGTERM {
			return exitSIGTERM
		}
		return exitSIGINT
	default:
		return code
	}
}

func runContext(ctx context.Context, args []string, streams commandIO) int {
	jsonRequested := clisupport.HasJSONFlag(args)
	command, err := parseInvocation(args)
	if err != nil {
		return usageFailure(streams, jsonRequested, err)
	}
	if command.showHelp {
		fmt.Fprint(streams.stdout, helpText)
		return exitSuccess
	}
	if command.showVersion {
		return renderVersion(streams, command.jsonOutput)
	}

	result, diagnostics, err := executeInvocation(ctx, command, streams.stdin)
	if len(diagnostics) > 0 {
		_ = json.NewEncoder(streams.stderr).Encode(diagnostics)
	}
	if diagnostics.HasErrors() {
		return processingFailure(streams, command.jsonOutput, errors.New("compilation failed"))
	}
	if err != nil {
		var usage *invocationError
		if errors.As(err, &usage) {
			return usageFailure(streams, command.jsonOutput, usage)
		}
		return processingFailure(streams, command.jsonOutput, err)
	}
	warnings := make([]clisupport.Warning, len(result.Warnings))
	for index, warning := range result.Warnings {
		warnings[index] = clisupport.Warning{Code: warning.Code, Path: warning.Path, Message: warning.Message}
	}
	fmt.Fprint(streams.stderr, clisupport.FormatWarnings(warnings))
	data, err := clisupport.MarshalExtractionResult(result, command.jsonOutput)
	if err != nil {
		return processingFailure(streams, command.jsonOutput, fmt.Errorf("marshal result: %w", err))
	}
	if command.jsonOutput || command.outPath == "" || command.outPath == "-" {
		if _, err := streams.stdout.Write(data); err != nil {
			fmt.Fprintln(streams.stderr, "write result:", err)
			return exitProcessing
		}
		return exitSuccess
	}
	if err := os.WriteFile(command.outPath, data, 0o644); err != nil {
		return processingFailure(streams, false, fmt.Errorf("write result: %w", err))
	}
	fmt.Fprintf(streams.stderr, "wrote: %s\n", command.outPath)
	return exitSuccess
}

type invocationError struct {
	err error
}

func (err *invocationError) Error() string { return err.err.Error() }
func (err *invocationError) Unwrap() error { return err.err }

func parseInvocation(args []string) (invocation, error) {
	command := invocation{requestTimeout: 30 * time.Second, userAgent: "scrape-kdl/0.5", headless: true}
	if hasHelpFlag(args) {
		command.showHelp = true
		return command, nil
	}
	if clisupport.HasPlaintextSecretFlag(args) {
		return command, errors.New(clisupport.PlaintextSecretFlagMessage)
	}
	flags := flag.NewFlagSet("scrape-kdl-rod", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var inputFlags clisupport.RepeatedFlag
	flags.StringVar(&command.spec, "spec", "", "path to an extractor KDL file")
	flags.BoolVar(&command.showVersion, "version", false, "print version and exit")
	flags.BoolVar(&command.allowJavaScript, "allow-js", false, "allow JavaScript from the trusted spec")
	flags.BoolVar(&command.headless, "headless", true, "run Chromium headlessly")
	flags.BoolVar(&command.allowPrivateHosts, "allow-private-hosts", false, "allow a non-public initial navigation target")
	flags.StringVar(&command.sessionPath, "session-file", "", "read session JSON from a file, or - for standard input")
	flags.DurationVar(&command.requestTimeout, "timeout", command.requestTimeout, "navigation timeout")
	flags.StringVar(&command.userAgent, "user-agent", command.userAgent, "browser User-Agent override")
	flags.BoolVar(&command.jsonOutput, "json", false, "emit exactly one JSON document on standard output")
	flags.StringVar(&command.outPath, "out", "", "write the result to a file, or - for standard output")
	flags.StringVar(&command.outPath, "o", "", "write the result to a file, or - for standard output")
	flags.Var(&inputFlags, "input", "runtime input as name=value (repeatable)")
	if err := flags.Parse(args); err != nil {
		return command, err
	}
	command.inputs = append([]string(nil), inputFlags...)
	if command.showVersion {
		if flags.NArg() != 0 {
			return command, errors.New("--version does not accept positional arguments")
		}
		return command, nil
	}
	if command.spec == "" || flags.NArg() != 0 {
		return command, errors.New("exactly one --spec extractor.kdl is required")
	}
	if command.jsonOutput && command.outPath != "" && command.outPath != "-" {
		return command, errors.New("--json writes one document to stdout and cannot be combined with --out FILE")
	}
	return command, nil
}

func executeInvocation(ctx context.Context, command invocation, stdin io.Reader) (*scrapekdl.Result, scrapekdl.Diagnostics, error) {
	program, diagnostics := scrapekdl.CompileFile(ctx, command.spec)
	if diagnostics.HasErrors() || program == nil {
		return nil, diagnostics, nil
	}
	declarations, err := inputDeclarations(program)
	if err != nil {
		return nil, diagnostics, err
	}
	inputs, err := clisupport.ParseRuntimeInputs(declarations, command.inputs)
	if err != nil {
		return nil, diagnostics, &invocationError{err: err}
	}
	session, err := readSessionFile(command.sessionPath, stdin)
	if err != nil {
		return nil, diagnostics, err
	}
	options := scrapekdl.Options{
		AllowJavaScript: command.allowJavaScript,
		Session:         session,
		RequestTimeout:  command.requestTimeout,
		UserAgent:       command.userAgent,
	}
	if !command.allowPrivateHosts {
		options.URLPolicy = scrapekdl.PublicInternetURLPolicy()
	}
	controlURL, err := launcher.New().Headless(command.headless).Launch()
	if err != nil {
		return nil, diagnostics, fmt.Errorf("launch browser: %w", err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, diagnostics, fmt.Errorf("connect browser: %w", err)
	}
	defer func() { _ = browser.Close() }()
	adapter, err := rodadapter.NewBrowser(browser)
	if err != nil {
		return nil, diagnostics, err
	}
	defer adapter.Close()
	options.Browser = adapter
	result, err := program.Extract(ctx, inputs, options)
	return result, diagnostics, err
}

type cliError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

func processingFailure(streams commandIO, jsonOutput bool, err error) int {
	fmt.Fprintln(streams.stderr, err)
	if jsonOutput {
		failure := cliError{Message: err.Error()}
		var execution *scrapekdl.ExecutionError
		if errors.As(err, &execution) {
			failure = cliError{Code: execution.Code, Message: execution.Message, Path: execution.Path}
		}
		if writeErr := writeFailureJSON(streams.stdout, failure); writeErr != nil {
			fmt.Fprintln(streams.stderr, writeErr)
		}
	}
	return exitProcessing
}

func usageFailure(streams commandIO, jsonOutput bool, err error) int {
	fmt.Fprintln(streams.stderr, err)
	fmt.Fprintln(streams.stderr, "\nRun 'scrape-kdl-rod --help' for usage.")
	if jsonOutput {
		if writeErr := writeFailureJSON(streams.stdout, cliError{Message: err.Error()}); writeErr != nil {
			fmt.Fprintln(streams.stderr, writeErr)
			return exitProcessing
		}
	}
	return exitUsage
}

func writeFailureJSON(writer io.Writer, failure cliError) error {
	data, err := json.MarshalIndent(struct {
		OK    bool     `json:"ok"`
		Error cliError `json:"error"`
	}{OK: false, Error: failure}, "", "  ")
	if err != nil {
		return err
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}

func renderVersion(streams commandIO, jsonOutput bool) int {
	if !jsonOutput {
		fmt.Fprintf(streams.stdout, "scrape-kdl-rod %s (commit=%s, built=%s)\n", version, commit, date)
		return exitSuccess
	}
	data, err := json.MarshalIndent(struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Built   string `json:"built"`
	}{Version: version, Commit: commit, Built: date}, "", "  ")
	if err != nil {
		return processingFailure(streams, true, err)
	}
	if _, err := streams.stdout.Write(append(data, '\n')); err != nil {
		fmt.Fprintln(streams.stderr, err)
		return exitProcessing
	}
	return exitSuccess
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
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

const helpText = `scrape-kdl-rod executes browser-mode Scraping KDL extractors.

USAGE
  scrape-kdl-rod --spec extractor.kdl [options]

EXAMPLES
  scrape-kdl-rod --spec extractor.kdl --input race_id=202401010101 --json
  scrape-kdl-rod --spec extractor.kdl --session-file session.json -o result.json

OPTIONS
  --spec FILE            Extractor KDL file
  --input NAME=VALUE     Typed runtime input; repeatable
  --session-file FILE|-  Session headers and cookies as JSON
  --timeout DURATION     Browser operation timeout (default 30s)
  --user-agent VALUE     Browser User-Agent (default scrape-kdl/0.5)
  -o, --out FILE|-       Result destination
  --json                 Emit one machine-readable document on stdout
  --allow-js             Enable trusted-spec JavaScript
  --allow-private-hosts  Allow a non-public initial navigation target
  --headless BOOL        Run Chromium headlessly (default true)
  --version              Print build metadata
  -h, --help             Show this help
`
