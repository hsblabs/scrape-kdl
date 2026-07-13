package executor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

func TestTransformPreflightUsesDeclarationOrder(t *testing.T) {
	stringType := typesys.Primitive("string")
	first := ir.ExternalTransform{
		Kind:          "external",
		TransformBase: ir.TransformBase{SymbolID: "transform:first", Name: "first", Input: stringType, Output: stringType},
		Symbol:        "first_symbol",
	}
	second := ir.ExternalTransform{
		Kind:          "external",
		TransformBase: ir.TransformBase{SymbolID: "transform:second", Name: "second", Input: stringType, Output: stringType},
		Symbol:        "second_symbol",
	}
	extractor := &ir.Extractor{Transforms: []ir.Transform{first, second}}

	for range 100 {
		runtime := newTransformRuntime(context.Background(), extractor, map[string]ExternalTransform{})
		var execution *ExecutionError
		if err := runtime.preflight(); !errors.As(err, &execution) || execution.Code != "E_EXTERNAL_TRANSFORM_MISSING" || execution.Path != "first" {
			t.Fatalf("preflight error = %#v", err)
		}
	}
	runtime := newTransformRuntime(context.Background(), extractor, map[string]ExternalTransform{
		"first_symbol": func(context.Context, any) (any, error) { return "ok", nil },
	})
	var execution *ExecutionError
	if err := runtime.preflight(); !errors.As(err, &execution) || execution.Path != "second" {
		t.Fatalf("second preflight error = %#v", err)
	}
	runtime = newTransformRuntime(context.Background(), extractor, map[string]ExternalTransform{
		"first_symbol":  func(context.Context, any) (any, error) { return "one", nil },
		"second_symbol": func(context.Context, any) (any, error) { return "two", nil },
	})
	if err := runtime.preflight(); err != nil {
		t.Fatalf("complete preflight error = %v", err)
	}
}

func TestTransformRuntimeDetectsRecursion(t *testing.T) {
	stringType := typesys.Primitive("string")
	const symbol = "transform:recursive"
	pipeline := ir.PipelineTransform{
		Kind:          "pipeline",
		TransformBase: ir.TransformBase{SymbolID: symbol, Name: "recursive", Input: stringType, Output: stringType},
		Calls: []ir.TransformCall{{
			Target: ir.DeclaredTarget{Kind: "declared", SymbolID: symbol}, Input: stringType, Output: stringType,
		}},
	}
	runtime := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{pipeline}}, nil)
	_, err := runtime.applyDeclared(symbol, "value", "output.value")
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_TRANSFORM_RECURSION" || execution.Path != "output.value" {
		t.Fatalf("error = %#v", err)
	}
	if len(runtime.callStack) != 0 {
		t.Fatalf("call stack leaked after recursion: %v", runtime.callStack)
	}
}

func TestTransformRuntimeRejectsMalformedIR(t *testing.T) {
	stringType := typesys.Primitive("string")
	match := ir.MatchTransform{
		Kind:          "match",
		TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: stringType},
		Cases:         []ir.MatchCase{{When: json.RawMessage(`not-json`), Then: json.RawMessage(`"value"`)}},
		Default:       json.RawMessage(`"default"`),
	}
	runtime := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{match}}, nil)
	tests := []struct {
		name     string
		call     ir.TransformCall
		wantCode string
	}{
		{name: "nil target", call: ir.TransformCall{}, wantCode: "E_TRANSFORM"},
		{name: "unknown builtin", call: ir.TransformCall{Target: ir.BuiltinTarget{Kind: "builtin", Name: "missing"}}, wantCode: "E_TRANSFORM"},
		{name: "missing declared", call: ir.TransformCall{Target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:missing"}}, wantCode: "E_TRANSFORM_MISSING"},
		{name: "invalid match JSON", call: ir.TransformCall{Target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:match"}}, wantCode: "E_TRANSFORM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runtime.applyCalls("input", []ir.TransformCall{tt.call}, "output.value")
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || execution.Path != "output.value" {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestTransformRuntimePreservesExternalError(t *testing.T) {
	stringType := typesys.Primitive("string")
	external := ir.ExternalTransform{
		Kind:          "external",
		TransformBase: ir.TransformBase{SymbolID: "transform:external", Name: "external", Input: stringType, Output: stringType},
		Symbol:        "external_symbol",
	}
	cause := errors.New("host failure")
	runtime := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{external}}, map[string]ExternalTransform{
		"external_symbol": func(context.Context, any) (any, error) { return nil, cause },
	})
	_, err := runtime.applyDeclared(external.SymbolID, "input", "output.value")
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_EXTERNAL_TRANSFORM" || execution.Path != "output.value" || !errors.Is(err, cause) {
		t.Fatalf("error = %#v", err)
	}
}
