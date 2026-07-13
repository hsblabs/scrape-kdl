package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

type transformRuntime struct {
	ctx       context.Context
	declared  map[string]ir.Transform
	external  map[string]ExternalTransform
	callStack map[string]bool
}

func newTransformRuntime(ctx context.Context, extractor *ir.Extractor, external map[string]ExternalTransform) *transformRuntime {
	declared := make(map[string]ir.Transform, len(extractor.Transforms))
	for _, transform := range extractor.Transforms {
		switch typed := transform.(type) {
		case ir.PipelineTransform:
			declared[typed.SymbolID] = transform
		case ir.MatchTransform:
			declared[typed.SymbolID] = transform
		case ir.ExternalTransform:
			declared[typed.SymbolID] = transform
		}
	}
	return &transformRuntime{ctx: ctx, declared: declared, external: external, callStack: map[string]bool{}}
}

func (runtime *transformRuntime) preflight() error {
	for _, transform := range runtime.declared {
		external, ok := transform.(ir.ExternalTransform)
		if !ok {
			continue
		}
		if _, exists := runtime.external[external.Symbol]; !exists {
			return &ExecutionError{Code: "E_EXTERNAL_TRANSFORM_MISSING", Message: fmt.Sprintf("external transform symbol %q is not registered", external.Symbol), Path: external.Name}
		}
	}
	return nil
}

func (runtime *transformRuntime) applyCalls(value any, calls []ir.TransformCall, path string) (any, error) {
	current := value
	for _, call := range calls {
		var err error
		switch target := call.Target.(type) {
		case ir.BuiltinTarget:
			current, err = applyBuiltinRuntime(target.Name, current, call)
		case ir.DeclaredTarget:
			current, err = runtime.applyDeclared(target.SymbolID, current, path)
		default:
			err = fmt.Errorf("unknown transform target %T", call.Target)
		}
		if err != nil {
			if execution, ok := err.(*ExecutionError); ok {
				if execution.Path == "" {
					execution.Path = path
				}
				return nil, execution
			}
			return nil, &ExecutionError{Code: "E_TRANSFORM", Message: err.Error(), Path: path, Cause: err}
		}
	}
	return current, nil
}

func (runtime *transformRuntime) applyDeclared(symbolID string, input any, path string) (any, error) {
	transform, ok := runtime.declared[symbolID]
	if !ok {
		return nil, &ExecutionError{Code: "E_TRANSFORM_MISSING", Message: fmt.Sprintf("declared transform %q is missing from IR", symbolID), Path: path}
	}
	if runtime.callStack[symbolID] {
		return nil, &ExecutionError{Code: "E_TRANSFORM_RECURSION", Message: fmt.Sprintf("recursive transform call %q", symbolID), Path: path}
	}
	runtime.callStack[symbolID] = true
	defer delete(runtime.callStack, symbolID)

	switch typed := transform.(type) {
	case ir.PipelineTransform:
		return runtime.applyCalls(input, typed.Calls, path)
	case ir.MatchTransform:
		decodeResult := func(raw json.RawMessage) (any, error) {
			value, err := decodeJSON(raw)
			if err != nil {
				return nil, err
			}
			normalized, ok := normalizeJSONResult(value, typed.Output)
			if !ok {
				return nil, fmt.Errorf("match result of type %T is not assignable to %s", value, typed.Output.String())
			}
			return normalized, nil
		}
		for _, item := range typed.Cases {
			when, err := decodeJSON(item.When)
			if err != nil {
				return nil, err
			}
			if equalScalar(input, when) {
				return decodeResult(item.Then)
			}
		}
		return decodeResult(typed.Default)
	case ir.ExternalTransform:
		function, ok := runtime.external[typed.Symbol]
		if !ok {
			return nil, &ExecutionError{Code: "E_EXTERNAL_TRANSFORM_MISSING", Message: fmt.Sprintf("external transform symbol %q is not registered", typed.Symbol), Path: path}
		}
		value, err := function(runtime.ctx, input)
		if err != nil {
			return nil, &ExecutionError{Code: "E_EXTERNAL_TRANSFORM", Message: err.Error(), Path: path, Cause: err}
		}
		return value, nil
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown declared transform %T", transform), Path: path}
	}
}

func equalScalar(left, right any) bool {
	if leftNumber, ok := numberString(left); ok {
		if rightNumber, ok := numberString(right); ok {
			return leftNumber == rightNumber
		}
	}
	return reflect.DeepEqual(left, right)
}

func numberString(value any) (string, bool) {
	switch typed := value.(type) {
	case json.Number:
		return typed.String(), true
	case int:
		return fmt.Sprintf("%d", typed), true
	case int8:
		return fmt.Sprintf("%d", typed), true
	case int16:
		return fmt.Sprintf("%d", typed), true
	case int32:
		return fmt.Sprintf("%d", typed), true
	case int64:
		return fmt.Sprintf("%d", typed), true
	case uint:
		return fmt.Sprintf("%d", typed), true
	case uint8:
		return fmt.Sprintf("%d", typed), true
	case uint16:
		return fmt.Sprintf("%d", typed), true
	case uint32:
		return fmt.Sprintf("%d", typed), true
	case uint64:
		return fmt.Sprintf("%d", typed), true
	case float32:
		return fmt.Sprintf("%g", typed), true
	case float64:
		return fmt.Sprintf("%g", typed), true
	default:
		return "", false
	}
}
