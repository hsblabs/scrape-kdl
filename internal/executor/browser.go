package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

// BrowserElement is an opaque element handle owned by a BrowserAdapter.
// Implementations typically wrap a Playwright Locator/ElementHandle, rod.Element,
// or chromedp node reference.
type BrowserElement any

type BrowserNavigateOptions struct {
	Timeout   time.Duration
	Session   *Session
	UserAgent string
}

type BrowserEvaluateOptions struct {
	Timeout time.Duration
	Scope   BrowserElement // nil means document scope
}

// BrowserAdapter is the portable browser capability required by browser-mode
// extractors. The runtime intentionally does not import a browser library.
// BrowserAdapterLease is an optional adapter capability used to serialize a
// complete extraction. Implementations that wrap a single mutable page should
// implement it so concurrent Program.Extract calls cannot interleave.
type BrowserAdapterLease interface {
	Acquire(context.Context) (release func(), err error)
}

type BrowserAdapter interface {
	Navigate(context.Context, string, BrowserNavigateOptions) error
	WaitFor(context.Context, string, string, time.Duration) error
	Click(context.Context, string, time.Duration) error
	Fill(context.Context, string, string, time.Duration) error
	Press(context.Context, string, string, time.Duration) error
	Scroll(context.Context, float64, float64) error
	WaitForNetworkIdle(context.Context, time.Duration, time.Duration) error
	Evaluate(context.Context, string, BrowserEvaluateOptions) (any, error)
	QueryAll(context.Context, BrowserElement, string) ([]BrowserElement, error)
	Text(context.Context, BrowserElement) (string, error)
	HTML(context.Context, BrowserElement) (string, error)
	Attribute(context.Context, BrowserElement, string) (string, bool, error)
}

type browserEngine struct {
	ctx        context.Context
	extractor  *ir.Extractor
	options    Options
	adapter    BrowserAdapter
	transforms *transformRuntime
	warnings   []Warning
	partial    bool
}

func ExecuteBrowser(ctx context.Context, extractor *ir.Extractor, inputs map[string]any, options Options) (*Result, error) {
	options = options.withDefaults()
	if extractor == nil {
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "extractor IR is nil"}
	}
	if extractor.Source.Fetch.Mode != "browser" {
		return nil, &ExecutionError{Code: "E_BROWSER_MODE_REQUIRED", Message: fmt.Sprintf("browser runtime cannot execute fetch mode %q", extractor.Source.Fetch.Mode)}
	}
	if options.Browser == nil {
		return nil, &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: "browser-mode extractor requires Options.Browser"}
	}
	if !options.AllowJavaScript && containsJavaScript(extractor) {
		return nil, &ExecutionError{Code: "E_JAVASCRIPT_DISABLED", Message: "extractor contains JavaScript; set AllowJavaScript=true for trusted specs"}
	}
	resolved, err := resolveInputs(extractor.Inputs, inputs)
	if err != nil {
		return nil, err
	}
	if extractor.Source.SessionPolicy == "required" && options.Session == nil {
		return nil, &ExecutionError{Code: "E_SESSION_REQUIRED", Message: "source requires a runtime session"}
	}
	target, err := expandTemplate(extractor.Source.Fetch.URLTemplate, resolved)
	if err != nil {
		return nil, err
	}
	if err := enforceURLPolicy(ctx, target, options.URLPolicy); err != nil {
		return nil, err
	}
	session := options.Session
	if extractor.Source.SessionPolicy == "none" {
		session = nil
	}
	if lease, ok := options.Browser.(BrowserAdapterLease); ok {
		release, err := lease.Acquire(ctx)
		if err != nil {
			return nil, &ExecutionError{Code: "E_BROWSER_ACQUIRE", Message: err.Error(), Cause: err}
		}
		if release == nil {
			return nil, &ExecutionError{Code: "E_BROWSER_ACQUIRE", Message: "browser adapter returned a nil release function"}
		}
		defer release()
	}
	if err := options.Browser.Navigate(ctx, target, BrowserNavigateOptions{Timeout: options.RequestTimeout, Session: session, UserAgent: options.UserAgent}); err != nil {
		return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_NAVIGATE", err), Message: err.Error(), Cause: err}
	}
	transforms := newTransformRuntime(ctx, extractor, options.ExternalTransforms)
	if err := transforms.preflight(); err != nil {
		return nil, err
	}
	e := &browserEngine{ctx: ctx, extractor: extractor, options: options, adapter: options.Browser, transforms: transforms}
	if err := e.runWorkflow(); err != nil {
		return nil, err
	}
	value, err := e.executeObject(nil, extractor.Output, "output")
	if err != nil {
		return nil, err
	}
	return finalizeResult(value, e.warnings, e.partial), nil
}

func timeout(ms *int, fallback time.Duration) time.Duration {
	if ms == nil {
		return fallback
	}
	return time.Duration(*ms) * time.Millisecond
}

func (e *browserEngine) runWorkflow() error {
	for i, step := range e.extractor.Source.Workflow {
		path := fmt.Sprintf("source.workflow[%d]", i)
		var err error
		switch s := step.(type) {
		case ir.WaitForStep:
			err = e.adapter.WaitFor(e.ctx, s.Selector, s.State, timeout(s.TimeoutMS, e.options.RequestTimeout))
		case ir.ClickStep:
			err = e.adapter.Click(e.ctx, s.Selector, timeout(s.TimeoutMS, e.options.RequestTimeout))
		case ir.FillStep:
			err = e.adapter.Fill(e.ctx, s.Selector, s.Value, timeout(s.TimeoutMS, e.options.RequestTimeout))
		case ir.PressStep:
			err = e.adapter.Press(e.ctx, s.Selector, s.Key, timeout(s.TimeoutMS, e.options.RequestTimeout))
		case ir.ScrollStep:
			err = e.adapter.Scroll(e.ctx, s.X, s.Y)
		case ir.NetworkIdleStep:
			err = e.adapter.WaitForNetworkIdle(e.ctx, time.Duration(s.IdleMS)*time.Millisecond, timeout(s.TimeoutMS, e.options.RequestTimeout))
		case ir.EvaluateJavaScriptStep:
			_, err = e.adapter.Evaluate(e.ctx, s.Source, BrowserEvaluateOptions{Timeout: timeout(s.TimeoutMS, e.options.RequestTimeout)})
		default:
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown workflow step %T", step), Path: path}
		}
		if err != nil {
			return &ExecutionError{Code: operationErrorCode("E_BROWSER_WORKFLOW", err), Message: err.Error(), Path: path, Cause: err}
		}
	}
	return nil
}

func (e *browserEngine) executeObject(scope BrowserElement, object ir.OutputObject, path string) (map[string]any, error) {
	out := make(map[string]any, len(object.Members))
	for _, member := range object.Members {
		switch m := member.(type) {
		case ir.Field:
			v, err := e.executeField(scope, m, path+"."+m.Name)
			if err != nil {
				return nil, err
			}
			out[m.Name] = v
		case ir.Collection:
			v, err := e.executeCollection(scope, m, path+"."+m.Name)
			if err != nil {
				return nil, err
			}
			out[m.Name] = v
		default:
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown output member %T", member), Path: path}
		}
	}
	return out, nil
}

func (e *browserEngine) executeField(scope BrowserElement, field ir.Field, path string) (any, error) {
	value, err := e.readField(scope, field, path)
	if err != nil {
		if _, ok := err.(missingValue); ok {
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

func (e *browserEngine) readField(scope BrowserElement, field ir.Field, path string) (any, error) {
	selected := scope
	if field.Selection != nil {
		matches, err := e.adapter.QueryAll(e.ctx, scope, field.Selection.Selector)
		if err != nil {
			return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_QUERY", err), Message: err.Error(), Path: path, Cause: err}
		}
		if len(matches) == 0 {
			return nil, missingValue{message: fmt.Sprintf("selector %q matched no elements", field.Selection.Selector)}
		}
		if field.Selection.Match == "one" && len(matches) != 1 {
			return nil, &ExecutionError{Code: "E_SELECTOR_CARDINALITY", Message: fmt.Sprintf("selector %q matched %d elements; expected exactly one", field.Selection.Selector, len(matches)), Path: path}
		}
		selected = matches[0]
	}
	switch s := field.ValueSource.(type) {
	case ir.TextValueSource:
		if selected == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "text value source has no selected element", Path: path}
		}
		v, err := e.adapter.Text(e.ctx, selected)
		if err != nil {
			return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_READ", err), Message: err.Error(), Path: path, Cause: err}
		}
		return v, nil
	case ir.HTMLValueSource:
		if selected == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "HTML value source has no selected element", Path: path}
		}
		v, err := e.adapter.HTML(e.ctx, selected)
		if err != nil {
			return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_READ", err), Message: err.Error(), Path: path, Cause: err}
		}
		return v, nil
	case ir.AttributeValueSource:
		if selected == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "attribute value source has no selected element", Path: path}
		}
		v, ok, err := e.adapter.Attribute(e.ctx, selected, s.Name)
		if err != nil {
			return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_READ", err), Message: err.Error(), Path: path, Cause: err}
		}
		if !ok {
			return nil, missingValue{message: fmt.Sprintf("attribute %q is missing", s.Name)}
		}
		return v, nil
	case ir.JavaScriptValueSource:
		jsScope := BrowserElement(nil)
		if s.Scope == "current" {
			jsScope = selected
			if jsScope == nil {
				return nil, &ExecutionError{Code: "E_CURRENT_SCOPE_UNAVAILABLE", Message: "current-scoped JavaScript has no element", Path: path}
			}
		}
		v, err := e.adapter.Evaluate(e.ctx, s.Source, BrowserEvaluateOptions{Timeout: timeout(s.TimeoutMS, e.options.RequestTimeout), Scope: jsScope})
		if err != nil {
			return nil, &ExecutionError{Code: operationErrorCode("E_JAVASCRIPT_EVALUATION", err), Message: err.Error(), Path: path, Cause: err}
		}
		v, ok := normalizeJSONResult(v, s.Returns)
		if !ok {
			return nil, &ExecutionError{Code: "E_JAVASCRIPT_RESULT_TYPE", Message: fmt.Sprintf("JavaScript result of type %T is not compatible with returns=%s", v, s.Returns.String()), Path: path}
		}
		return v, nil
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown value source %T", field.ValueSource), Path: path}
	}
}

func normalizeJSONResult(value any, target typesys.Type) (any, bool) {
	if !isJSONCompatible(value) {
		return value, false
	}
	if target.Kind == typesys.KindNullable {
		if value == nil {
			return nil, true
		}
		if target.Inner == nil {
			return value, false
		}
		return normalizeJSONResult(value, *target.Inner)
	}
	if value == nil {
		return value, target.Kind == typesys.KindPrimitive && target.Name == "unknown"
	}
	if target.Kind == typesys.KindArray {
		if target.Element == nil {
			return value, false
		}
		var values []any
		switch typed := value.(type) {
		case []any:
			values = typed
		case []string:
			values = make([]any, len(typed))
			for i := range typed {
				values[i] = typed[i]
			}
		default:
			return value, false
		}
		normalized := make([]any, len(values))
		for i := range values {
			var ok bool
			normalized[i], ok = normalizeJSONResult(values[i], *target.Element)
			if !ok {
				return value, false
			}
		}
		return normalized, true
	}
	if target.Kind != typesys.KindPrimitive {
		return value, false
	}
	switch target.Name {
	case "unknown":
		return value, true
	case "object":
		_, ok := value.(map[string]any)
		return value, ok
	case "string":
		_, ok := value.(string)
		return value, ok
	case "bool":
		_, ok := value.(bool)
		return value, ok
	case "int", "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64":
		return normalizeJSONInteger(value, target.Name)
	case "float", "f32", "f64":
		return normalizeJSONFloat(value, target.Name)
	default:
		return value, false
	}
}

func normalizeJSONInteger(value any, target string) (any, bool) {
	integer, ok := jsonInteger(value)
	if !ok {
		return value, false
	}
	if target == "u64" {
		if !integer.IsUint64() {
			return value, false
		}
		return integer.Uint64(), true
	}
	if target[0] == 'u' {
		if !integer.IsUint64() {
			return value, false
		}
		converted := integer.Uint64()
		switch target {
		case "u8":
			if converted <= math.MaxUint8 {
				return uint8(converted), true
			}
		case "u16":
			if converted <= math.MaxUint16 {
				return uint16(converted), true
			}
		case "u32":
			if converted <= math.MaxUint32 {
				return uint32(converted), true
			}
		}
		return value, false
	}
	if !integer.IsInt64() {
		return value, false
	}
	converted := integer.Int64()
	switch target {
	case "int", "i64":
		return converted, true
	case "i8":
		if converted >= math.MinInt8 && converted <= math.MaxInt8 {
			return int8(converted), true
		}
	case "i16":
		if converted >= math.MinInt16 && converted <= math.MaxInt16 {
			return int16(converted), true
		}
	case "i32":
		if converted >= math.MinInt32 && converted <= math.MaxInt32 {
			return int32(converted), true
		}
	}
	return value, false
}

func jsonInteger(value any) (*big.Int, bool) {
	switch typed := value.(type) {
	case int:
		return big.NewInt(int64(typed)), true
	case int8:
		return big.NewInt(int64(typed)), true
	case int16:
		return big.NewInt(int64(typed)), true
	case int32:
		return big.NewInt(int64(typed)), true
	case int64:
		return big.NewInt(typed), true
	case uint:
		return new(big.Int).SetUint64(uint64(typed)), true
	case uint8:
		return new(big.Int).SetUint64(uint64(typed)), true
	case uint16:
		return new(big.Int).SetUint64(uint64(typed)), true
	case uint32:
		return new(big.Int).SetUint64(uint64(typed)), true
	case uint64:
		return new(big.Int).SetUint64(typed), true
	case float32:
		return integerFromDecimal(strconv.FormatFloat(float64(typed), 'g', -1, 32))
	case float64:
		return integerFromDecimal(strconv.FormatFloat(typed, 'g', -1, 64))
	case json.Number:
		return integerFromDecimal(typed.String())
	default:
		return nil, false
	}
}

func integerFromDecimal(value string) (*big.Int, bool) {
	rational, ok := new(big.Rat).SetString(value)
	if !ok || !rational.IsInt() {
		return nil, false
	}
	return new(big.Int).Set(rational.Num()), true
}

func normalizeJSONFloat(value any, target string) (any, bool) {
	var converted float64
	switch typed := value.(type) {
	case int:
		converted = float64(typed)
	case int8:
		converted = float64(typed)
	case int16:
		converted = float64(typed)
	case int32:
		converted = float64(typed)
	case int64:
		converted = float64(typed)
	case uint:
		converted = float64(typed)
	case uint8:
		converted = float64(typed)
	case uint16:
		converted = float64(typed)
	case uint32:
		converted = float64(typed)
	case uint64:
		converted = float64(typed)
	case float32:
		converted = float64(typed)
	case float64:
		converted = typed
	case json.Number:
		var err error
		converted, err = typed.Float64()
		if err != nil {
			return value, false
		}
	default:
		return value, false
	}
	if math.IsNaN(converted) || math.IsInf(converted, 0) {
		return value, false
	}
	if target == "f32" {
		narrowed := float32(converted)
		if float32IsInvalid(narrowed) {
			return value, false
		}
		return narrowed, true
	}
	return converted, true
}

func (e *browserEngine) handleMissing(field ir.Field, path, message string) (any, error) {
	if field.Required {
		return nil, &ExecutionError{Code: "E_REQUIRED_VALUE_MISSING", Message: message, Path: path}
	}
	if field.Default != nil {
		v, err := decodeJSON(*field.Default)
		if err != nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: err.Error(), Path: path, Cause: err}
		}
		return v, nil
	}
	return nil, nil
}

func (e *browserEngine) recoverField(field ir.Field, path string, cause error) (any, error) {
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
		if ex, ok := cause.(*ExecutionError); ok {
			return nil, ex
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
		if field.Default == nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "on-error default requires a field default", Path: path}
		}
		v, err := decodeJSON(*field.Default)
		if err != nil {
			return nil, &ExecutionError{Code: "E_IR_INVALID", Message: err.Error(), Path: path, Cause: err}
		}
		e.partial = true
		return v, nil
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown on-error policy %q", policy), Path: path}
	}
}

func (e *browserEngine) executeCollection(scope BrowserElement, c ir.Collection, path string) ([]any, error) {
	rows, err := e.adapter.QueryAll(e.ctx, scope, c.Selector)
	if err != nil {
		return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_QUERY", err), Message: err.Error(), Path: path, Cause: err}
	}
	out := make([]any, 0, len(rows))
	for i, row := range rows {
		v, err := e.executeObject(row, c.Row, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			if c.OnRowError != "skip" {
				return nil, err
			}
			e.partial = true
			idx := i
			e.warnings = append(e.warnings, Warning{Code: "W_ROW_SKIPPED", Message: err.Error(), Path: path, Row: &idx})
			continue
		}
		out = append(out, v)
	}
	min := c.MinItems
	if c.Required && min < 1 {
		min = 1
	}
	if len(out) < min {
		return nil, &ExecutionError{Code: "E_COLLECTION_CARDINALITY", Message: fmt.Sprintf("collection has %d rows after recovery; minimum is %d", len(out), min), Path: path}
	}
	if c.MaxItems != nil && len(out) > *c.MaxItems {
		return nil, &ExecutionError{Code: "E_COLLECTION_CARDINALITY", Message: fmt.Sprintf("collection has %d rows; maximum is %d", len(out), *c.MaxItems), Path: path}
	}
	return out, nil
}

func containsJavaScript(extractor *ir.Extractor) bool {
	for _, s := range extractor.Source.Workflow {
		if _, ok := s.(ir.EvaluateJavaScriptStep); ok {
			return true
		}
	}
	var walk func(ir.OutputObject) bool
	walk = func(o ir.OutputObject) bool {
		for _, m := range o.Members {
			switch v := m.(type) {
			case ir.Field:
				if _, ok := v.ValueSource.(ir.JavaScriptValueSource); ok {
					return true
				}
			case ir.Collection:
				if walk(v.Row) {
					return true
				}
			}
		}
		return false
	}
	return walk(extractor.Output)
}
