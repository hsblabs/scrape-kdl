package executor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

func TestTransformPreflightRejectsDuplicateSymbols(t *testing.T) {
	stringType := typesys.Primitive("string")
	base := ir.TransformBase{SymbolID: "transform:duplicate", Input: stringType, Output: stringType}
	extractor := &ir.Extractor{Transforms: []ir.Transform{
		ir.PipelineTransform{Kind: "pipeline", TransformBase: base},
		ir.MatchTransform{Kind: "match", TransformBase: base, Default: json.RawMessage(`"fallback"`)},
	}}
	var execution *ExecutionError
	err := newTransformRuntime(context.Background(), extractor, nil).preflight()
	if !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "transforms" || !strings.Contains(execution.Message, `"transform:duplicate"`) {
		t.Fatalf("duplicate symbol preflight error = %#v", err)
	}
}

func TestTransformPreflightValidatesCalls(t *testing.T) {
	stringType := typesys.Primitive("string")
	validCall := ir.TransformCall{
		Target: ir.BuiltinTarget{Kind: "builtin", Name: "trim"}, Input: stringType, Output: stringType,
	}
	newExtractor := func() *ir.Extractor {
		return &ir.Extractor{Output: ir.OutputObject{Members: []ir.OutputMember{ir.Field{
			ID: "output.value", Transforms: []ir.TransformCall{validCall},
		}}}}
	}
	if err := newTransformRuntime(context.Background(), newExtractor(), nil).preflight(); err != nil {
		t.Fatalf("valid preflight error = %v", err)
	}

	tests := []struct {
		name     string
		target   ir.TransformTarget
		wantCode string
	}{
		{name: "nil target", wantCode: "E_TRANSFORM"},
		{name: "unknown builtin", target: ir.BuiltinTarget{Kind: "builtin", Name: "missing"}, wantCode: "E_TRANSFORM"},
		{name: "missing declared", target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:missing"}, wantCode: "E_TRANSFORM_MISSING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := newExtractor()
			field := extractor.Output.Members[0].(ir.Field)
			field.Transforms[0].Target = tt.target
			extractor.Output.Members[0] = field
			err := newTransformRuntime(context.Background(), extractor, nil).preflight()
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || execution.Path != "output.value" {
				t.Fatalf("preflight error = %#v", err)
			}
		})
	}

	argumentTests := []struct {
		name        string
		call        ir.TransformCall
		wantMessage string
	}{
		{
			name: "valid arguments",
			call: ir.TransformCall{
				Target:              ir.BuiltinTarget{Kind: "builtin", Name: "assert-enum"},
				PositionalArguments: []json.RawMessage{json.RawMessage(`"allowed"`)},
			},
		},
		{
			name: "invalid positional JSON",
			call: ir.TransformCall{
				Target:              ir.BuiltinTarget{Kind: "builtin", Name: "assert-enum"},
				PositionalArguments: []json.RawMessage{json.RawMessage(`"allowed" trailing`)},
			},
			wantMessage: "invalid positional transform argument 0",
		},
		{
			name: "duplicate named argument",
			call: ir.TransformCall{
				Target: ir.BuiltinTarget{Kind: "builtin", Name: "prepend"},
				NamedArguments: []ir.NamedArgument{
					{Name: "value", Value: json.RawMessage(`"first"`)},
					{Name: "value", Value: json.RawMessage(`"second"`)},
				},
			},
			wantMessage: `duplicate transform argument "value"`,
		},
		{
			name: "invalid named JSON",
			call: ir.TransformCall{
				Target: ir.BuiltinTarget{Kind: "builtin", Name: "prepend"},
				NamedArguments: []ir.NamedArgument{
					{Name: "value", Value: json.RawMessage(`not-json`)},
				},
			},
			wantMessage: `invalid transform argument "value"`,
		},
		{
			name: "unknown named argument",
			call: ir.TransformCall{
				Target:         ir.BuiltinTarget{Kind: "builtin", Name: "trim"},
				NamedArguments: []ir.NamedArgument{{Name: "unknown", Value: json.RawMessage(`true`)}},
			},
			wantMessage: `argument "unknown" is not allowed`,
		},
		{
			name:        "missing required argument",
			call:        ir.TransformCall{Target: ir.BuiltinTarget{Kind: "builtin", Name: "prepend"}},
			wantMessage: `requires argument "value"`,
		},
		{
			name: "disallowed positional argument",
			call: ir.TransformCall{
				Target:              ir.BuiltinTarget{Kind: "builtin", Name: "trim"},
				PositionalArguments: []json.RawMessage{json.RawMessage(`"value"`)},
			},
			wantMessage: "accepts 0..0 positional arguments",
		},
		{
			name:        "missing enum value",
			call:        ir.TransformCall{Target: ir.BuiltinTarget{Kind: "builtin", Name: "assert-enum"}},
			wantMessage: "requires at least 1 positional arguments",
		},
	}
	for _, tt := range argumentTests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := newExtractor()
			field := extractor.Output.Members[0].(ir.Field)
			field.Transforms[0] = tt.call
			extractor.Output.Members[0] = field
			err := newTransformRuntime(context.Background(), extractor, nil).preflight()
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("preflight error = %v", err)
				}
				return
			}
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != "E_TRANSFORM" || execution.Path != "output.value" || execution.Cause == nil || !strings.Contains(execution.Message, tt.wantMessage) {
				t.Fatalf("preflight error = %#v", err)
			}
		})
	}

	declared := ir.PipelineTransform{
		Kind:          "pipeline",
		TransformBase: ir.TransformBase{SymbolID: "transform:declared", Name: "declared", Input: stringType, Output: stringType},
	}
	extractor := newExtractor()
	extractor.Transforms = []ir.Transform{declared}
	field := extractor.Output.Members[0].(ir.Field)
	field.Transforms[0] = ir.TransformCall{
		Target:              ir.DeclaredTarget{Kind: "declared", SymbolID: declared.SymbolID},
		PositionalArguments: []json.RawMessage{json.RawMessage(`"value"`)},
	}
	extractor.Output.Members[0] = field
	err := newTransformRuntime(context.Background(), extractor, nil).preflight()
	var argumentExecution *ExecutionError
	if !errors.As(err, &argumentExecution) || argumentExecution.Code != "E_TRANSFORM" || argumentExecution.Path != "output.value" || !strings.Contains(argumentExecution.Message, "accepts no arguments") {
		t.Fatalf("declared argument preflight error = %#v", err)
	}

	pipeline := ir.PipelineTransform{
		Kind:          "pipeline",
		TransformBase: ir.TransformBase{SymbolID: "transform:pipeline", Name: "pipeline", Input: stringType, Output: stringType},
		Calls:         []ir.TransformCall{{Target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:missing"}}},
	}
	err = newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{pipeline}}, nil).preflight()
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_TRANSFORM_MISSING" || execution.Path != "pipeline" {
		t.Fatalf("pipeline preflight error = %#v", err)
	}
}

func TestTransformPreflightValidatesMatchLiterals(t *testing.T) {
	stringType := typesys.Primitive("string")
	valid := ir.MatchTransform{
		Kind:          "match",
		TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: stringType},
		Cases:         []ir.MatchCase{{When: json.RawMessage(`"input"`), Then: json.RawMessage(`"output"`)}},
		Default:       json.RawMessage(`"fallback"`),
	}
	if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{valid}}, nil).preflight(); err != nil {
		t.Fatalf("valid match preflight error = %v", err)
	}

	tests := []struct {
		name        string
		mutate      func(*ir.MatchTransform)
		wantMessage string
	}{
		{name: "malformed case input", mutate: func(match *ir.MatchTransform) { match.Cases[0].When = json.RawMessage(`not-json`) }, wantMessage: "invalid match case 0 input"},
		{name: "incompatible case input", mutate: func(match *ir.MatchTransform) { match.Cases[0].When = json.RawMessage(`1`) }, wantMessage: "match case 0 input"},
		{name: "malformed case result", mutate: func(match *ir.MatchTransform) { match.Cases[0].Then = json.RawMessage(`not-json`) }, wantMessage: "invalid match case 0 result"},
		{name: "incompatible case result", mutate: func(match *ir.MatchTransform) { match.Cases[0].Then = json.RawMessage(`1`) }, wantMessage: "match case 0 result"},
		{name: "malformed default", mutate: func(match *ir.MatchTransform) { match.Default = json.RawMessage(`not-json`) }, wantMessage: "invalid match default"},
		{name: "incompatible default", mutate: func(match *ir.MatchTransform) { match.Default = json.RawMessage(`1`) }, wantMessage: "match default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := valid
			match.Cases = append([]ir.MatchCase(nil), valid.Cases...)
			tt.mutate(&match)
			err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{match}}, nil).preflight()
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != "E_TRANSFORM" || execution.Path != "match" || execution.Cause == nil || !strings.Contains(execution.Message, tt.wantMessage) {
				t.Fatalf("preflight error = %#v", err)
			}
		})
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
