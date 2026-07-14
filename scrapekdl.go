package scrapekdl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

func CompileFile(path string) (*Program, Diagnostics) {
	extractor, internalDiagnostics := compiler.CompileFile(path)
	diagnostics := convertDiagnostics(internalDiagnostics)
	if extractor == nil || diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return &Program{extractor: extractor}, diagnostics
}

func ValidateFile(path string) Diagnostics {
	return convertDiagnostics(compiler.ValidateFile(path))
}

func (program *Program) Name() string    { return program.extractor.Name }
func (program *Program) Version() string { return program.extractor.Version }
func (program *Program) Capabilities() []string {
	return append([]string(nil), program.extractor.Capabilities...)
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
type BrowserElement = executor.BrowserElement
type BrowserNavigateOptions = executor.BrowserNavigateOptions
type BrowserEvaluateOptions = executor.BrowserEvaluateOptions
type BrowserAdapter = executor.BrowserAdapter
type BrowserAdapterLease = executor.BrowserAdapterLease
type CharsetDecoder func(body []byte, charset string) (string, error)
type URLPolicy = executor.URLPolicy

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
		Browser: options.Browser, AllowJavaScript: options.AllowJavaScript,
		HTTPClient: options.HTTPClient, Session: session, ExternalTransforms: external, CharsetDecoder: executor.CharsetDecoder(options.CharsetDecoder),
		RequestTimeout: options.RequestTimeout, MaxResponseBytes: options.MaxResponseBytes, UserAgent: options.UserAgent,
		URLPolicy: options.URLPolicy,
	}
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
