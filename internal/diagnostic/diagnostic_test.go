package diagnostic

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/source"
)

func TestSortedIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	input := List{
		diagnosticAt("E_TYPE_UNKNOWN", SeverityError, "b.kdl", 4),
		diagnosticAt("E_DUPLICATE_SYMBOL", SeverityError, "a.kdl", 2),
		diagnosticAt("E_ARGUMENT_COUNT", SeverityWarning, "a.kdl", 2),
		diagnosticAt("E_DOCUMENT_ROOT", SeverityError, "a.kdl", 1),
	}
	original := append(List(nil), input...)

	got := input.Sorted()
	wantCodes := []string{"E_DOCUMENT_ROOT", "E_ARGUMENT_COUNT", "E_DUPLICATE_SYMBOL", "E_TYPE_UNKNOWN"}
	for i, want := range wantCodes {
		if got[i].Code != want {
			t.Fatalf("sorted[%d].Code = %q, want %q", i, got[i].Code, want)
		}
	}
	for i := range input {
		if input[i] != original[i] {
			t.Fatalf("Sorted mutated input at index %d: got %#v, want %#v", i, input[i], original[i])
		}
	}
}

func TestHasErrorsDistinguishesWarnings(t *testing.T) {
	if (List{diagnosticAt("W_JAVASCRIPT_PRESENT", SeverityWarning, "a.kdl", 0)}).HasErrors() {
		t.Fatal("warning-only list reports errors")
	}
	if !(List{diagnosticAt("E_KDL_SYNTAX", SeverityError, "a.kdl", 0)}).HasErrors() {
		t.Fatal("error list does not report errors")
	}
}

func TestWriteTextSortsAndIncludesOptionalPath(t *testing.T) {
	diagnostics := List{
		{Code: "E_TYPE_MISMATCH", Severity: SeverityError, Message: "second", Span: spanAt("b.kdl", 8, 3, 4)},
		{Code: "W_JAVASCRIPT_PRESENT", Severity: SeverityWarning, Message: "first", Span: spanAt("a.kdl", 2, 1, 2), Path: "output.title"},
	}
	var output bytes.Buffer

	if err := diagnostics.WriteText(&output); err != nil {
		t.Fatal(err)
	}

	want := "a.kdl:1:2: warning W_JAVASCRIPT_PRESENT: first [output.title]\n" +
		"b.kdl:3:4: error E_TYPE_MISMATCH: second\n"
	if output.String() != want {
		t.Fatalf("text output = %q, want %q", output.String(), want)
	}
}

func TestWriteJSONSortsAndPropagatesWriterFailure(t *testing.T) {
	diagnostics := List{
		diagnosticAt("E_TYPE_MISMATCH", SeverityError, "b.kdl", 2),
		diagnosticAt("E_KDL_SYNTAX", SeverityError, "a.kdl", 1),
	}
	var output bytes.Buffer
	if err := diagnostics.WriteJSON(&output); err != nil {
		t.Fatal(err)
	}
	if strings.Index(output.String(), "E_KDL_SYNTAX") > strings.Index(output.String(), "E_TYPE_MISMATCH") {
		t.Fatalf("JSON diagnostics are not sorted: %s", output.String())
	}

	wantErr := errors.New("write failed")
	if err := diagnostics.WriteJSON(errorWriter{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("WriteJSON error = %v, want %v", err, wantErr)
	}
}

func TestWriteTextPropagatesWriterFailure(t *testing.T) {
	wantErr := errors.New("write failed")
	diagnostics := List{diagnosticAt("E_KDL_SYNTAX", SeverityError, "a.kdl", 1)}
	if err := diagnostics.WriteText(errorWriter{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("WriteText error = %v, want %v", err, wantErr)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func diagnosticAt(code string, severity Severity, file string, offset int) Diagnostic {
	return Diagnostic{Code: code, Severity: severity, Span: spanAt(file, offset, 1, offset+1)}
}

func spanAt(file string, offset, line, column int) source.Span {
	position := source.Position{Offset: offset, Line: line, Column: column}
	return source.Span{File: file, Start: position, End: position}
}
