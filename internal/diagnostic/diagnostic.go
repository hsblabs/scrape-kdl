package diagnostic

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/hsblabs/scrape-kdl/internal/source"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Diagnostic struct {
	Code     string      `json:"code"`
	Severity Severity    `json:"severity"`
	Message  string      `json:"message"`
	Span     source.Span `json:"span"`
	Path     string      `json:"path,omitempty"`
}

type List []Diagnostic

func (l *List) Add(code string, severity Severity, message string, span source.Span, path string) {
	*l = append(*l, Diagnostic{Code: code, Severity: severity, Message: message, Span: span, Path: path})
}

func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (l List) Sorted() List {
	out := append(List(nil), l...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Span.File != b.Span.File {
			return a.Span.File < b.Span.File
		}
		if a.Span.Start.Offset != b.Span.Start.Offset {
			return a.Span.Start.Offset < b.Span.Start.Offset
		}
		return a.Code < b.Code
	})
	return out
}

func (l List) WriteText(w io.Writer) {
	for _, d := range l.Sorted() {
		path := ""
		if d.Path != "" {
			path = " [" + d.Path + "]"
		}
		fmt.Fprintf(w, "%s:%d:%d: %s %s: %s%s\n",
			d.Span.File,
			d.Span.Start.Line,
			d.Span.Start.Column,
			d.Severity,
			d.Code,
			d.Message,
			path,
		)
	}
}

func (l List) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(l.Sorted())
}
