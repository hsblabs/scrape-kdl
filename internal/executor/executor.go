package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/compatibility"
	"github.com/hsblabs/scrape-kdl/internal/dom"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/limits"
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

var (
	irRootNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	irSHA256Pattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

func (m missingValue) Error() string { return m.message }

func Execute(ctx context.Context, extractor *ir.Extractor, inputs map[string]any, options Options) (*Result, error) {
	prepared, err := Prepare(extractor)
	if err != nil {
		return nil, err
	}
	return prepared.Execute(ctx, inputs, options)
}

func executeHTTPPrepared(ctx context.Context, prepared *Prepared, inputs map[string]any, options Options) (*Result, error) {
	options = options.withDefaults()
	extractor := prepared.extractor
	engine, err := newPreparedEngine(ctx, prepared, options)
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
	prepared, err := Prepare(extractor)
	if err != nil {
		return nil, err
	}
	return prepared.ExecuteHTML(ctx, html, options)
}

func executeHTMLPrepared(ctx context.Context, prepared *Prepared, html string, options Options) (*Result, error) {
	options = options.withDefaults()
	engine, err := newPreparedEngine(ctx, prepared, options)
	if err != nil {
		return nil, err
	}
	if err := executionContextError(ctx, "output"); err != nil {
		return nil, err
	}
	document, err := dom.ParseHTML(strings.NewReader(html))
	if err != nil {
		return nil, &ExecutionError{Code: "E_HTML_PARSE", Message: err.Error(), Cause: err}
	}
	return engine.executeDocument(document)
}

func newEngine(ctx context.Context, extractor *ir.Extractor, options Options) (*engine, error) {
	prepared, err := Prepare(extractor)
	if err != nil {
		return nil, err
	}
	return newPreparedEngine(ctx, prepared, options)
}

func newPreparedEngine(ctx context.Context, prepared *Prepared, options Options) (*engine, error) {
	extractor := prepared.extractor
	if prepared.mode != "http" {
		return nil, &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: fmt.Sprintf("HTTP runtime cannot execute fetch mode %q", prepared.mode)}
	}
	transforms := newTransformRuntime(ctx, extractor, options.ExternalTransforms)
	if err := transforms.preflightExternalBindings(); err != nil {
		return nil, err
	}
	if prepared.postBindingError != nil {
		return nil, prepared.postBindingError
	}
	result := &engine{
		ctx: ctx, extractor: extractor, options: options, transforms: transforms,
		selectors: prepared.selectors,
	}
	return result, nil
}

func preflightExtractorStructure(extractor *ir.Extractor) error {
	if extractor.Kind != "extractor" {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown extractor kind %q", extractor.Kind), Path: "extractor"}
	}
	if !compatibility.IsDateIdentifier(extractor.IRVersion) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("malformed IR version %q", extractor.IRVersion), Path: "irVersion"}
	}
	if !compatibility.IsSupportedIRVersion(extractor.IRVersion) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unsupported IR version %q", extractor.IRVersion), Path: "irVersion"}
	}
	if !compatibility.IsDateIdentifier(extractor.LanguageVersion) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("malformed language version %q", extractor.LanguageVersion), Path: "languageVersion"}
	}
	if !compatibility.IsSupportedLanguageVersion(extractor.LanguageVersion) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unsupported language version %q", extractor.LanguageVersion), Path: "languageVersion"}
	}
	if !compatibility.IsDateIdentifier(extractor.Version) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: "extractor version must be a real calendar date in YYYY-MM-DD form", Path: "version"}
	}
	if !irRootNamePattern.MatchString(extractor.Name) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid extractor name %q", extractor.Name), Path: "name"}
	}
	if err := preflightSourceFiles(extractor); err != nil {
		return err
	}
	return nil
}

func preflightCapabilities(extractor *ir.Extractor) error {
	expectedCapabilities := deriveCapabilities(extractor)
	if !slices.Equal(extractor.Capabilities, expectedCapabilities) {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("capabilities %q do not match derived sorted capabilities %q", extractor.Capabilities, expectedCapabilities), Path: "capabilities"}
	}
	return nil
}

func preflightSourceFiles(extractor *ir.Extractor) error {
	if len(extractor.Files) == 0 {
		return &ExecutionError{Code: "E_IR_INVALID", Message: "extractor requires at least one source file", Path: "files"}
	}
	knownPaths := make(map[string]struct{}, len(extractor.Files))
	previousPath := ""
	for index, file := range extractor.Files {
		path := fmt.Sprintf("files[%d]", index)
		if file.Path == "" {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "source file path must be non-empty", Path: path + ".path"}
		}
		if index > 0 && file.Path <= previousPath {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "source files must have unique lexicographically sorted paths", Path: path + ".path"}
		}
		previousPath = file.Path
		knownPaths[file.Path] = struct{}{}
		if !irSHA256Pattern.MatchString(file.SHA256) {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "source file sha256 must be 64 lowercase hexadecimal characters", Path: path + ".sha256"}
		}
		if (file.ModuleName == "") != (file.ModuleVersion == "") {
			return &ExecutionError{Code: "E_IR_INVALID", Message: "moduleName and moduleVersion must be present together", Path: path}
		}
		if file.ModuleName != "" {
			if !irRootNamePattern.MatchString(file.ModuleName) {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid module name %q", file.ModuleName), Path: path + ".moduleName"}
			}
			if !compatibility.IsDateIdentifier(file.ModuleVersion) {
				return &ExecutionError{Code: "E_IR_INVALID", Message: "moduleVersion must be a real calendar date in YYYY-MM-DD form", Path: path + ".moduleVersion"}
			}
		}
	}
	if _, ok := knownPaths[extractor.Span.File]; !ok {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("extractor span references unknown source file %q", extractor.Span.File), Path: "span.file"}
	}
	return nil
}

func deriveCapabilities(extractor *ir.Extractor) []string {
	set := map[string]struct{}{}
	if extractor.Source.Fetch.Mode == "http" {
		set["http.fetch"] = struct{}{}
	} else if extractor.Source.Fetch.Mode == "browser" {
		set["browser.navigate"] = struct{}{}
	}
	for _, step := range extractor.Source.Workflow {
		switch step.(type) {
		case ir.WaitForStep:
			set["browser.wait"] = struct{}{}
		case ir.ClickStep, ir.FillStep, ir.PressStep:
			set["browser.input"] = struct{}{}
		case ir.ScrollStep:
			set["browser.scroll"] = struct{}{}
		case ir.NetworkIdleStep:
			set["browser.network-idle"] = struct{}{}
		case ir.EvaluateJavaScriptStep:
			set["browser.evaluate-js"] = struct{}{}
		}
	}
	for _, transform := range extractor.Transforms {
		if external, ok := transform.(ir.ExternalTransform); ok && external.Symbol != "" {
			set["transform.external:"+external.Symbol] = struct{}{}
		}
	}
	if extractor.Source.Fetch.Mode == "browser" {
		deriveOutputCapabilities(extractor.Output, set)
	}
	capabilities := make([]string, 0, len(set))
	for capability := range set {
		capabilities = append(capabilities, capability)
	}
	slices.Sort(capabilities)
	return capabilities
}

func deriveOutputCapabilities(object ir.OutputObject, set map[string]struct{}) {
	for _, member := range object.Members {
		switch typed := member.(type) {
		case ir.Field:
			switch source := typed.ValueSource.(type) {
			case ir.TextValueSource:
				set["browser.query"] = struct{}{}
				set["browser.read-text"] = struct{}{}
			case ir.HTMLValueSource:
				set["browser.query"] = struct{}{}
				set["browser.read-html"] = struct{}{}
			case ir.AttributeValueSource:
				set["browser.query"] = struct{}{}
				set["browser.read-attr"] = struct{}{}
			case ir.JavaScriptValueSource:
				set["browser.evaluate-js"] = struct{}{}
				if typed.Selection != nil || source.Scope == "current" {
					set["browser.query"] = struct{}{}
				}
			}
		case ir.Collection:
			set["browser.query"] = struct{}{}
			deriveOutputCapabilities(typed.Row, set)
		}
	}
}

func preflightSourceStructure(source ir.Source) error {
	if source.Kind != "html" {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown source kind %q", source.Kind), Path: "source"}
	}
	if source.SessionPolicy != "none" && source.SessionPolicy != "optional" && source.SessionPolicy != "required" {
		return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown session policy %q", source.SessionPolicy), Path: "source.session"}
	}
	for index, segment := range source.Fetch.URLTemplate.Segments {
		switch typed := segment.(type) {
		case ir.LiteralTemplateSegment:
			if typed.Kind != "literal" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid literal URL template segment kind %q", typed.Kind), Path: fmt.Sprintf("source.fetch.urlTemplate.segments[%d]", index)}
			}
		case ir.InputTemplateSegment:
			if typed.Kind != "input" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid input URL template segment kind %q", typed.Kind), Path: fmt.Sprintf("source.fetch.urlTemplate.segments[%d]", index)}
			}
			if typed.Name == "" {
				return &ExecutionError{Code: "E_IR_INVALID", Message: "URL template input segment name must be non-empty", Path: fmt.Sprintf("source.fetch.urlTemplate.segments[%d]", index)}
			}
		default:
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown URL template segment %T", segment), Path: fmt.Sprintf("source.fetch.urlTemplate.segments[%d]", index)}
		}
	}
	return nil
}

func preflightOutputStructure(root ir.OutputObject) error {
	seenIDs := map[string]struct{}{}
	var walk func(ir.OutputObject, string) error
	walk = func(object ir.OutputObject, path string) error {
		if object.Kind != "object" {
			return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid output object kind %q", object.Kind), Path: path}
		}
		seenNames := map[string]struct{}{}
		for _, member := range object.Members {
			var name, id string
			var row *ir.OutputObject
			switch typed := member.(type) {
			case ir.Field:
				name, id = typed.Name, typed.ID
				if typed.Kind != "field" {
					return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid field kind %q", typed.Kind), Path: typed.ID}
				}
				if !validRuntimeType(typed.SuccessfulType) {
					return &ExecutionError{Code: "E_IR_INVALID", Message: "field has an invalid successful type", Path: typed.ID}
				}
				if !validRuntimeType(typed.EffectiveType) {
					return &ExecutionError{Code: "E_IR_INVALID", Message: "field has an invalid effective type", Path: typed.ID}
				}
				if typed.Selection != nil && typed.Selection.Match != "one" && typed.Selection.Match != "first" {
					return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid selection match mode %q", typed.Selection.Match), Path: typed.ID}
				}
				switch source := typed.ValueSource.(type) {
				case ir.TextValueSource:
					if source.Kind != "text" {
						return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid text value-source kind %q", source.Kind), Path: typed.ID}
					}
					if !typesys.Equal(source.RawType, typesys.Primitive("string")) {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "text value-source rawType must be string", Path: typed.ID}
					}
					if typed.Selection == nil {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "value source requires a selection", Path: typed.ID}
					}
				case ir.HTMLValueSource:
					if source.Kind != "html" {
						return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid HTML value-source kind %q", source.Kind), Path: typed.ID}
					}
					if !typesys.Equal(source.RawType, typesys.Primitive("string")) {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "HTML value-source rawType must be string", Path: typed.ID}
					}
					if typed.Selection == nil {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "value source requires a selection", Path: typed.ID}
					}
				case ir.AttributeValueSource:
					if source.Kind != "attribute" {
						return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid attribute value-source kind %q", source.Kind), Path: typed.ID}
					}
					if !typesys.Equal(source.RawType, typesys.Primitive("string")) {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "attribute value-source rawType must be string", Path: typed.ID}
					}
					if typed.Selection == nil {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "value source requires a selection", Path: typed.ID}
					}
					if source.Name == "" {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "attribute value source requires a non-empty name", Path: typed.ID}
					}
				case ir.JavaScriptValueSource:
					if source.Kind != "javascript" {
						return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid JavaScript value-source kind %q", source.Kind), Path: typed.ID}
					}
					if !validRuntimeType(source.Returns) {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "JavaScript value source has an invalid returns type", Path: typed.ID}
					}
					if source.Scope != "document" && source.Scope != "current" {
						return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid JavaScript scope %q", source.Scope), Path: typed.ID}
					}
					if source.TimeoutMS != nil {
						if _, ok := limits.Milliseconds(*source.TimeoutMS); !ok {
							return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("JavaScript timeoutMs must be between 1 and %d", limits.MaxMilliseconds), Path: typed.ID}
						}
					}
					if source.Scope == "document" && typed.Selection != nil {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "document-scoped JavaScript forbids a selection", Path: typed.ID}
					}
					if source.Scope == "current" && typed.Selection == nil && !strings.Contains(path, "[]") {
						return &ExecutionError{Code: "E_IR_INVALID", Message: "top-level current-scoped JavaScript requires a selection", Path: typed.ID}
					}
				}
				if typed.Required && typed.Default != nil {
					return &ExecutionError{Code: "E_IR_INVALID", Message: "required field must not declare a default", Path: typed.ID}
				}
				if typed.Default != nil {
					value, err := decodeJSON(*typed.Default)
					if err != nil {
						return &ExecutionError{Code: "E_IR_INVALID", Message: err.Error(), Path: typed.ID, Cause: err}
					}
					if _, ok := normalizeJSONResult(value, typed.SuccessfulType); !ok {
						return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("field default of type %T is not assignable to %s", value, typed.SuccessfulType.String()), Path: typed.ID}
					}
				}
				if typed.OnError != "" && typed.OnError != "fail" && typed.OnError != "null" && typed.OnError != "warn" && typed.OnError != "default" {
					return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown on-error policy %q", typed.OnError), Path: typed.ID}
				}
				if (typed.OnError == "null" || typed.OnError == "warn") && !typesys.IsNullable(typed.EffectiveType) {
					return &ExecutionError{Code: "E_IR_INVALID", Message: typed.OnError + " requires nullable effective type", Path: typed.ID}
				}
				if typed.OnError == "default" && typed.Default == nil {
					return &ExecutionError{Code: "E_IR_INVALID", Message: "default policy requires a field default", Path: typed.ID}
				}
			case ir.Collection:
				name, id, row = typed.Name, typed.ID, &typed.Row
				if typed.Kind != "collection" {
					return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("invalid collection kind %q", typed.Kind), Path: typed.ID}
				}
				if typed.MinItems < 0 {
					return &ExecutionError{Code: "E_IR_INVALID", Message: "collection minItems must be non-negative", Path: typed.ID}
				}
				effectiveMinimum := typed.MinItems
				if typed.Required && effectiveMinimum < 1 {
					effectiveMinimum = 1
				}
				if typed.MaxItems != nil && *typed.MaxItems < effectiveMinimum {
					return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("collection maxItems %d is less than effective minItems %d", *typed.MaxItems, effectiveMinimum), Path: typed.ID}
				}
				if typed.OnRowError != "fail" && typed.OnRowError != "skip" {
					return &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown on-row-error policy %q", typed.OnRowError), Path: typed.ID}
				}
				if len(typed.Row.Members) == 0 {
					return &ExecutionError{Code: "E_IR_INVALID", Message: "collection row requires at least one output member", Path: typed.ID}
				}
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

func validRuntimeType(value typesys.Type) bool {
	parsed, err := typesys.Parse(value.String())
	return err == nil && typesys.Equal(parsed, value)
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
	walker := newOutputWalker[*dom.Node](e.ctx, e, e.transforms, &e.warnings, &e.partial)
	value, err := walker.executeObject(document, e.extractor.Output, "output")
	if err != nil {
		return nil, err
	}
	return finalizeResult(value, e.warnings, e.partial), nil
}

func (e *engine) readOutputField(scope *dom.Node, field ir.Field, path string) (any, error) {
	var selected *dom.Node
	if field.Selection != nil {
		selector, _ := e.selector(field.Selection.Selector)
		limit := 1
		if field.Selection.Match == "one" {
			limit = 2
		}
		matches := dom.QueryLimit(scope, selector, limit)
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

func (e *engine) queryOutputRows(scope *dom.Node, collection ir.Collection, _ string) ([]*dom.Node, error) {
	selector, _ := e.selector(collection.Selector)
	return dom.QueryAll(scope, selector), nil
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
