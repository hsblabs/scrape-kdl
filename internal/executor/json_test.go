package executor

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func TestDecodeJSONRequiresSingleValue(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want any
	}{
		{name: "null", raw: "null", want: nil},
		{name: "number", raw: "123", want: json.Number("123")},
		{name: "object", raw: `{"items":[true,"value"]}`, want: map[string]any{"items": []any{true, "value"}}},
		{name: "surrounding whitespace", raw: " \n 1 \t", want: json.Number("1")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeJSON(json.RawMessage(tt.raw))
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("decodeJSON(%q) = %#v, %v; want %#v", tt.raw, got, err, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name string
		raw  string
	}{
		{name: "empty"},
		{name: "truncated", raw: `{"value"`},
		{name: "second value", raw: "1 2"},
		{name: "trailing token", raw: "1 invalid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if value, err := decodeJSON(json.RawMessage(tt.raw)); err == nil {
				t.Fatalf("decodeJSON(%q) = %#v, want error", tt.raw, value)
			}
		})
	}
}

func TestExecuteHTMLRejectsTrailingDataInDefault(t *testing.T) {
	path := compileTestSpec(t, `extractor "offline-default" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "value" type="string" required=#false default="fallback" { select ".missing"; value "text"; on-error "default" }
}`)
	extractor, diagnostics := compileFile(t, path)
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	result, err := ExecuteHTML(context.Background(), extractor, `<main></main>`, Options{})
	if err != nil || result.Value["value"] != "fallback" {
		t.Fatalf("valid default result = %#v, error = %v", result, err)
	}

	field := extractor.Output.Members[0].(ir.Field)
	malformed := json.RawMessage(`"fallback" "trailing"`)
	field.Default = &malformed
	extractor.Output.Members[0] = field
	_, err = ExecuteHTML(context.Background(), extractor, `<main></main>`, Options{})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "output.value" || execution.Cause == nil {
		t.Fatalf("malformed default error = %#v", err)
	}
}
