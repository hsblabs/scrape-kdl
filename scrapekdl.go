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

// Position identifies a UTF-8 byte offset and one-based line and column.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Span identifies a half-open source range.
type Span struct {
	File  string   `json:"file"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic is a stable compiler finding suitable for machine processing.
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Span     Span   `json:"span"`
	Path     string `json:"path,omitempty"`
}

// Diagnostics is a deterministically ordered collection of compiler findings.
type Diagnostics []Diagnostic

// HasErrors reports whether diagnostics contains at least one error.
func (diagnostics Diagnostics) HasErrors() bool {
	for _, item := range diagnostics {
		if item.Severity == "error" {
			return true
		}
	}
	return false
}

// Program is an immutable compiled extractor with a reusable execution plan.
type Program struct {
	extractor *ir.Extractor
	prepared  *executor.Prepared
}

// Source supplies an in-memory KDL document and its logical path.
type Source struct {
	Path string
	Data []byte
}

// SourceLoader resolves an imported logical path. It must honor cancellation.
type SourceLoader func(context.Context, string) ([]byte, error)

// CompileOptions controls import loading during compilation.
type CompileOptions struct {
	Loader SourceLoader
}

// Compile validates and compiles an in-memory source into an immutable Program.
// Document findings are returned as Diagnostics; cancellation and source-loading
// failures are returned as an error that preserves the original cause.
func Compile(ctx context.Context, source Source, options CompileOptions) (*Program, Diagnostics, error) {
	if ctx == nil {
		panic("scrapekdl: nil context")
	}
	if source.Path == "" {
		return nil, nil, errors.New("scrapekdl: source path must not be empty")
	}
	extractor, internalDiagnostics, err := compiler.CompileSource(ctx, source.Path, source.Data, compiler.SourceLoader(options.Loader))
	if err != nil {
		return nil, nil, err
	}
	return newProgram(extractor, internalDiagnostics)
}

// Validate returns diagnostics without retaining the compiled Program and
// returns operational failures separately.
func Validate(ctx context.Context, source Source, options CompileOptions) (Diagnostics, error) {
	_, diagnostics, err := Compile(ctx, source, options)
	return diagnostics, err
}

// CompileFile compiles a KDL file and resolves imports from the filesystem.
// Filesystem and cancellation failures are returned separately from Diagnostics.
func CompileFile(ctx context.Context, path string) (*Program, Diagnostics, error) {
	if ctx == nil {
		panic("scrapekdl: nil context")
	}
	extractor, internalDiagnostics, err := compiler.CompileFileContext(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	return newProgram(extractor, internalDiagnostics)
}

func newProgram(extractor *ir.Extractor, internalDiagnostics diagnostic.List) (*Program, Diagnostics, error) {
	diagnostics := convertDiagnostics(internalDiagnostics)
	if extractor == nil || diagnostics.HasErrors() {
		return nil, diagnostics, nil
	}
	prepared, err := executor.Prepare(extractor)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{Code: "E_IR_INVALID", Severity: "error", Message: err.Error()})
		return nil, diagnostics, nil
	}
	return &Program{extractor: extractor, prepared: prepared}, diagnostics, nil
}

// ValidateFile validates a KDL file and its imports and returns operational
// failures separately.
func ValidateFile(ctx context.Context, path string) (Diagnostics, error) {
	_, diagnostics, err := CompileFile(ctx, path)
	return diagnostics, err
}

// Name returns the extractor name.
func (program *Program) Name() string { return program.extractor.Name }

// Version returns the extractor document version.
func (program *Program) Version() string { return program.extractor.Version }

// Capabilities returns a defensive copy of the sorted required capabilities.
func (program *Program) Capabilities() []string {
	return append([]string(nil), program.extractor.Capabilities...)
}

// SourceFile records one source document included in a Program.
type SourceFile struct {
	Path          string `json:"path"`
	ModuleName    string `json:"moduleName,omitempty"`
	ModuleVersion string `json:"moduleVersion,omitempty"`
	SHA256        string `json:"sha256"`
}

// ProgramMetadata describes a Program without exposing mutable internal IR.
type ProgramMetadata struct {
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	LanguageVersion string       `json:"languageVersion"`
	IRVersion       string       `json:"irVersion"`
	Files           []SourceFile `json:"files"`
	Capabilities    []string     `json:"capabilities"`
}

// Metadata returns defensive copies of the Program metadata.
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

// SupportedLanguageVersions returns a new slice of accepted language identifiers.
func SupportedLanguageVersions() []string {
	return compatibility.SupportedLanguageVersions()
}

// SupportedIRVersions returns a new slice of accepted IR identifiers.
func SupportedIRVersions() []string {
	return compatibility.SupportedIRVersions()
}

// IRJSON returns the Program's validated IR as indented JSON.
func (program *Program) IRJSON() ([]byte, error) {
	return json.MarshalIndent(program.extractor, "", "  ")
}

// ExternalTransform implements a declared host transform and must honor cancellation.
type ExternalTransform func(context.Context, any) (any, error)

// BrowserElement is an opaque handle owned by its BrowserAdapter.
type BrowserElement any

// BrowserNavigateOptions carries bounded navigation and session settings.
type BrowserNavigateOptions struct {
	Timeout   time.Duration
	Session   *Session
	UserAgent string
}

// BrowserEvaluateOptions carries bounded JavaScript evaluation settings.
type BrowserEvaluateOptions struct {
	Timeout time.Duration
	Scope   BrowserElement
}

// BrowserAdapter is the browser-library-neutral execution seam. Implementations
// own their element handles and must preserve context cancellation.
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

// BrowserAdapterLease serializes a complete extraction over a mutable page.
type BrowserAdapterLease interface {
	Acquire(context.Context) (release func(), err error)
}

// BrowserAdapterQueryLimit is an optional adapter capability for bounded
// selector queries used by first/one field cardinality.
type BrowserAdapterQueryLimit interface {
	QueryLimit(context.Context, BrowserElement, string, int) ([]BrowserElement, error)
}

// CharsetDecoder decodes bounded response bytes for a non-default charset.
type CharsetDecoder func(body []byte, charset string) (string, error)

// URLPolicy authorizes initial targets, HTTP redirects, and initial browser
// navigation targets.
type URLPolicy func(context.Context, *url.URL) error

// PublicInternetURLPolicy rejects targets that are not plain public-internet
// HTTP(S) URLs: other schemes, userinfo, and addresses that IANA does not mark
// globally reachable in its special-purpose registries.
// Pair it with NewPublicInternetHTTPClient to also cover DNS rebinding.
func PublicInternetURLPolicy() URLPolicy {
	return URLPolicy(executor.PublicInternetURLPolicy())
}

// NewPublicInternetHTTPClient returns a direct, proxy-disabled HTTP client
// whose dialer rejects non-public addresses after DNS resolution.
func NewPublicInternetHTTPClient() *http.Client {
	return executor.NewPublicInternetHTTPClient()
}

// NormalizeBrowserResult validates and normalizes a value before an adapter
// returns it from BrowserAdapter.Evaluate.
func NormalizeBrowserResult(value any) (any, error) {
	return executor.NormalizeBrowserResult(value)
}

// Session contains sensitive request state. Runtimes do not log its values.
type Session struct {
	Headers http.Header
	Cookies []*http.Cookie
}

// Options configures one extraction. Mutable state is extraction-local.
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

// Warning describes a recovered extraction error.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Row     *int   `json:"row,omitempty"`
}

// Result contains the extracted value and deterministic recovery warnings.
type Result struct {
	Value    map[string]any `json:"value"`
	Warnings []Warning      `json:"warnings"`
	Partial  bool           `json:"partial"`
}

// ExecutionError is a stable runtime failure with an optional path and cause.
type ExecutionError struct {
	Code    string
	Message string
	Path    string
	Cause   error
}

// Error formats the stable code, optional path, and human-readable message.
func (e *ExecutionError) Error() string {
	if e.Path != "" {
		return e.Code + " at " + e.Path + ": " + e.Message
	}
	return e.Code + ": " + e.Message
}

// Unwrap returns the underlying adapter, context, or runtime cause.
func (e *ExecutionError) Unwrap() error { return e.Cause }

// Extract executes the Program through its configured HTTP or browser runtime.
func (program *Program) Extract(ctx context.Context, inputs map[string]any, options Options) (*Result, error) {
	result, err := program.prepared.Execute(ctx, inputs, convertOptions(options))
	if err != nil {
		return nil, convertExecutionError(err)
	}
	return convertResult(result), nil
}

// ExtractHTML executes an HTTP-mode Program against already-decoded HTML.
func (program *Program) ExtractHTML(ctx context.Context, html string, options Options) (*Result, error) {
	result, err := program.prepared.ExecuteHTML(ctx, html, convertOptions(options))
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

func (bridge browserAdapterBridge) QueryLimit(ctx context.Context, scope executor.BrowserElement, selector string, limit int) ([]executor.BrowserElement, error) {
	limited, ok := bridge.adapter.(BrowserAdapterQueryLimit)
	if !ok {
		return bridge.QueryAll(ctx, scope, selector)
	}
	elements, err := limited.QueryLimit(ctx, BrowserElement(scope), selector, limit)
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
