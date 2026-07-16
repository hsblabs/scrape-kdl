package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/hsblabs/scrape-kdl/internal/dom"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/limits"
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

// BrowserAdapterQueryLimit is an optional capability that avoids materializing
// every match when the runtime only needs the first one or two elements.
type BrowserAdapterQueryLimit interface {
	QueryLimit(context.Context, BrowserElement, string, int) ([]BrowserElement, error)
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
	prepared, err := Prepare(extractor)
	if err != nil {
		return nil, err
	}
	if prepared.mode != "browser" {
		return nil, &ExecutionError{Code: "E_BROWSER_MODE_REQUIRED", Message: fmt.Sprintf("browser runtime cannot execute fetch mode %q", prepared.mode)}
	}
	return executeBrowserPrepared(ctx, prepared, inputs, options)
}

func executeBrowserPrepared(ctx context.Context, prepared *Prepared, inputs map[string]any, options Options) (*Result, error) {
	options = options.withDefaults()
	extractor := prepared.extractor
	if options.Browser == nil {
		return nil, &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: "browser-mode extractor requires Options.Browser"}
	}
	if !options.AllowJavaScript && containsJavaScript(extractor) {
		return nil, &ExecutionError{Code: "E_JAVASCRIPT_DISABLED", Message: "extractor contains JavaScript; set AllowJavaScript=true for trusted specs"}
	}
	transforms := newTransformRuntime(ctx, extractor, options.ExternalTransforms)
	if err := transforms.preflightExternalBindings(); err != nil {
		return nil, err
	}
	if prepared.postBindingError != nil {
		return nil, prepared.postBindingError
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
			return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_ACQUIRE", err), Message: err.Error(), Cause: err}
		}
		if release == nil {
			return nil, &ExecutionError{Code: "E_BROWSER_ACQUIRE", Message: "browser adapter returned a nil release function"}
		}
		defer release()
	}
	if err := options.Browser.Navigate(ctx, target, BrowserNavigateOptions{Timeout: options.RequestTimeout, Session: session, UserAgent: options.UserAgent}); err != nil {
		return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_NAVIGATE", err), Message: err.Error(), Cause: err}
	}
	e := &browserEngine{ctx: ctx, extractor: extractor, options: options, adapter: options.Browser, transforms: transforms}
	if err := e.runWorkflow(); err != nil {
		return nil, err
	}
	walker := newOutputWalker[BrowserElement](ctx, e, transforms, &e.warnings, &e.partial)
	value, err := walker.executeObject(nil, extractor.Output, "output")
	if err != nil {
		return nil, err
	}
	return finalizeResult(value, e.warnings, e.partial), nil
}

func preflightBrowserWorkflow(steps []ir.WorkflowStep) error {
	for index, step := range steps {
		path := fmt.Sprintf("source.workflow[%d]", index)
		selector := ""
		var timeoutMS *int
		waitState := ""
		hasSelector := false
		isWait := false
		switch typed := step.(type) {
		case ir.WaitForStep:
			if typed.Kind != "wait-for" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid wait-for step kind %q", typed.Kind), Path: path}
			}
			selector = typed.Selector
			timeoutMS = typed.TimeoutMS
			waitState = typed.State
			hasSelector = true
			isWait = true
		case ir.ClickStep:
			if typed.Kind != "click" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid click step kind %q", typed.Kind), Path: path}
			}
			selector = typed.Selector
			timeoutMS = typed.TimeoutMS
			hasSelector = true
		case ir.FillStep:
			if typed.Kind != "fill" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid fill step kind %q", typed.Kind), Path: path}
			}
			selector = typed.Selector
			timeoutMS = typed.TimeoutMS
			hasSelector = true
		case ir.PressStep:
			if typed.Kind != "press" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid press step kind %q", typed.Kind), Path: path}
			}
			selector = typed.Selector
			timeoutMS = typed.TimeoutMS
			hasSelector = true
		case ir.ScrollStep:
			if typed.Kind != "scroll" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid scroll step kind %q", typed.Kind), Path: path}
			}
			if math.IsNaN(typed.X) || math.IsInf(typed.X, 0) || math.IsNaN(typed.Y) || math.IsInf(typed.Y, 0) {
				return &ExecutionError{Code: "E_IR_INVALID", Message: "scroll coordinates must be finite", Path: path}
			}
		case ir.NetworkIdleStep:
			if typed.Kind != "wait-for-network-idle" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid network-idle step kind %q", typed.Kind), Path: path}
			}
			if _, ok := limits.Milliseconds(typed.IdleMS); !ok {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("network idleMs must be between 1 and %d", limits.MaxMilliseconds), Path: path}
			}
			timeoutMS = typed.TimeoutMS
		case ir.EvaluateJavaScriptStep:
			if typed.Kind != "evaluate-js" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid evaluate-js step kind %q", typed.Kind), Path: path}
			}
			timeoutMS = typed.TimeoutMS
		default:
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown workflow step %T", step), Path: path}
		}
		if hasSelector {
			if _, err := dom.ParseSelector(selector); err != nil {
				return &ExecutionError{Code: "E_SELECTOR_INVALID", Message: err.Error(), Path: path, Cause: err}
			}
		}
		if isWait && waitState != "attached" && waitState != "visible" && waitState != "hidden" && waitState != "detached" {
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid wait-for state %q", waitState), Path: path}
		}
		if timeoutMS != nil {
			if _, ok := limits.Milliseconds(*timeoutMS); !ok {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("workflow timeoutMs must be between 1 and %d", limits.MaxMilliseconds), Path: path}
			}
		}
	}
	return nil
}

func preflightBrowserOutput(object ir.OutputObject) error {
	for _, member := range object.Members {
		switch typed := member.(type) {
		case ir.Field:
			if typed.Selection != nil {
				if _, err := dom.ParseSelector(typed.Selection.Selector); err != nil {
					return &ExecutionError{Code: "E_SELECTOR_INVALID", Message: err.Error(), Path: typed.ID, Cause: err}
				}
			}
			switch typed.ValueSource.(type) {
			case ir.TextValueSource, ir.HTMLValueSource, ir.AttributeValueSource, ir.JavaScriptValueSource:
			default:
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown value source %T", typed.ValueSource), Path: typed.ID}
			}
		case ir.Collection:
			if _, err := dom.ParseSelector(typed.Selector); err != nil {
				return &ExecutionError{Code: "E_SELECTOR_INVALID", Message: err.Error(), Path: typed.ID, Cause: err}
			}
			if err := preflightBrowserOutput(typed.Row); err != nil {
				return err
			}
		default:
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown output member %T", member)}
		}
	}
	return nil
}

func timeout(ms *int, fallback time.Duration) time.Duration {
	if ms == nil {
		return fallback
	}
	duration, _ := limits.Milliseconds(*ms)
	return duration
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
			idle, _ := limits.Milliseconds(s.IdleMS)
			err = e.adapter.WaitForNetworkIdle(e.ctx, idle, timeout(s.TimeoutMS, e.options.RequestTimeout))
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

func (e *browserEngine) readOutputField(scope BrowserElement, field ir.Field, path string) (any, error) {
	selected := scope
	if field.Selection != nil {
		limit := 1
		if field.Selection.Match == "one" {
			limit = 2
		}
		matches, err := queryBrowser(e.ctx, e.adapter, scope, field.Selection.Selector, limit)
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

func (e *browserEngine) queryOutputRows(scope BrowserElement, collection ir.Collection, path string) ([]BrowserElement, error) {
	rows, err := e.adapter.QueryAll(e.ctx, scope, collection.Selector)
	if err != nil {
		return nil, &ExecutionError{Code: operationErrorCode("E_BROWSER_QUERY", err), Message: err.Error(), Path: path, Cause: err}
	}
	return rows, nil
}

func queryBrowser(ctx context.Context, adapter BrowserAdapter, scope BrowserElement, selector string, limit int) ([]BrowserElement, error) {
	if limited, ok := adapter.(BrowserAdapterQueryLimit); ok {
		return limited.QueryLimit(ctx, scope, selector, limit)
	}
	return adapter.QueryAll(ctx, scope, selector)
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

// NormalizeBrowserResult validates and normalizes the concrete Go value forms
// accepted from BrowserAdapter.Evaluate. It does not use reflection or invoke
// json.Marshaler implementations.
func NormalizeBrowserResult(value any) (any, error) {
	switch typed := value.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return value, nil
	case json.Number:
		if !isJSONCompatible(typed) {
			return nil, fmt.Errorf("browser result contains an invalid JSON number")
		}
		return typed, nil
	case float32:
		if float32IsInvalid(typed) {
			return nil, fmt.Errorf("browser result contains a non-finite number")
		}
		return typed, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, fmt.Errorf("browser result contains a non-finite number")
		}
		return typed, nil
	case []string:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = item
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			normalized, err := NormalizeBrowserResult(item)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		for name, item := range typed {
			normalized, err := NormalizeBrowserResult(item)
			if err != nil {
				return nil, err
			}
			result[name] = normalized
		}
		return result, nil
	default:
		return nil, fmt.Errorf("browser result has unsupported Go type %T", value)
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
