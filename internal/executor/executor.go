package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/dom"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

type engine struct {
	ctx        context.Context
	extractor  *ir.Extractor
	options    Options
	transforms *transformRuntime
	selectors  map[string]dom.Selector
	warnings   []Warning
	partial    bool
}

type missingValue struct {
	message string
}

func (m missingValue) Error() string { return m.message }

func Execute(ctx context.Context, extractor *ir.Extractor, inputs map[string]any, options Options) (*Result, error) {
	options = options.withDefaults()
	if extractor != nil && extractor.Source.Fetch.Mode == "browser" {
		return ExecuteBrowser(ctx, extractor, inputs, options)
	}
	engine, err := newEngine(ctx, extractor, options)
	if err != nil {
		return nil, err
	}
	resolvedInputs, err := resolveInputs(extractor.Inputs, inputs)
	if err != nil {
		return nil, err
	}
	if extractor.Source.SessionPolicy == "required" && options.Session == nil {
		return nil, &ExecutionError{Code: "E_SESSION_REQUIRED", Message: "source requires a runtime session"}
	}
	targetURL, err := expandTemplate(extractor.Source.Fetch.URLTemplate, resolvedInputs)
	if err != nil {
		return nil, err
	}
	if err := enforceURLPolicy(ctx, targetURL, options.URLPolicy); err != nil {
		return nil, err
	}
	fetchOptions := options
	if extractor.Source.SessionPolicy == "none" {
		fetchOptions.Session = nil
	}
	document, err := fetchDocument(ctx, targetURL, fetchOptions)
	if err != nil {
		return nil, err
	}
	return engine.executeDocument(document)
}

// ExecuteHTML runs output extraction against an already-decoded HTML fixture.
// It is intended for tests, inspectors, and offline validation; it bypasses
// source fetch, URL input expansion, and session policy.
func ExecuteHTML(ctx context.Context, extractor *ir.Extractor, html string, options Options) (*Result, error) {
	options = options.withDefaults()
	engine, err := newEngine(ctx, extractor, options)
	if err != nil {
		return nil, err
	}
	document, err := dom.ParseHTML(strings.NewReader(html))
	if err != nil {
		return nil, &ExecutionError{Code: "E_HTML_PARSE", Message: err.Error(), Cause: err}
	}
	return engine.executeDocument(document)
}

func newEngine(ctx context.Context, extractor *ir.Extractor, options Options) (*engine, error) {
	if extractor == nil {
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "extractor IR is nil"}
	}
	if extractor.Source.Fetch.Mode != "http" {
		return nil, &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: fmt.Sprintf("HTTP runtime cannot execute fetch mode %q", extractor.Source.Fetch.Mode)}
	}
	transforms := newTransformRuntime(ctx, extractor, options.ExternalTransforms)
	if err := transforms.preflight(); err != nil {
		return nil, err
	}
	if err := preflightOutputIdentities(extractor.Output); err != nil {
		return nil, err
	}
	result := &engine{
		ctx: ctx, extractor: extractor, options: options, transforms: transforms,
		selectors: map[string]dom.Selector{},
	}
	if err := result.preflightOutput(extractor.Output); err != nil {
		return nil, err
	}
	return result, nil
}

func preflightOutputIdentities(root ir.OutputObject) error {
	seenIDs := map[string]struct{}{}
	var walk func(ir.OutputObject, string) error
	walk = func(object ir.OutputObject, path string) error {
		seenNames := map[string]struct{}{}
		for _, member := range object.Members {
			var name, id string
			var row *ir.OutputObject
			switch typed := member.(type) {
			case ir.Field:
				name, id = typed.Name, typed.ID
			case ir.Collection:
				name, id, row = typed.Name, typed.ID, &typed.Row
			default:
				continue
			}
			if _, exists := seenNames[name]; exists {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("duplicate output member name %q", name), Path: path}
			}
			seenNames[name] = struct{}{}
			if _, exists := seenIDs[id]; exists {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("duplicate output member ID %q", id), Path: path}
			}
			seenIDs[id] = struct{}{}
			if row != nil {
				if err := walk(*row, id+"[]"); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(root, "output")
}

func (e *engine) preflightOutput(object ir.OutputObject) error {
	for _, member := range object.Members {
		switch typed := member.(type) {
		case ir.Field:
			if typed.Selection != nil {
				if _, err := e.selector(typed.Selection.Selector); err != nil {
					return &ExecutionError{Code: "E_SELECTOR_INVALID", Message: err.Error(), Path: typed.ID, Cause: err}
				}
			}
			switch typed.ValueSource.(type) {
			case ir.TextValueSource, ir.HTMLValueSource, ir.AttributeValueSource:
			case ir.JavaScriptValueSource:
				return &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: "HTTP runtime cannot execute JavaScript value sources", Path: typed.ID}
			default:
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown value source %T", typed.ValueSource), Path: typed.ID}
			}
		case ir.Collection:
			if _, err := e.selector(typed.Selector); err != nil {
				return &ExecutionError{Code: "E_SELECTOR_INVALID", Message: err.Error(), Path: typed.ID, Cause: err}
			}
			if err := e.preflightOutput(typed.Row); err != nil {
				return err
			}
		default:
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown output member %T", member)}
		}
	}
	return nil
}

func (e *engine) selector(source string) (dom.Selector, error) {
	if cached, ok := e.selectors[source]; ok {
		return cached, nil
	}
	parsed, err := dom.ParseSelector(source)
	if err != nil {
		return dom.Selector{}, err
	}
	e.selectors[source] = parsed
	return parsed, nil
}

func (e *engine) executeDocument(document *dom.Node) (*Result, error) {
	value, err := e.executeObject(document, e.extractor.Output, "output")
	if err != nil {
		return nil, err
	}
	return finalizeResult(value, e.warnings, e.partial), nil
}

func (e *engine) executeObject(scope *dom.Node, object ir.OutputObject, path string) (map[string]any, error) {
	result := make(map[string]any, len(object.Members))
	for _, member := range object.Members {
		switch typed := member.(type) {
		case ir.Field:
			value, err := e.executeField(scope, typed, path+"."+typed.Name)
			if err != nil {
				return nil, err
			}
			result[typed.Name] = value
		case ir.Collection:
			value, err := e.executeCollection(scope, typed, path+"."+typed.Name)
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

func (e *engine) executeField(scope *dom.Node, field ir.Field, path string) (any, error) {
	value, err := e.readFieldValue(scope, field, path)
	if err != nil {
		if _, missing := err.(missingValue); missing {
			return e.handleMissing(field, path, err.Error())
		}
		return e.recoverField(field, path, err)
	}
	value, err = e.transforms.applyCalls(value, field.Transforms, path)
	if err != nil {
		return e.recoverField(field, path, err)
	}
	if !matchesRuntimeType(value, field.SuccessfulType) {
		return e.recoverField(field, path, &ExecutionError{Code: "E_OUTPUT_TYPE", Message: fmt.Sprintf("value of type %T is not assignable to %s", value, field.SuccessfulType.String()), Path: path})
	}
	return value, nil
}

func (e *engine) readFieldValue(scope *dom.Node, field ir.Field, path string) (any, error) {
	var selected *dom.Node
	if field.Selection != nil {
		selector, _ := e.selector(field.Selection.Selector)
		matches := dom.QueryAll(scope, selector)
		if len(matches) == 0 {
			return nil, missingValue{message: fmt.Sprintf("selector %q matched no elements", field.Selection.Selector)}
		}
		if field.Selection.Match == "one" && len(matches) != 1 {
			return nil, &ExecutionError{Code: "E_SELECTOR_CARDINALITY", Message: fmt.Sprintf("selector %q matched %d elements; expected exactly one", field.Selection.Selector, len(matches)), Path: path}
		}
		selected = matches[0]
	}
	switch source := field.ValueSource.(type) {
	case ir.TextValueSource:
		if selected == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "text value source has no selected element", Path: path}
		}
		return selected.TextContent(), nil
	case ir.HTMLValueSource:
		if selected == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "HTML value source has no selected element", Path: path}
		}
		return selected.InnerHTML(), nil
	case ir.AttributeValueSource:
		if selected == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "attribute value source has no selected element", Path: path}
		}
		value, ok := selected.Attr(source.Name)
		if !ok {
			return nil, missingValue{message: fmt.Sprintf("attribute %q is missing", source.Name)}
		}
		return value, nil
	case ir.JavaScriptValueSource:
		return nil, &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: "HTTP runtime cannot evaluate JavaScript", Path: path}
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown value source %T", field.ValueSource), Path: path}
	}
}

func (e *engine) handleMissing(field ir.Field, path, message string) (any, error) {
	if field.Required {
		return nil, &ExecutionError{Code: "E_REQUIRED_VALUE_MISSING", Message: message, Path: path}
	}
	if field.Default != nil {
		return decodeFieldDefault(field, path)
	}
	return nil, nil
}

func decodeFieldDefault(field ir.Field, path string) (any, error) {
	if field.Default == nil {
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "on-error default requires a field default", Path: path}
	}
	value, err := decodeJSON(*field.Default)
	if err != nil {
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: err.Error(), Path: path, Cause: err}
	}
	normalized, ok := normalizeJSONResult(value, field.SuccessfulType)
	if !ok {
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("field default of type %T is not assignable to %s", value, field.SuccessfulType.String()), Path: path}
	}
	return normalized, nil
}

func (e *engine) recoverField(field ir.Field, path string, cause error) (any, error) {
	policy := field.OnError
	if policy == "" {
		if field.Required {
			policy = "fail"
		} else {
			policy = "null"
		}
	}
	switch policy {
	case "fail":
		if execution, ok := cause.(*ExecutionError); ok {
			return nil, execution
		}
		return nil, &ExecutionError{Code: "E_FIELD_EXECUTION", Message: cause.Error(), Path: path, Cause: cause}
	case "null":
		e.partial = true
		return nil, nil
	case "warn":
		e.partial = true
		e.warnings = append(e.warnings, Warning{Code: "W_ERROR_RECOVERED", Message: cause.Error(), Path: path})
		return nil, nil
	case "default":
		value, err := decodeFieldDefault(field, path)
		if err != nil {
			return nil, err
		}
		e.partial = true
		return value, nil
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown on-error policy %q", policy), Path: path}
	}
}

func (e *engine) executeCollection(scope *dom.Node, collection ir.Collection, path string) ([]any, error) {
	selector, _ := e.selector(collection.Selector)
	rows := dom.QueryAll(scope, selector)
	result := make([]any, 0, len(rows))
	for index, row := range rows {
		value, err := e.executeObject(row, collection.Row, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			if collection.OnRowError != "skip" {
				return nil, err
			}
			e.partial = true
			rowIndex := index
			e.warnings = append(e.warnings, Warning{Code: "W_ROW_SKIPPED", Message: err.Error(), Path: path, Row: &rowIndex})
			continue
		}
		result = append(result, value)
	}
	minimum := collection.MinItems
	if collection.Required && minimum < 1 {
		minimum = 1
	}
	if len(result) < minimum {
		return nil, &ExecutionError{Code: "E_COLLECTION_CARDINALITY", Message: fmt.Sprintf("collection has %d rows after recovery; minimum is %d", len(result), minimum), Path: path}
	}
	if collection.MaxItems != nil && len(result) > *collection.MaxItems {
		return nil, &ExecutionError{Code: "E_COLLECTION_CARDINALITY", Message: fmt.Sprintf("collection has %d rows; maximum is %d", len(result), *collection.MaxItems), Path: path}
	}
	return result, nil
}

func matchesRuntimeType(value any, target typesys.Type) bool {
	if target.Kind == typesys.KindNullable {
		if value == nil {
			return true
		}
		return target.Inner != nil && matchesRuntimeType(value, *target.Inner)
	}
	if value == nil {
		return target.Kind == typesys.KindPrimitive && target.Name == "unknown"
	}
	if target.Kind == typesys.KindArray {
		if target.Element == nil {
			return false
		}
		switch values := value.(type) {
		case []string:
			for _, item := range values {
				if !matchesRuntimeType(item, *target.Element) {
					return false
				}
			}
			return true
		case []any:
			for _, item := range values {
				if !matchesRuntimeType(item, *target.Element) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	if target.Kind != typesys.KindPrimitive {
		return false
	}
	switch target.Name {
	case "unknown":
		return isJSONCompatible(value)
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "bool":
		_, ok := value.(bool)
		return ok
	case "int", "i64":
		_, ok := value.(int64)
		return ok
	case "i8":
		_, ok := value.(int8)
		return ok
	case "i16":
		_, ok := value.(int16)
		return ok
	case "i32":
		_, ok := value.(int32)
		return ok
	case "u8":
		_, ok := value.(uint8)
		return ok
	case "u16":
		_, ok := value.(uint16)
		return ok
	case "u32":
		_, ok := value.(uint32)
		return ok
	case "u64":
		_, ok := value.(uint64)
		return ok
	case "float", "f64":
		value, ok := value.(float64)
		return ok && !math.IsNaN(value) && !math.IsInf(value, 0)
	case "f32":
		value, ok := value.(float32)
		return ok && !float32IsInvalid(value)
	default:
		return false
	}
}

func float32IsInvalid(value float32) bool {
	converted := float64(value)
	return math.IsNaN(converted) || math.IsInf(converted, 0)
}

func isJSONCompatible(value any) bool {
	switch typed := value.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case json.Number:
		if _, err := json.Marshal(typed); err != nil {
			return false
		}
		number, err := typed.Float64()
		return err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
	case float32:
		return !float32IsInvalid(typed)
	case float64:
		return !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case []string:
		return true
	case []any:
		for _, item := range typed {
			if !isJSONCompatible(item) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, item := range typed {
			if !isJSONCompatible(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
