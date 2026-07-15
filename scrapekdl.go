package scrapekdl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/compatibility"
	"github.com/hsblabs/scrape-kdl/internal/compiler"
	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/executor"
	"github.com/hsblabs/scrape-kdl/internal/ir"
)

type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Span struct {
	File  string   `json:"file"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Span     Span   `json:"span"`
	Path     string `json:"path,omitempty"`
}

type Diagnostics []Diagnostic

func (diagnostics Diagnostics) HasErrors() bool {
	for _, item := range diagnostics {
		if item.Severity == "error" {
			return true
		}
	}
	return false
}

type Program struct {
	extractor *ir.Extractor
}

type Source struct {
	Path string
	Data []byte
}

type SourceLoader func(context.Context, string) ([]byte, error)

type CompileOptions struct {
	Loader SourceLoader
}

func Compile(ctx context.Context, source Source, options CompileOptions) (*Program, Diagnostics) {
	if ctx == nil {
		panic("scrapekdl: nil context")
	}
	if source.Path == "" {
		return nil, Diagnostics{{Code: "E_KDL_SYNTAX", Severity: "error", Message: "source path must not be empty"}}
	}
	extractor, internalDiagnostics := compiler.CompileSource(ctx, source.Path, source.Data, compiler.SourceLoader(options.Loader))
	return newProgram(extractor, internalDiagnostics)
}

func Validate(ctx context.Context, source Source, options CompileOptions) Diagnostics {
	_, diagnostics := Compile(ctx, source, options)
	return diagnostics
}

func CompileFile(ctx context.Context, path string) (*Program, Diagnostics) {
	if ctx == nil {
		panic("scrapekdl: nil context")
	}
	extractor, internalDiagnostics := compiler.CompileFileContext(ctx, path)
	return newProgram(extractor, internalDiagnostics)
}

func newProgram(extractor *ir.Extractor, internalDiagnostics diagnostic.List) (*Program, Diagnostics) {
	diagnostics := convertDiagnostics(internalDiagnostics)
	if extractor == nil || diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return &Program{extractor: extractor}, diagnostics
}

func ValidateFile(ctx context.Context, path string) Diagnostics {
	_, diagnostics := CompileFile(ctx, path)
	return diagnostics
}

func (program *Program) Name() string    { return program.extractor.Name }
func (program *Program) Version() string { return program.extractor.Version }
func (program *Program) Capabilities() []string {
	return append([]string(nil), program.extractor.Capabilities...)
}

type SourceFile struct {
	Path          string `json:"path"`
	ModuleName    string `json:"moduleName,omitempty"`
	ModuleVersion string `json:"moduleVersion,omitempty"`
	SHA256        string `json:"sha256"`
}

type ProgramMetadata struct {
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	LanguageVersion string       `json:"languageVersion"`
	IRVersion       string       `json:"irVersion"`
	Files           []SourceFile `json:"files"`
	Capabilities    []string     `json:"capabilities"`
}

func (program *Program) Metadata() ProgramMetadata {
	files := make([]SourceFile, len(program.extractor.Files))
	for index, file := range program.extractor.Files {
		files[index] = SourceFile{
			Path: file.Path, ModuleName: file.ModuleName, ModuleVersion: file.ModuleVersion, SHA256: file.SHA256,
		}
	}
	return ProgramMetadata{
		Name: program.extractor.Name, Version: program.extractor.Version,
		LanguageVersion: program.extractor.LanguageVersion, IRVersion: program.extractor.IRVersion,
		Files: files, Capabilities: program.Capabilities(),
	}
}

func SupportedLanguageVersions() []string {
	return compatibility.SupportedLanguageVersions()
}

func SupportedIRVersions() []string {
	return compatibility.SupportedIRVersions()
}

func (program *Program) IRJSON() ([]byte, error) {
	return json.MarshalIndent(program.extractor, "", "  ")
}

type ExternalTransform func(context.Context, any) (any, error)
type BrowserElement any

type BrowserNavigateOptions struct {
	Timeout   time.Duration
	Session   *Session
	UserAgent string
}

type BrowserEvaluateOptions struct {
	Timeout time.Duration
	Scope   BrowserElement
}

type BrowserAdapter interface {
	Navigate(context.Context, string, BrowserNavigateOptions) error
	WaitFor(context.Context, string, string, time.Duration) error
	Click(context.Context, string, time.Duration) error
	Fill(context.Context, string, string, time.Duration) error
	Press(context.Context, string, string, time.Duration) error
	Scroll(context.Context, float64, float64) error
	WaitForNetworkIdle(context.Context, time.Duration, time.Duration) error
	Evaluate(context.Context, string, BrowserEvaluateOptions) (any, error)
	QueryAll(context.Context, BrowserElement, string) ([]BrowserElement, error)
	Text(context.Context, BrowserElement) (string, error)
	HTML(context.Context, BrowserElement) (string, error)
	Attribute(context.Context, BrowserElement, string) (string, bool, error)
}

type BrowserAdapterLease interface {
	Acquire(context.Context) (release func(), err error)
}

type CharsetDecoder func(body []byte, charset string) (string, error)
type URLPolicy func(context.Context, *url.URL) error

// NormalizeBrowserResult validates and normalizes a value before an adapter
// returns it from BrowserAdapter.Evaluate.
func NormalizeBrowserResult(value any) (any, error) {
	return executor.NormalizeBrowserResult(value)
}

type Session struct {
	Headers http.Header
	Cookies []*http.Cookie
}

type Options struct {
	Browser            BrowserAdapter
	AllowJavaScript    bool
	HTTPClient         *http.Client
	Session            *Session
	ExternalTransforms map[string]ExternalTransform
	CharsetDecoder     CharsetDecoder
	RequestTimeout     time.Duration
	MaxResponseBytes   int64
	UserAgent          string
	URLPolicy          URLPolicy
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Row     *int   `json:"row,omitempty"`
}

type Result struct {
	Value    map[string]any `json:"value"`
	Warnings []Warning      `json:"warnings"`
	Partial  bool           `json:"partial"`
}

type ExecutionError struct {
	Code    string
	Message string
	Path    string
	Cause   error
}

func (e *ExecutionError) Error() string {
	if e.Path != "" {
		return e.Code + " at " + e.Path + ": " + e.Message
	}
	return e.Code + ": " + e.Message
}
func (e *ExecutionError) Unwrap() error { return e.Cause }

func (program *Program) Extract(ctx context.Context, inputs map[string]any, options Options) (*Result, error) {
	result, err := executor.Execute(ctx, program.extractor, inputs, convertOptions(options))
	if err != nil {
		return nil, convertExecutionError(err)
	}
	return convertResult(result), nil
}

func (program *Program) ExtractHTML(ctx context.Context, html string, options Options) (*Result, error) {
	result, err := executor.ExecuteHTML(ctx, program.extractor, html, convertOptions(options))
	if err != nil {
		return nil, convertExecutionError(err)
	}
	return convertResult(result), nil
}

func convertOptions(options Options) executor.Options {
	external := make(map[string]executor.ExternalTransform, len(options.ExternalTransforms))
	for name, transform := range options.ExternalTransforms {
		external[name] = executor.ExternalTransform(transform)
	}
	var session *executor.Session
	if options.Session != nil {
		session = &executor.Session{Headers: options.Session.Headers, Cookies: options.Session.Cookies}
	}
	return executor.Options{
		Browser: convertBrowserAdapter(options.Browser), AllowJavaScript: options.AllowJavaScript,
		HTTPClient: options.HTTPClient, Session: session, ExternalTransforms: external, CharsetDecoder: executor.CharsetDecoder(options.CharsetDecoder),
		RequestTimeout: options.RequestTimeout, MaxResponseBytes: options.MaxResponseBytes, UserAgent: options.UserAgent,
		URLPolicy: executor.URLPolicy(options.URLPolicy),
	}
}

type browserAdapterBridge struct {
	adapter BrowserAdapter
}

func convertBrowserAdapter(adapter BrowserAdapter) executor.BrowserAdapter {
	if adapter == nil {
		return nil
	}
	bridge := browserAdapterBridge{adapter: adapter}
	if lease, ok := adapter.(BrowserAdapterLease); ok {
		return leasedBrowserAdapterBridge{browserAdapterBridge: bridge, lease: lease}
	}
	return bridge
}

func (bridge browserAdapterBridge) Navigate(ctx context.Context, target string, options executor.BrowserNavigateOptions) error {
	return bridge.adapter.Navigate(ctx, target, BrowserNavigateOptions{
		Timeout: options.Timeout, Session: convertInternalSession(options.Session), UserAgent: options.UserAgent,
	})
}

func (bridge browserAdapterBridge) WaitFor(ctx context.Context, selector, state string, timeout time.Duration) error {
	return bridge.adapter.WaitFor(ctx, selector, state, timeout)
}

func (bridge browserAdapterBridge) Click(ctx context.Context, selector string, timeout time.Duration) error {
	return bridge.adapter.Click(ctx, selector, timeout)
}

func (bridge browserAdapterBridge) Fill(ctx context.Context, selector, value string, timeout time.Duration) error {
	return bridge.adapter.Fill(ctx, selector, value, timeout)
}

func (bridge browserAdapterBridge) Press(ctx context.Context, selector, key string, timeout time.Duration) error {
	return bridge.adapter.Press(ctx, selector, key, timeout)
}

func (bridge browserAdapterBridge) Scroll(ctx context.Context, x, y float64) error {
	return bridge.adapter.Scroll(ctx, x, y)
}

func (bridge browserAdapterBridge) WaitForNetworkIdle(ctx context.Context, idle, timeout time.Duration) error {
	return bridge.adapter.WaitForNetworkIdle(ctx, idle, timeout)
}

func (bridge browserAdapterBridge) Evaluate(ctx context.Context, source string, options executor.BrowserEvaluateOptions) (any, error) {
	return bridge.adapter.Evaluate(ctx, source, BrowserEvaluateOptions{Timeout: options.Timeout, Scope: BrowserElement(options.Scope)})
}

func (bridge browserAdapterBridge) QueryAll(ctx context.Context, scope executor.BrowserElement, selector string) ([]executor.BrowserElement, error) {
	elements, err := bridge.adapter.QueryAll(ctx, BrowserElement(scope), selector)
	if err != nil {
		return nil, err
	}
	converted := make([]executor.BrowserElement, len(elements))
	for index, element := range elements {
		converted[index] = executor.BrowserElement(element)
	}
	return converted, nil
}

func (bridge browserAdapterBridge) Text(ctx context.Context, element executor.BrowserElement) (string, error) {
	return bridge.adapter.Text(ctx, BrowserElement(element))
}

func (bridge browserAdapterBridge) HTML(ctx context.Context, element executor.BrowserElement) (string, error) {
	return bridge.adapter.HTML(ctx, BrowserElement(element))
}

func (bridge browserAdapterBridge) Attribute(ctx context.Context, element executor.BrowserElement, name string) (string, bool, error) {
	return bridge.adapter.Attribute(ctx, BrowserElement(element), name)
}

type leasedBrowserAdapterBridge struct {
	browserAdapterBridge
	lease BrowserAdapterLease
}

func (bridge leasedBrowserAdapterBridge) Acquire(ctx context.Context) (func(), error) {
	return bridge.lease.Acquire(ctx)
}

func convertInternalSession(session *executor.Session) *Session {
	if session == nil {
		return nil
	}
	return &Session{Headers: session.Headers, Cookies: session.Cookies}
}

func convertResult(result *executor.Result) *Result {
	warnings := make([]Warning, len(result.Warnings))
	for index, warning := range result.Warnings {
		warnings[index] = Warning(warning)
	}
	return &Result{Value: result.Value, Warnings: warnings, Partial: result.Partial}
}

func convertExecutionError(err error) error {
	var internal *executor.ExecutionError
	if errors.As(err, &internal) {
		return &ExecutionError{Code: internal.Code, Message: internal.Message, Path: internal.Path, Cause: internal.Cause}
	}
	return err
}

func convertDiagnostics(items diagnostic.List) Diagnostics {
	items = items.Sorted()
	result := make(Diagnostics, len(items))
	for index, item := range items {
		result[index] = Diagnostic{
			Code: item.Code, Severity: string(item.Severity), Message: item.Message, Path: item.Path,
			Span: Span{
				File:  item.Span.File,
				Start: Position{Offset: item.Span.Start.Offset, Line: item.Span.Start.Line, Column: item.Span.Start.Column},
				End:   Position{Offset: item.Span.End.Offset, Line: item.Span.End.Line, Column: item.Span.End.Column},
			},
		}
	}
	return result
}
