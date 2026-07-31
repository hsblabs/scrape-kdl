package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/clisupport"
	"github.com/hsblabs/scrape-kdl/internal/compiler"
	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/executor"
	"github.com/hsblabs/scrape-kdl/internal/ir"
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

type command struct {
	io commandIO
}

type errorTrackingWriter struct {
	writer io.Writer
	err    error
}

func (writer *errorTrackingWriter) Write(data []byte) (int, error) {
	if writer.err != nil {
		return 0, writer.err
	}
	written, err := writer.writer.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		writer.err = err
	}
	return written, err
}

func main() {
	streams := commandIO{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(runWithSignals(os.Args[1:], streams))
}

func run(args []string) int {
	return runContext(context.Background(), args, commandIO{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr})
}

func runWithSignals(args []string, streams commandIO) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	observed := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
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

func runContext(ctx context.Context, args []string, streams commandIO) (code int) {
	stdout := &errorTrackingWriter{writer: streams.stdout}
	stderr := &errorTrackingWriter{writer: streams.stderr}
	streams.stdout = stdout
	streams.stderr = stderr
	defer func() {
		if stdout.err != nil || stderr.err != nil {
			code = exitProcessing
		}
	}()
	cli := command{io: streams}
	if len(args) == 0 {
		cli.conciseRoot(streams.stderr)
		return exitUsage
	}
	switch args[0] {
	case "validate":
		return cli.runValidate(ctx, args[1:])
	case "compile":
		return cli.runCompile(ctx, args[1:])
	case "extract":
		return cli.runExtract(ctx, args[1:])
	case "version", "--version":
		return cli.runVersion(args[1:])
	case "help":
		return cli.runHelp(args[1:])
	case "--help", "-h":
		cli.helpRoot(streams.stdout)
		return exitSuccess
	default:
		if hasHelpFlag(args) {
			cli.helpRoot(streams.stdout)
			return exitSuccess
		}
		fmt.Fprintf(streams.stderr, "unknown command %q\n\n", args[0])
		fmt.Fprintln(streams.stderr, "Run 'scrape-kdl --help' for usage.")
		return exitUsage
	}
}

func (cli command) runHelp(args []string) int {
	if len(args) == 0 {
		cli.helpRoot(cli.io.stdout)
		return exitSuccess
	}
	if len(args) != 1 {
		return cli.usageError("help", "help accepts at most one command")
	}
	switch args[0] {
	case "validate":
		cli.helpValidate(cli.io.stdout)
	case "compile":
		cli.helpCompile(cli.io.stdout)
	case "extract":
		cli.helpExtract(cli.io.stdout)
	case "version":
		cli.helpVersion(cli.io.stdout)
	default:
		return cli.usageError("help", fmt.Sprintf("unknown help topic %q", args[0]))
	}
	return exitSuccess
}

func (cli command) runValidate(ctx context.Context, args []string) int {
	if hasHelpFlag(args) {
		cli.helpValidate(cli.io.stdout)
		return exitSuccess
	}
	path, jsonOutput, ok := parseValidateArgs(args)
	if !ok {
		if hasJSONFlag(args) {
			return cli.usageFailure(true, "validate", errors.New("invalid validate arguments"))
		}
		cli.conciseValidate(cli.io.stderr)
		return exitUsage
	}
	_, diagnostics, err := cli.compileInput(ctx, path)
	if err != nil {
		return cli.fail(jsonOutput, err)
	}
	if len(diagnostics) > 0 {
		if err := diagnostics.WriteText(cli.io.stderr); err != nil {
			return exitProcessing
		}
	}
	if jsonOutput {
		if err := cli.writeJSON(struct {
			OK          bool            `json:"ok"`
			Diagnostics diagnostic.List `json:"diagnostics"`
		}{OK: !diagnostics.HasErrors(), Diagnostics: nonNilDiagnostics(diagnostics)}); err != nil {
			fmt.Fprintln(cli.io.stderr, err)
			return exitProcessing
		}
	}
	if diagnostics.HasErrors() {
		return exitProcessing
	}
	if !jsonOutput {
		fmt.Fprintf(cli.io.stdout, "valid: %s\n", displayInput(path))
	}
	return exitSuccess
}

func (cli command) runCompile(ctx context.Context, args []string) int {
	if hasHelpFlag(args) {
		cli.helpCompile(cli.io.stdout)
		return exitSuccess
	}
	path, outPath, jsonOutput, ok := parseCompileArgs(args)
	if !ok {
		if hasJSONFlag(args) {
			return cli.usageFailure(true, "compile", errors.New("invalid compile arguments"))
		}
		cli.conciseCompile(cli.io.stderr)
		return exitUsage
	}
	if jsonOutput && outPath != "" && outPath != "-" {
		return cli.usageFailure(true, "compile", errors.New("--json writes one document to stdout and cannot be combined with --out FILE"))
	}
	program, diagnostics, err := cli.compileInput(ctx, path)
	if err != nil {
		return cli.fail(jsonOutput, err)
	}
	if len(diagnostics) > 0 {
		if err := diagnostics.WriteText(cli.io.stderr); err != nil {
			return exitProcessing
		}
	}
	if diagnostics.HasErrors() || program == nil {
		if jsonOutput {
			if err := cli.writeJSON(struct {
				OK          bool            `json:"ok"`
				Diagnostics diagnostic.List `json:"diagnostics"`
			}{OK: false, Diagnostics: nonNilDiagnostics(diagnostics)}); err != nil {
				return exitProcessing
			}
		}
		return exitProcessing
	}
	if jsonOutput {
		if err := cli.writeJSON(struct {
			OK          bool            `json:"ok"`
			Diagnostics diagnostic.List `json:"diagnostics"`
			IR          *ir.Extractor   `json:"ir"`
		}{OK: true, Diagnostics: nonNilDiagnostics(diagnostics), IR: program}); err != nil {
			fmt.Fprintln(cli.io.stderr, "marshal IR:", err)
			return exitProcessing
		}
		return exitSuccess
	}
	data, err := json.MarshalIndent(program, "", "  ")
	if err != nil {
		fmt.Fprintln(cli.io.stderr, "marshal IR:", err)
		return exitProcessing
	}
	return cli.writePrimary(append(data, '\n'), outPath, "IR")
}

func (cli command) runExtract(ctx context.Context, args []string) int {
	if hasHelpFlag(args) {
		cli.helpExtract(cli.io.stdout)
		return exitSuccess
	}
	if clisupport.HasPlaintextSecretFlag(args) {
		return cli.usageFailure(hasJSONFlag(args), "extract", errors.New(clisupport.PlaintextSecretFlagMessage))
	}
	path := ""
	if len(args) > 0 && (args[0] == "-" || !strings.HasPrefix(args[0], "-")) {
		path = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("extract", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var inputFlags repeatedFlag
	var htmlPath, outPath, sessionPath string
	var requestTimeout time.Duration
	var maxBody int64
	var userAgent string
	var sessionProvided, jsonOutput, allowPrivateHosts bool
	flags.Var(&inputFlags, "input", "runtime input as name=value (repeatable)")
	flags.StringVar(&sessionPath, "session-file", "", "read session JSON from a file, or - for standard input")
	flags.StringVar(&htmlPath, "html", "", "execute against decoded HTML from a file, or - for standard input")
	flags.StringVar(&outPath, "out", "", "write the result to a file, or - for standard output")
	flags.StringVar(&outPath, "o", "", "write the result to a file, or - for standard output")
	flags.DurationVar(&requestTimeout, "timeout", 30*time.Second, "HTTP request timeout")
	flags.Int64Var(&maxBody, "max-body", 32<<20, "maximum HTTP response body size in bytes")
	flags.StringVar(&userAgent, "user-agent", "", "HTTP User-Agent")
	flags.BoolVar(&sessionProvided, "session", false, "mark an empty runtime session as supplied")
	flags.BoolVar(&allowPrivateHosts, "allow-private-hosts", false, "allow targets that are not globally reachable")
	flags.BoolVar(&jsonOutput, "json", false, "emit exactly one JSON document on standard output")
	if err := flags.Parse(args); err != nil {
		return cli.usageFailure(hasJSONFlag(args), "extract", err)
	}
	if path == "" {
		if flags.NArg() != 1 {
			if jsonOutput {
				return cli.usageFailure(true, "extract", errors.New("exactly one KDL source is required"))
			}
			cli.conciseExtract(cli.io.stderr)
			return exitUsage
		}
		path = flags.Arg(0)
	} else if flags.NArg() != 0 {
		return cli.usageFailure(jsonOutput, "extract", errors.New("expected exactly one KDL source"))
	}
	if jsonOutput && outPath != "" && outPath != "-" {
		return cli.usageFailure(true, "extract", errors.New("--json writes one document to stdout and cannot be combined with --out FILE"))
	}
	stdinUsers := 0
	for _, value := range []string{path, htmlPath, sessionPath} {
		if value == "-" {
			stdinUsers++
		}
	}
	if stdinUsers > 1 {
		return cli.usageFailure(jsonOutput, "extract", errors.New("standard input can supply only one of KDL source, --html -, or --session-file -"))
	}
	program, diagnostics, err := cli.compileInput(ctx, path)
	if err != nil {
		return cli.fail(jsonOutput, err)
	}
	if len(diagnostics) > 0 {
		if err := diagnostics.WriteText(cli.io.stderr); err != nil {
			return exitProcessing
		}
	}
	if diagnostics.HasErrors() || program == nil {
		if jsonOutput {
			if err := cli.writeJSON(jsonFailure{OK: false, Error: cliError{Message: "compilation failed"}}); err != nil {
				return exitProcessing
			}
		}
		return exitProcessing
	}
	inputs, err := parseRuntimeInputs(program.Inputs, inputFlags)
	if err != nil {
		return cli.usageFailure(jsonOutput, "extract", err)
	}
	session, err := cli.readSessionFile(sessionPath)
	if err != nil {
		return cli.fail(jsonOutput, err)
	}
	if sessionProvided && session == nil {
		session = &executor.Session{Headers: make(http.Header)}
	}
	options := executor.Options{Session: session, RequestTimeout: requestTimeout, MaxResponseBytes: maxBody, UserAgent: userAgent}
	if !allowPrivateHosts && htmlPath == "" {
		options.URLPolicy = executor.PublicInternetURLPolicy()
		options.HTTPClient = executor.NewPublicInternetHTTPClient()
	}
	var result *executor.Result
	if htmlPath != "" {
		html, readErr := cli.readInput(htmlPath, "HTML")
		if readErr != nil {
			return cli.fail(jsonOutput, readErr)
		}
		result, err = executor.ExecuteHTML(ctx, program, string(html), options)
	} else {
		result, err = executor.Execute(ctx, program, inputs, options)
	}
	if err != nil {
		return cli.fail(jsonOutput, err)
	}
	warnings := make([]clisupport.Warning, len(result.Warnings))
	for index, warning := range result.Warnings {
		warnings[index] = clisupport.Warning{Code: warning.Code, Path: warning.Path, Message: warning.Message}
	}
	fmt.Fprint(cli.io.stderr, clisupport.FormatWarnings(warnings))
	data, err := clisupport.MarshalExtractionResult(result, jsonOutput)
	if err != nil {
		fmt.Fprintln(cli.io.stderr, "marshal result:", err)
		return exitProcessing
	}
	if jsonOutput {
		if _, err := cli.io.stdout.Write(data); err != nil {
			return exitProcessing
		}
		return exitSuccess
	}
	return cli.writePrimary(data, outPath, "result")
}

func (cli command) runVersion(args []string) int {
	if hasHelpFlag(args) {
		cli.helpVersion(cli.io.stdout)
		return exitSuccess
	}
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" && !jsonOutput {
			jsonOutput = true
			continue
		}
		return cli.usageError("version", fmt.Sprintf("unknown argument %q", arg))
	}
	if jsonOutput {
		if err := cli.writeJSON(struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
			Built   string `json:"built"`
		}{Version: version, Commit: commit, Built: date}); err != nil {
			fmt.Fprintln(cli.io.stderr, err)
			return exitProcessing
		}
	} else {
		fmt.Fprintf(cli.io.stdout, "scrape-kdl %s (commit=%s, built=%s)\n", version, commit, date)
	}
	return exitSuccess
}

func (cli command) compileInput(ctx context.Context, path string) (*ir.Extractor, diagnostic.List, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if path != "-" {
		return compiler.CompileFileContext(ctx, path)
	}
	data, err := io.ReadAll(cli.io.stdin)
	if err != nil {
		return nil, nil, fmt.Errorf("read KDL from standard input: %w", err)
	}
	return compiler.CompileSource(ctx, "<stdin>", data, func(ctx context.Context, path string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return os.ReadFile(path)
	})
}

func (cli command) readInput(path, label string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(cli.io.stdin)
		if err != nil {
			return nil, fmt.Errorf("read %s from standard input: %w", label, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return data, nil
}

func (cli command) writePrimary(data []byte, outPath, label string) int {
	if outPath == "" || outPath == "-" {
		if _, err := cli.io.stdout.Write(data); err != nil {
			fmt.Fprintf(cli.io.stderr, "write %s: %v\n", label, err)
			return exitProcessing
		}
		return exitSuccess
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fmt.Fprintf(cli.io.stderr, "write %s: %v\n", label, err)
		return exitProcessing
	}
	fmt.Fprintf(cli.io.stderr, "wrote: %s\n", outPath)
	return exitSuccess
}

type cliError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type jsonFailure struct {
	OK    bool     `json:"ok"`
	Error cliError `json:"error"`
}

func (cli command) fail(jsonOutput bool, err error) int {
	fmt.Fprintln(cli.io.stderr, err)
	if jsonOutput {
		failure := cliError{Message: err.Error()}
		var execution *executor.ExecutionError
		if errors.As(err, &execution) {
			failure = cliError{Code: execution.Code, Message: execution.Message, Path: execution.Path}
		}
		if writeErr := cli.writeJSON(jsonFailure{OK: false, Error: failure}); writeErr != nil {
			return exitProcessing
		}
	}
	return exitProcessing
}

func (cli command) usageFailure(jsonOutput bool, commandName string, err error) int {
	if jsonOutput {
		if writeErr := cli.writeJSON(jsonFailure{OK: false, Error: cliError{Message: err.Error()}}); writeErr != nil {
			return exitProcessing
		}
	}
	return cli.usageError(commandName, err.Error())
}

func (cli command) writeJSON(value any) error {
	encoder := json.NewEncoder(cli.io.stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (cli command) usageError(commandName, message string) int {
	fmt.Fprintln(cli.io.stderr, message)
	fmt.Fprintf(cli.io.stderr, "\nRun 'scrape-kdl %s --help' for usage.\n", commandName)
	return exitUsage
}

func displayInput(path string) string {
	if path == "-" {
		return "<stdin>"
	}
	return path
}

func nonNilDiagnostics(items diagnostic.List) diagnostic.List {
	if items == nil {
		return diagnostic.List{}
	}
	return items
}

func parseValidateArgs(args []string) (path string, jsonOutput bool, ok bool) {
	for _, arg := range args {
		switch arg {
		case "--json":
			if jsonOutput {
				return "", false, false
			}
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") && arg != "-" || path != "" {
				return "", false, false
			}
			path = arg
		}
	}
	return path, jsonOutput, path != ""
}

func parseCompileArgs(args []string) (path, outPath string, jsonOutput bool, ok bool) {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--emit-ir":
		case "--json":
			if jsonOutput {
				return "", "", false, false
			}
			jsonOutput = true
		case "--out", "-o":
			index++
			if index >= len(args) || outPath != "" {
				return "", "", false, false
			}
			outPath = args[index]
		default:
			if strings.HasPrefix(args[index], "-") && args[index] != "-" || path != "" {
				return "", "", false, false
			}
			path = args[index]
		}
	}
	return path, outPath, jsonOutput, path != ""
}

type repeatedFlag = clisupport.RepeatedFlag

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func hasJSONFlag(args []string) bool {
	return clisupport.HasJSONFlag(args)
}

func parseRuntimeInputs(definitions []ir.Input, values []string) (map[string]any, error) {
	declarations := make([]clisupport.InputDeclaration, len(definitions))
	for index, definition := range definitions {
		declarations[index] = clisupport.InputDeclaration{Name: definition.Name, Type: definition.Type, Required: definition.Required}
	}
	return clisupport.ParseRuntimeInputs(declarations, values)
}

func (cli command) readSessionFile(path string) (*executor.Session, error) {
	if path == "" {
		return nil, nil
	}
	data, err := cli.readInput(path, "session file")
	if err != nil {
		return nil, err
	}
	headers, cookies, err := clisupport.DecodeSessionDocument(data)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}
	return &executor.Session{Headers: headers, Cookies: cookies}, nil
}

func (cli command) conciseRoot(writer io.Writer) {
	fmt.Fprintln(writer, "scrape-kdl validates, compiles, and executes Scraping KDL specifications.")
	fmt.Fprintln(writer, "usage: scrape-kdl <validate|compile|extract|version> [options]")
	fmt.Fprintln(writer, "Run 'scrape-kdl --help' for examples and full command help.")
}

func (cli command) conciseValidate(writer io.Writer) {
	fmt.Fprintln(writer, "usage: scrape-kdl validate <file.kdl|-> [--json]\nRun 'scrape-kdl validate --help' for details.")
}
func (cli command) conciseCompile(writer io.Writer) {
	fmt.Fprintln(writer, "usage: scrape-kdl compile <file.kdl|-> [--json] [-o file.json|-]\nRun 'scrape-kdl compile --help' for details.")
}
func (cli command) conciseExtract(writer io.Writer) {
	fmt.Fprintln(writer, "usage: scrape-kdl extract <file.kdl|-> [options]\nRun 'scrape-kdl extract --help' for details.")
}

func (cli command) helpRoot(writer io.Writer) {
	fmt.Fprint(writer, `scrape-kdl validates, compiles, and executes Scraping KDL specifications.

USAGE
  scrape-kdl <command> [options]

EXAMPLES
  scrape-kdl validate extractor.kdl
  scrape-kdl compile extractor.kdl --out extractor.ir.json
  scrape-kdl extract extractor.kdl --input id=42
  cat extractor.kdl | scrape-kdl validate - --json

COMMANDS
  validate   Validate syntax and semantics
  compile    Compile a specification to Validated IR
  extract    Execute HTTP or offline HTML extraction
  version    Print build version information

OPTIONS
  -h, --help      Show full help
  --version       Print version information

Run 'scrape-kdl help <command>' for command help.
Documentation: https://github.com/hsblabs/scrape-kdl
`)
}

func (cli command) helpValidate(writer io.Writer) {
	fmt.Fprint(writer, `Validate a Scraping KDL specification without network or browser activity.

USAGE
  scrape-kdl validate <file.kdl|-> [--json]

EXAMPLES
  scrape-kdl validate extractor.kdl
  cat extractor.kdl | scrape-kdl validate - --json

OPTIONS
  --json      Emit one JSON result document on stdout
  -h, --help  Show full help
`)
}

func (cli command) helpCompile(writer io.Writer) {
	fmt.Fprint(writer, `Compile a Scraping KDL specification to deterministic Validated IR.

USAGE
  scrape-kdl compile <file.kdl|-> [--json] [-o file.json|-]

EXAMPLES
  scrape-kdl compile extractor.kdl
  scrape-kdl compile extractor.kdl --out extractor.ir.json
  cat extractor.kdl | scrape-kdl compile - --json

OPTIONS
  --json          Emit one JSON result document on stdout
  -o, --out PATH  Write bare IR to PATH; use - for stdout
  --emit-ir       Compatibility spelling; compile always emits IR
  -h, --help      Show full help
`)
}

func (cli command) helpExtract(writer io.Writer) {
	fmt.Fprint(writer, `Execute a validated HTTP extractor or run it against saved HTML.

USAGE
  scrape-kdl extract <file.kdl|-> [options]

EXAMPLES
  scrape-kdl extract extractor.kdl --input id=42
  scrape-kdl extract extractor.kdl --html page.html
  scrape-kdl extract extractor.kdl --session-file session.json --json
  cat page.html | scrape-kdl extract extractor.kdl --html -

OPTIONS
  --input NAME=VALUE   Runtime input; repeat for multiple inputs
  --html PATH          Use decoded HTML from PATH; use - for stdin
  --session-file PATH  Read headers and cookies from JSON; use - for stdin
  --session            Supply an explicit empty session
  --allow-private-hosts
                       Allow targets that are not globally reachable,
                       and restore ordinary proxy behavior
  --timeout DURATION   HTTP request timeout (default 30s)
  --max-body BYTES     Maximum HTTP response body (default 33554432)
  --user-agent VALUE   HTTP User-Agent
  --json               Emit one JSON result document on stdout
  -o, --out PATH       Write the bare result to PATH; use - for stdout
  -h, --help           Show full help

Secrets are accepted only through --session-file or explicit stdin. The removed
--header and --cookie flags are intentionally rejected to avoid process-list and
shell-history exposure.
`)
}

func (cli command) helpVersion(writer io.Writer) {
	fmt.Fprint(writer, `Print scrape-kdl build version information.

USAGE
  scrape-kdl version [--json]

OPTIONS
  --json      Emit one JSON document on stdout
  -h, --help  Show full help
`)
}
