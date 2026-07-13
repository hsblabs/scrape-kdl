package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

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

func finalizeResult(value map[string]any, warnings []Warning, partial bool) *Result {
	if warnings == nil {
		warnings = []Warning{}
	}
	if partial {
		warnings = append(warnings, Warning{
			Code:    "W_PARTIAL_EXTRACTION",
			Message: "extraction completed with one or more recovered errors",
		})
	}
	return &Result{Value: value, Warnings: warnings, Partial: partial}
}

type ExecutionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Cause   error  `json:"-"`
}

func (e *ExecutionError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s at %s: %s", e.Code, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ExecutionError) Unwrap() error { return e.Cause }

func operationErrorCode(fallback string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "E_TIMEOUT"
	}
	return fallback
}

type ExternalTransform func(context.Context, any) (any, error)
type CharsetDecoder func(body []byte, charset string) (string, error)

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

func (o Options) withDefaults() Options {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{}
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = 30 * time.Second
	}
	if o.MaxResponseBytes <= 0 {
		o.MaxResponseBytes = 32 << 20
	}
	if o.UserAgent == "" {
		o.UserAgent = "scrape-kdl/0.1"
	}
	if o.ExternalTransforms == nil {
		o.ExternalTransforms = map[string]ExternalTransform{}
	}
	return o
}

func decodeJSONValue(raw *json.RawMessage) (any, error) {
	if raw == nil {
		return nil, nil
	}
	return decodeJSON(*raw)
}
