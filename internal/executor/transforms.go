package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

type transformRuntime struct {
	extractor     *ir.Extractor
	ctx           context.Context
	declared      map[string]ir.Transform
	externalDecls []ir.ExternalTransform
	external      map[string]ExternalTransform
	callStack     map[string]bool
	duplicateID   string
	hasDuplicate  bool
}

func newTransformRuntime(ctx context.Context, extractor *ir.Extractor, external map[string]ExternalTransform) *transformRuntime {
	declared := make(map[string]ir.Transform, len(extractor.Transforms))
	externalDecls := make([]ir.ExternalTransform, 0)
	var duplicateID string
	runtimeDuplicateSet := false
	addDeclared := func(symbolID string, transform ir.Transform) {
		if _, exists := declared[symbolID]; exists && !runtimeDuplicateSet {
			duplicateID, runtimeDuplicateSet = symbolID, true
		}
		declared[symbolID] = transform
	}
	for _, transform := range extractor.Transforms {
		switch typed := transform.(type) {
		case ir.PipelineTransform:
			addDeclared(typed.SymbolID, transform)
		case ir.MatchTransform:
			addDeclared(typed.SymbolID, transform)
		case ir.ExternalTransform:
			addDeclared(typed.SymbolID, transform)
			externalDecls = append(externalDecls, typed)
		}
	}
	return &transformRuntime{extractor: extractor, ctx: ctx, declared: declared, externalDecls: externalDecls, external: external, callStack: map[string]bool{}, duplicateID: duplicateID, hasDuplicate: runtimeDuplicateSet}
}

func (runtime *transformRuntime) preflight() error {
	if runtime.hasDuplicate {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("duplicate declared transform symbol %q", runtime.duplicateID), Path: "transforms"}
	}
	for _, external := range runtime.externalDecls {
		if external.Kind != "external" {
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid external transform kind %q", external.Kind), Path: external.Name}
		}
		if external.Symbol == "" {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "external transform symbol must be non-empty", Path: external.Name}
		}
		if _, exists := runtime.external[external.Symbol]; !exists {
			return &ExecutionError{Code: "E_EXTERNAL_TRANSFORM_MISSING", Message: fmt.Sprintf("external transform symbol %q is not registered", external.Symbol), Path: external.Name}
		}
	}
	for _, transform := range runtime.extractor.Transforms {
		switch typed := transform.(type) {
		case ir.PipelineTransform:
			if typed.Kind != "pipeline" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid pipeline transform kind %q", typed.Kind), Path: typed.Name}
			}
			if err := preflightTransformTypes(typed.Input, typed.Output, typed.Name); err != nil {
				return err
			}
			if len(typed.Calls) == 0 {
				return &ExecutionError{Code: "E_IR_INVALID", Message: "pipeline transform requires at least one call", Path: typed.Name}
			}
			if err := runtime.preflightCalls(typed.Calls, typed.Name); err != nil {
				return err
			}
			if err := preflightCallContinuity(typed.Calls, typed.Input, typed.Output, typed.Name); err != nil {
				return err
			}
		case ir.MatchTransform:
			if typed.Kind != "match" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid match transform kind %q", typed.Kind), Path: typed.Name}
			}
			if err := preflightTransformTypes(typed.Input, typed.Output, typed.Name); err != nil {
				return err
			}
			if err := preflightMatchTransform(typed); err != nil {
				return err
			}
		case ir.ExternalTransform:
			if err := preflightTransformTypes(typed.Input, typed.Output, typed.Name); err != nil {
				return err
			}
		}
	}
	return runtime.preflightOutputCalls(runtime.extractor.Output)
}

func preflightTransformTypes(input, output typesys.Type, path string) error {
	if !validRuntimeType(input) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: "transform has an invalid input type", Path: path}
	}
	if !validRuntimeType(output) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: "transform has an invalid output type", Path: path}
	}
	return nil
}

func preflightMatchTransform(match ir.MatchTransform) error {
	if !typesys.IsScalar(match.Input) || !typesys.IsScalar(match.Output) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: "match transform input and output must be scalar or nullable scalar", Path: match.Name}
	}
	validate := func(raw json.RawMessage, expected typesys.Type, description string) (any, error) {
		value, err := decodeJSON(raw)
		if err != nil {
			cause := fmt.Errorf("invalid %s: %w", description, err)
			return nil, &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: match.Name, Cause: cause}
		}
		normalized, ok := normalizeJSONResult(value, expected)
		if !ok {
			cause := fmt.Errorf("%s of type %T is not assignable to %s", description, value, expected.String())
			return nil, &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: match.Name, Cause: cause}
		}
		return normalized, nil
	}
	seen := make([]any, 0, len(match.Cases))
	for index, item := range match.Cases {
		when, err := validate(item.When, match.Input, fmt.Sprintf("match case %d input", index))
		if err != nil {
			return err
		}
		for _, previous := range seen {
			if equalScalar(previous, when) {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("duplicate match case input at index %d", index), Path: match.Name}
			}
		}
		seen = append(seen, when)
		if _, err := validate(item.Then, match.Output, fmt.Sprintf("match case %d result", index)); err != nil {
			return err
		}
	}
	_, err := validate(match.Default, match.Output, "match default")
	return err
}

func (runtime *transformRuntime) preflightOutputCalls(object ir.OutputObject) error {
	for _, member := range object.Members {
		switch typed := member.(type) {
		case ir.Field:
			if err := runtime.preflightCalls(typed.Transforms, typed.ID); err != nil {
				return err
			}
			if raw, ok := fieldRawType(typed); ok {
				if err := preflightCallContinuity(typed.Transforms, raw, typed.SuccessfulType, typed.ID); err != nil {
					return err
				}
			}
		case ir.Collection:
			if err := runtime.preflightOutputCalls(typed.Row); err != nil {
				return err
			}
		}
	}
	return nil
}

func fieldRawType(field ir.Field) (typesys.Type, bool) {
	switch source := field.ValueSource.(type) {
	case ir.TextValueSource:
		return source.RawType, true
	case ir.HTMLValueSource:
		return source.RawType, true
	case ir.AttributeValueSource:
		return source.RawType, true
	case ir.JavaScriptValueSource:
		return source.Returns, true
	default:
		return typesys.Type{}, false
	}
}

func preflightCallContinuity(calls []ir.TransformCall, initial, expected typesys.Type, path string) error {
	if !validRuntimeType(initial) || !validRuntimeType(expected) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: "transform chain has invalid boundary type metadata", Path: path}
	}
	current := initial
	for index, call := range calls {
		if !typesys.Equal(call.Input, current) {
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("transform call %d input %s does not match previous output %s", index, call.Input.String(), current.String()), Path: path}
		}
		current = call.Output
	}
	if !typesys.IsAssignable(current, expected) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("transform chain output %s is not assignable to %s", current.String(), expected.String()), Path: path}
	}
	return nil
}

func (runtime *transformRuntime) preflightCalls(calls []ir.TransformCall, path string) error {
	for _, call := range calls {
		var declaredInput, declaredOutput typesys.Type
		hasDeclaredTarget := false
		switch target := call.Target.(type) {
		case ir.BuiltinTarget:
			if target.Kind != "builtin" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid built-in target kind %q", target.Kind), Path: path}
			}
			if !isKnownBuiltinRuntime(target.Name) {
				cause := fmt.Errorf("unknown built-in %q", target.Name)
				return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
			}
		case ir.DeclaredTarget:
			if target.Kind != "declared" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid declared target kind %q", target.Kind), Path: path}
			}
			declared, ok := runtime.declared[target.SymbolID]
			if !ok {
				return &ExecutionError{Code: "E_TRANSFORM_MISSING", Message: fmt.Sprintf("declared transform %q is missing from IR", target.SymbolID), Path: path}
			}
			declaredInput, declaredOutput, _, hasDeclaredTarget = transformSignature(declared)
			if len(call.PositionalArguments) > 0 || len(call.NamedArguments) > 0 {
				cause := fmt.Errorf("declared transform %q accepts no arguments", target.SymbolID)
				return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
			}
		default:
			cause := fmt.Errorf("unknown transform target %T", call.Target)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
		}
		if err := preflightCallArguments(call, path); err != nil {
			return err
		}
		if target, ok := call.Target.(ir.BuiltinTarget); ok {
			if err := preflightBuiltinCallSignature(target.Name, call, path); err != nil {
				return err
			}
		}
		if !validRuntimeType(call.Input) {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "transform call has an invalid input type", Path: path}
		}
		if !validRuntimeType(call.Output) {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "transform call has an invalid output type", Path: path}
		}
		if hasDeclaredTarget {
			if !typesys.IsAssignable(call.Input, declaredInput) {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("transform call input %s is not assignable to declared input %s", call.Input.String(), declaredInput.String()), Path: path}
			}
			if !typesys.Equal(call.Output, declaredOutput) {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("transform call output %s does not match declared output %s", call.Output.String(), declaredOutput.String()), Path: path}
			}
		}
	}
	return nil
}

func transformSignature(transform ir.Transform) (typesys.Type, typesys.Type, string, bool) {
	switch typed := transform.(type) {
	case ir.PipelineTransform:
		return typed.Input, typed.Output, typed.Name, true
	case ir.MatchTransform:
		return typed.Input, typed.Output, typed.Name, true
	case ir.ExternalTransform:
		return typed.Input, typed.Output, typed.Name, true
	default:
		return typesys.Type{}, typesys.Type{}, "", false
	}
}

func preflightBuiltinCallSignature(name string, call ir.TransformCall, path string) error {
	signature := builtinRuntimeSignatures[name]
	if len(call.PositionalArguments) < signature.minPositional || (signature.maxPositional >= 0 && len(call.PositionalArguments) > signature.maxPositional) {
		var cause error
		if signature.maxPositional < 0 {
			cause = fmt.Errorf("built-in %q requires at least %d positional arguments, got %d", name, signature.minPositional, len(call.PositionalArguments))
		} else {
			cause = fmt.Errorf("built-in %q accepts %d..%d positional arguments, got %d", name, signature.minPositional, signature.maxPositional, len(call.PositionalArguments))
		}
		return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
	}
	present := make(map[string]struct{}, len(call.NamedArguments))
	for _, argument := range call.NamedArguments {
		present[argument.Name] = struct{}{}
		if !argumentNameListed(signature.allowed, argument.Name) {
			cause := fmt.Errorf("argument %q is not allowed on built-in %q", argument.Name, name)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
		}
	}
	for _, required := range strings.Fields(signature.required) {
		if _, ok := present[required]; !ok {
			cause := fmt.Errorf("built-in %q requires argument %q", name, required)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
		}
	}
	return nil
}

func argumentNameListed(names, name string) bool {
	for _, candidate := range strings.Fields(names) {
		if candidate == name {
			return true
		}
	}
	return false
}

func preflightCallArguments(call ir.TransformCall, path string) error {
	for index, raw := range call.PositionalArguments {
		if _, err := decodeJSON(raw); err != nil {
			cause := fmt.Errorf("invalid positional transform argument %d: %w", index, err)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
		}
	}
	seen := make(map[string]struct{}, len(call.NamedArguments))
	for _, argument := range call.NamedArguments {
		if _, exists := seen[argument.Name]; exists {
			cause := fmt.Errorf("duplicate transform argument %q", argument.Name)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
		}
		seen[argument.Name] = struct{}{}
		if _, err := decodeJSON(argument.Value); err != nil {
			cause := fmt.Errorf("invalid transform argument %q: %w", argument.Name, err)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
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
			rawWhen, err := decodeJSON(item.When)
			if err != nil {
				return nil, err
			}
			when, ok := normalizeJSONResult(rawWhen, typed.Input)
			if !ok {
				return nil, fmt.Errorf("match input of type %T is not assignable to %s", rawWhen, typed.Input.String())
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
