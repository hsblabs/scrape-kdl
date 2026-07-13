package executor

import (
	"errors"
	"math"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func TestExpandTemplateEncodesScalarInputs(t *testing.T) {
	template := ir.Template{Segments: []ir.TemplateSegment{
		ir.LiteralTemplateSegment{Kind: "literal", Value: "https://example.invalid/"},
		ir.InputTemplateSegment{Kind: "input", Name: "text"},
		ir.LiteralTemplateSegment{Kind: "literal", Value: "/"},
		ir.InputTemplateSegment{Kind: "input", Name: "enabled"},
		ir.LiteralTemplateSegment{Kind: "literal", Value: "/"},
		ir.InputTemplateSegment{Kind: "input", Name: "count"},
		ir.LiteralTemplateSegment{Kind: "literal", Value: "/"},
		ir.InputTemplateSegment{Kind: "input", Name: "ratio"},
	}}
	got, err := expandTemplate(template, map[string]any{
		"text":    "a /あ~",
		"enabled": true,
		"count":   int64(-2),
		"ratio":   float64(1.25),
	})
	want := "https://example.invalid/a%20%2F%E3%81%82~/true/-2/1.25"
	if err != nil || got != want {
		t.Fatalf("expandTemplate = %q, %v; want %q", got, err, want)
	}

	literal, err := expandTemplate(ir.Template{Segments: []ir.TemplateSegment{
		ir.LiteralTemplateSegment{Kind: "literal", Value: "https://example.invalid/{literal}"},
	}}, nil)
	if err != nil || literal != "https://example.invalid/{literal}" {
		t.Fatalf("literal braces = %q, %v", literal, err)
	}
}

func TestExpandTemplateRejectsInvalidInputsAndTargets(t *testing.T) {
	inputTemplate := ir.Template{Segments: []ir.TemplateSegment{
		ir.LiteralTemplateSegment{Kind: "literal", Value: "https://example.invalid/"},
		ir.InputTemplateSegment{Kind: "input", Name: "value"},
	}}
	tests := []struct {
		name      string
		template  ir.Template
		inputs    map[string]any
		wantCode  string
		wantPath  string
		wantCause bool
	}{
		{name: "missing", template: inputTemplate, wantCode: "E_INPUT_REQUIRED", wantPath: "input.value"},
		{name: "invalid type", template: inputTemplate, inputs: map[string]any{"value": []string{"x"}}, wantCode: "E_INPUT_TYPE", wantPath: "input.value", wantCause: true},
		{name: "non-finite float", template: inputTemplate, inputs: map[string]any{"value": math.NaN()}, wantCode: "E_INPUT_TYPE", wantPath: "input.value", wantCause: true},
		{name: "relative", template: ir.Template{Segments: []ir.TemplateSegment{ir.LiteralTemplateSegment{Value: "/path"}}}, wantCode: "E_URL_INVALID"},
		{name: "unsupported scheme", template: ir.Template{Segments: []ir.TemplateSegment{ir.LiteralTemplateSegment{Value: "ftp://example.invalid/path"}}}, wantCode: "E_URL_INVALID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := expandTemplate(tt.template, tt.inputs)
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || execution.Path != tt.wantPath || tt.wantCause && execution.Cause == nil {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestPercentEncodePreservesOnlyUnreservedBytes(t *testing.T) {
	if got, want := percentEncode("AZaz09-._~:/?#[]@!$&'()*+,;= あ"), "AZaz09-._~%3A%2F%3F%23%5B%5D%40%21%24%26%27%28%29%2A%2B%2C%3B%3D%20%E3%81%82"; got != want {
		t.Fatalf("percentEncode = %q, want %q", got, want)
	}
}
