package executor

import (
	"context"
	"fmt"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

type outputReader[Scope any] interface {
	readOutputField(Scope, ir.Field, string) (any, error)
	queryOutputRows(Scope, ir.Collection, string) ([]Scope, error)
}

type outputWalker[Scope any] struct {
	ctx        context.Context
	reader     outputReader[Scope]
	transforms *transformRuntime
	warnings   *[]Warning
	partial    *bool
}

func newOutputWalker[Scope any](ctx context.Context, reader outputReader[Scope], transforms *transformRuntime, warnings *[]Warning, partial *bool) *outputWalker[Scope] {
	return &outputWalker[Scope]{ctx: ctx, reader: reader, transforms: transforms, warnings: warnings, partial: partial}
}

func (walker *outputWalker[Scope]) executeObject(scope Scope, object ir.OutputObject, path string) (map[string]any, error) {
	result := make(map[string]any, len(object.Members))
	for _, member := range object.Members {
		if err := executionContextError(walker.ctx, path); err != nil {
			return nil, err
		}
		switch typed := member.(type) {
		case ir.Field:
			value, err := walker.executeField(scope, typed, path+"."+typed.Name)
			if err != nil {
				return nil, err
			}
			result[typed.Name] = value
		case ir.Collection:
			value, err := walker.executeCollection(scope, typed, path+"."+typed.Name)
			if err != nil {
				return nil, err
			}
			result[typed.Name] = value
		default:
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown output member %T", member), Path: path}
		}
	}
	return result, nil
}

func (walker *outputWalker[Scope]) executeField(scope Scope, field ir.Field, path string) (any, error) {
	value, err := walker.reader.readOutputField(scope, field, path)
	if err != nil {
		if _, missing := err.(missingValue); missing {
			return handleMissingOutput(field, path, err.Error())
		}
		return recoverOutputField(walker.partial, walker.warnings, field, path, err)
	}
	value, err = walker.transforms.applyCalls(value, field.Transforms, path)
	if err != nil {
		return recoverOutputField(walker.partial, walker.warnings, field, path, err)
	}
	if !matchesRuntimeType(value, field.SuccessfulType) {
		return recoverOutputField(walker.partial, walker.warnings, field, path, &ExecutionError{Code: "E_OUTPUT_TYPE", Message: fmt.Sprintf("value of type %T is not assignable to %s", value, field.SuccessfulType.String()), Path: path})
	}
	return value, nil
}

func (walker *outputWalker[Scope]) executeCollection(scope Scope, collection ir.Collection, path string) ([]any, error) {
	rows, err := walker.reader.queryOutputRows(scope, collection, path)
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(rows))
	for index, row := range rows {
		rowPath := fmt.Sprintf("%s[%d]", path, index)
		if err := executionContextError(walker.ctx, rowPath); err != nil {
			return nil, err
		}
		value, err := walker.executeObject(row, collection.Row, rowPath)
		if err != nil {
			if isExecutionCanceled(err) || collection.OnRowError != "skip" {
				return nil, err
			}
			*walker.partial = true
			rowIndex := index
			*walker.warnings = append(*walker.warnings, Warning{Code: "W_ROW_SKIPPED", Message: err.Error(), Path: path, Row: &rowIndex})
			continue
		}
		result = append(result, value)
		if err := validateCollectionMaximum(collection.MaxItems, len(result), path); err != nil {
			return nil, err
		}
	}
	if err := validateCollectionMinimum(collection.Required, collection.MinItems, len(result), path); err != nil {
		return nil, err
	}
	return result, nil
}
