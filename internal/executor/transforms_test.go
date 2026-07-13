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

func TestTransformPreflightRejectsMalformedDeclarationTypes(t *testing.T) {
	stringType := typesys.Primitive("string")
	tests := []ir.Transform{
		ir.PipelineTransform{
			Kind:          "pipeline",
			TransformBase: ir.TransformBase{SymbolID: "transform:pipeline", Name: "pipeline", Input: typesys.Type{Kind: typesys.KindArray}, Output: stringType},
		},
		ir.MatchTransform{
			Kind:          "match",
			TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: typesys.Type{Kind: typesys.KindNullable}},
			Default:       json.RawMessage(`"fallback"`),
		},
	}
	for _, transform := range tests {
		runtime := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{transform}}, nil)
		var execution *ExecutionError
		if err := runtime.preflight(); !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" {
			t.Fatalf("preflight error = %#v", err)
		}
	}
}

func TestTransformPreflightRejectsInvalidKindDiscriminators(t *testing.T) {
	stringType := typesys.Primitive("string")
	validCall := ir.TransformCall{Target: ir.BuiltinTarget{Kind: "builtin", Name: "trim"}, Input: stringType, Output: stringType}
	tests := []ir.Transform{
		ir.PipelineTransform{Kind: "sequence", TransformBase: ir.TransformBase{SymbolID: "transform:pipeline", Name: "pipeline", Input: stringType, Output: stringType}, Calls: []ir.TransformCall{validCall}},
		ir.MatchTransform{Kind: "switch", TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: stringType}, Default: json.RawMessage(`"fallback"`)},
		ir.ExternalTransform{Kind: "host", TransformBase: ir.TransformBase{SymbolID: "transform:external", Name: "external", Input: stringType, Output: stringType}, Symbol: "external"},
	}
	for _, transform := range tests {
		external := map[string]ExternalTransform{"external": func(context.Context, any) (any, error) { return "value", nil }}
		var execution *ExecutionError
		if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{transform}}, external).preflight(); !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" {
			t.Fatalf("preflight error = %#v", err)
		}
	}
}

func TestTransformPreflightRejectsEmptyExternalSymbol(t *testing.T) {
	stringType := typesys.Primitive("string")
	valid := ir.ExternalTransform{
		Kind:          "external",
		TransformBase: ir.TransformBase{SymbolID: "transform:external", Name: "external", Input: stringType, Output: stringType},
		Symbol:        "external",
	}
	callback := func(context.Context, any) (any, error) { return "value", nil }
	if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{valid}}, map[string]ExternalTransform{"external": callback}).preflight(); err != nil {
		t.Fatalf("valid external preflight error = %v", err)
	}
	invalid := valid
	invalid.Symbol = ""
	var execution *ExecutionError
	if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{invalid}}, map[string]ExternalTransform{"": callback}).preflight(); !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "external" {
		t.Fatalf("empty external symbol preflight error = %#v", err)
	}
}

func TestTransformPreflightValidatesPipelineContinuity(t *testing.T) {
	stringType := typesys.Primitive("string")
	boolType := typesys.Primitive("bool")
	validCall := ir.TransformCall{Target: ir.BuiltinTarget{Kind: "builtin", Name: "trim"}, Input: stringType, Output: stringType}
	valid := ir.PipelineTransform{
		Kind:          "pipeline",
		TransformBase: ir.TransformBase{SymbolID: "transform:pipeline", Name: "pipeline", Input: stringType, Output: stringType},
		Calls:         []ir.TransformCall{validCall},
	}
	if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{valid}}, nil).preflight(); err != nil {
		t.Fatalf("valid pipeline preflight error = %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ir.PipelineTransform)
	}{
		{name: "empty", mutate: func(pipeline *ir.PipelineTransform) { pipeline.Calls = nil }},
		{name: "first input", mutate: func(pipeline *ir.PipelineTransform) { pipeline.Calls[0].Input = boolType }},
		{name: "adjacent input", mutate: func(pipeline *ir.PipelineTransform) {
			pipeline.Calls = append(pipeline.Calls, ir.TransformCall{Target: validCall.Target, Input: boolType, Output: stringType})
		}},
		{name: "final output", mutate: func(pipeline *ir.PipelineTransform) { pipeline.Calls[0].Output = boolType }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := valid
			pipeline.Calls = append([]ir.TransformCall(nil), valid.Calls...)
			tt.mutate(&pipeline)
			var execution *ExecutionError
			if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{pipeline}}, nil).preflight(); !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "pipeline" {
				t.Fatalf("preflight error = %#v", err)
			}
		})
	}
}

func TestTransformPreflightValidatesDeclaredCallTypes(t *testing.T) {
	stringType := typesys.Primitive("string")
	boolType := typesys.Primitive("bool")
	declaration := ir.MatchTransform{
		Kind:          "match",
		TransformBase: ir.TransformBase{SymbolID: "transform:declared", Name: "declared", Input: stringType, Output: boolType},
		Default:       json.RawMessage(`true`),
	}
	newCaller := func(input, output typesys.Type) ir.PipelineTransform {
		return ir.PipelineTransform{
			Kind:          "pipeline",
			TransformBase: ir.TransformBase{SymbolID: "transform:caller", Name: "caller", Input: input, Output: output},
			Calls: []ir.TransformCall{{
				Target: ir.DeclaredTarget{Kind: "declared", SymbolID: declaration.SymbolID}, Input: input, Output: output,
			}},
		}
	}
	if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{newCaller(stringType, boolType), declaration}}, nil).preflight(); err != nil {
		t.Fatalf("valid declared call preflight error = %v", err)
	}
	for _, tt := range []struct {
		name          string
		input, output typesys.Type
	}{
		{name: "input", input: boolType, output: boolType},
		{name: "output", input: stringType, output: stringType},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var execution *ExecutionError
			err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{newCaller(tt.input, tt.output), declaration}}, nil).preflight()
			if !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "caller" {
				t.Fatalf("declared call preflight error = %#v", err)
			}
		})
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
		{name: "invalid builtin kind", target: ir.BuiltinTarget{Kind: "native", Name: "trim"}, wantCode: "E_IR_INVALID"},
		{name: "missing declared", target: ir.DeclaredTarget{Kind: "declared", SymbolID: "transform:missing"}, wantCode: "E_TRANSFORM_MISSING"},
		{name: "invalid declared kind", target: ir.DeclaredTarget{Kind: "symbol", SymbolID: "transform:missing"}, wantCode: "E_IR_INVALID"},
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
			call := tt.call
			call.Input, call.Output = stringType, stringType
			field.Transforms[0] = call
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

	extractor := newExtractor()
	field := extractor.Output.Members[0].(ir.Field)
	field.Transforms[0] = ir.TransformCall{Target: ir.BuiltinTarget{Kind: "builtin", Name: "trim"}, Output: stringType}
	extractor.Output.Members[0] = field
	err := newTransformRuntime(context.Background(), extractor, nil).preflight()
	var typeExecution *ExecutionError
	if !errors.As(err, &typeExecution) || typeExecution.Code != "E_IR_INVALID" || typeExecution.Path != "output.value" {
		t.Fatalf("call type preflight error = %#v", err)
	}

	declared := ir.PipelineTransform{
		Kind:          "pipeline",
		TransformBase: ir.TransformBase{SymbolID: "transform:declared", Name: "declared", Input: stringType, Output: stringType},
		Calls:         []ir.TransformCall{validCall},
	}
	extractor = newExtractor()
	extractor.Transforms = []ir.Transform{declared}
	field = extractor.Output.Members[0].(ir.Field)
	field.Transforms[0] = ir.TransformCall{
		Target:              ir.DeclaredTarget{Kind: "declared", SymbolID: declared.SymbolID},
		PositionalArguments: []json.RawMessage{json.RawMessage(`"value"`)},
	}
	extractor.Output.Members[0] = field
	err = newTransformRuntime(context.Background(), extractor, nil).preflight()
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

func TestTransformPreflightValidatesMatchStructure(t *testing.T) {
	stringType := typesys.Primitive("string")
	valid := ir.MatchTransform{
		Kind:          "match",
		TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: stringType, Output: stringType},
		Cases:         []ir.MatchCase{{When: json.RawMessage(`"input"`), Then: json.RawMessage(`"output"`)}},
		Default:       json.RawMessage(`"fallback"`),
	}
	tests := []struct {
		name   string
		mutate func(*ir.MatchTransform)
	}{
		{name: "array input", mutate: func(match *ir.MatchTransform) { match.Input = typesys.Array(stringType) }},
		{name: "object output", mutate: func(match *ir.MatchTransform) { match.Output = typesys.Primitive("object") }},
		{name: "duplicate case", mutate: func(match *ir.MatchTransform) {
			match.Cases = append(match.Cases, ir.MatchCase{When: json.RawMessage(`"input"`), Then: json.RawMessage(`"other"`)})
		}},
		{name: "normalized numeric duplicate", mutate: func(match *ir.MatchTransform) {
			match.Input = typesys.Primitive("float")
			match.Cases = []ir.MatchCase{
				{When: json.RawMessage(`1`), Then: json.RawMessage(`"one"`)},
				{When: json.RawMessage(`1.0`), Then: json.RawMessage(`"other"`)},
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := valid
			match.Cases = append([]ir.MatchCase(nil), valid.Cases...)
			tt.mutate(&match)
			var execution *ExecutionError
			if err := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{match}}, nil).preflight(); !errors.As(err, &execution) || execution.Code != "E_IR_INVALID" || execution.Path != "match" {
				t.Fatalf("match structure preflight error = %#v", err)
			}
		})
	}
}

func TestTransformRuntimeNormalizesMatchCaseInputs(t *testing.T) {
	floatType := typesys.Primitive("float")
	stringType := typesys.Primitive("string")
	match := ir.MatchTransform{
		Kind:          "match",
		TransformBase: ir.TransformBase{SymbolID: "transform:match", Name: "match", Input: floatType, Output: stringType},
		Cases:         []ir.MatchCase{{When: json.RawMessage(`1.0`), Then: json.RawMessage(`"matched"`)}},
		Default:       json.RawMessage(`"fallback"`),
	}
	runtime := newTransformRuntime(context.Background(), &ir.Extractor{Transforms: []ir.Transform{match}}, nil)
	if err := runtime.preflight(); err != nil {
		t.Fatalf("match preflight error = %v", err)
	}
	result, err := runtime.applyDeclared(match.SymbolID, float64(1), "output.value")
	if err != nil || result != "matched" {
		t.Fatalf("match result = %#v, error = %v", result, err)
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
