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
		if _, exists := runtime.external[external.Symbol]; !exists {
			return &ExecutionError{Code: "E_EXTERNAL_TRANSFORM_MISSING", Message: fmt.Sprintf("external transform symbol %q is not registered", external.Symbol), Path: external.Name}
		}
	}
	for _, transform := range runtime.extractor.Transforms {
		switch typed := transform.(type) {
		case ir.PipelineTransform:
			if err := runtime.preflightCalls(typed.Calls, typed.Name); err != nil {
				return err
			}
		case ir.MatchTransform:
			if err := preflightMatchTransform(typed); err != nil {
				return err
			}
		}
	}
	return runtime.preflightOutputCalls(runtime.extractor.Output)
}

func preflightMatchTransform(match ir.MatchTransform) error {
	validate := func(raw json.RawMessage, expected typesys.Type, description string) error {
		value, err := decodeJSON(raw)
		if err != nil {
			cause := fmt.Errorf("invalid %s: %w", description, err)
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: match.Name, Cause: cause}
		}
		if _, ok := normalizeJSONResult(value, expected); !ok {
			cause := fmt.Errorf("%s of type %T is not assignable to %s", description, value, expected.String())
			return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: match.Name, Cause: cause}
		}
		return nil
	}
	for index, item := range match.Cases {
		if err := validate(item.When, match.Input, fmt.Sprintf("match case %d input", index)); err != nil {
			return err
		}
		if err := validate(item.Then, match.Output, fmt.Sprintf("match case %d result", index)); err != nil {
			return err
		}
	}
	return validate(match.Default, match.Output, "match default")
}

func (runtime *transformRuntime) preflightOutputCalls(object ir.OutputObject) error {
	for _, member := range object.Members {
		switch typed := member.(type) {
		case ir.Field:
			if err := runtime.preflightCalls(typed.Transforms, typed.ID); err != nil {
				return err
			}
		case ir.Collection:
			if err := runtime.preflightOutputCalls(typed.Row); err != nil {
				return err
			}
		}
	}
	return nil
}

func (runtime *transformRuntime) preflightCalls(calls []ir.TransformCall, path string) error {
	for _, call := range calls {
		switch target := call.Target.(type) {
		case ir.BuiltinTarget:
			if !isKnownBuiltinRuntime(target.Name) {
				cause := fmt.Errorf("unknown built-in %q", target.Name)
				return &ExecutionError{Code: "E_TRANSFORM", Message: cause.Error(), Path: path, Cause: cause}
			}
		case ir.DeclaredTarget:
			if _, ok := runtime.declared[target.SymbolID]; !ok {
				return &ExecutionError{Code: "E_TRANSFORM_MISSING", Message: fmt.Sprintf("declared transform %q is missing from IR", target.SymbolID), Path: path}
			}
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
	}
	return nil
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
