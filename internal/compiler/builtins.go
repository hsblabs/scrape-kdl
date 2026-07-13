package compiler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/kdl"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

var builtinNames = map[string]struct{}{
	"trim": {}, "normalize-whitespace": {}, "lowercase": {}, "uppercase": {},
	"replace": {}, "regex-replace": {}, "regex-capture": {}, "substring": {},
	"split": {}, "join": {}, "prepend": {}, "append": {}, "parse-int": {},
	"parse-float": {}, "parse-bool": {}, "to-string": {}, "empty-to-null": {},
	"coalesce": {}, "url-resolve": {}, "url-query": {}, "url-path": {},
	"path-segment": {}, "assert-matches": {}, "assert-enum": {},
	"assert-min": {}, "assert-max": {},
}

func isBuiltin(name string) bool {
	_, ok := builtinNames[name]
	return ok
}

func applyBuiltin(name string, input typesys.Type, node *kdl.Node) (typesys.Type, error) {
	stringToString := func() (typesys.Type, error) {
		if !typesys.IsString(input) {
			return typesys.Type{}, fmt.Errorf("requires string input, got %s", input.String())
		}
		return typesys.Primitive("string"), nil
	}
	switch name {
	case "trim", "normalize-whitespace", "lowercase", "uppercase", "replace", "regex-replace", "substring", "prepend", "append", "url-resolve", "url-path", "assert-matches":
		return stringToString()
	case "regex-capture", "empty-to-null", "url-query", "path-segment":
		if !typesys.IsString(input) {
			return typesys.Type{}, fmt.Errorf("requires string input, got %s", input.String())
		}
		return typesys.Nullable(typesys.Primitive("string")), nil
	case "split":
		if !typesys.IsString(input) {
			return typesys.Type{}, fmt.Errorf("requires string input, got %s", input.String())
		}
		return typesys.Array(typesys.Primitive("string")), nil
	case "join":
		if !typesys.IsStringArray(input) {
			return typesys.Type{}, fmt.Errorf("requires string[] input, got %s", input.String())
		}
		return typesys.Primitive("string"), nil
	case "parse-int":
		if !typesys.IsString(input) {
			return typesys.Type{}, fmt.Errorf("requires string input, got %s", input.String())
		}
		as, ok := stringProperty(node, "as")
		if !ok {
			return typesys.Type{}, fmt.Errorf("requires property as")
		}
		t, err := typesys.Parse(as)
		if err != nil || !typesys.IsInteger(t) {
			return typesys.Type{}, fmt.Errorf("property as must be an integer type")
		}
		return t, nil
	case "parse-float":
		if !typesys.IsString(input) {
			return typesys.Type{}, fmt.Errorf("requires string input, got %s", input.String())
		}
		as, ok := stringProperty(node, "as")
		if !ok {
			return typesys.Type{}, fmt.Errorf("requires property as")
		}
		if as != "float" && as != "f32" && as != "f64" {
			return typesys.Type{}, fmt.Errorf("property as must be float, f32, or f64")
		}
		return typesys.Primitive(as), nil
	case "parse-bool":
		if !typesys.IsString(input) {
			return typesys.Type{}, fmt.Errorf("requires string input, got %s", input.String())
		}
		return typesys.Primitive("bool"), nil
	case "to-string":
		if !(typesys.IsScalar(input) && !(input.Kind == typesys.KindNullable)) {
			return typesys.Type{}, fmt.Errorf("requires non-null scalar input, got %s", input.String())
		}
		return typesys.Primitive("string"), nil
	case "coalesce":
		if input.Kind != typesys.KindNullable || input.Inner == nil {
			return typesys.Type{}, fmt.Errorf("requires nullable input, got %s", input.String())
		}
		return *input.Inner, nil
	case "assert-enum":
		if !typesys.IsScalar(input) {
			return typesys.Type{}, fmt.Errorf("requires scalar input, got %s", input.String())
		}
		return input, nil
	case "assert-min", "assert-max":
		if !typesys.IsNumeric(input) {
			return typesys.Type{}, fmt.Errorf("requires numeric input, got %s", input.String())
		}
		return input, nil
	default:
		return typesys.Type{}, fmt.Errorf("unknown built-in %q", name)
	}
}

func builtinAllowedProperties(name string) map[string]valueExpectation {
	switch name {
	case "trim", "normalize-whitespace", "lowercase", "uppercase", "join", "to-string", "empty-to-null", "url-path":
		if name == "join" {
			return map[string]valueExpectation{"separator": expectString}
		}
		return map[string]valueExpectation{}
	case "replace":
		return map[string]valueExpectation{"old": expectString, "new": expectString, "count": expectNonNegativeInt}
	case "regex-replace":
		return map[string]valueExpectation{"pattern": expectString, "replacement": expectString, "flags": expectString, "count": expectNonNegativeInt}
	case "regex-capture":
		return map[string]valueExpectation{"pattern": expectString, "group": expectNonNegativeInt, "flags": expectString}
	case "substring":
		return map[string]valueExpectation{"start": expectInt, "end": expectInt}
	case "split":
		return map[string]valueExpectation{"separator": expectString, "limit": expectNonNegativeInt}
	case "prepend", "append":
		return map[string]valueExpectation{"value": expectString}
	case "parse-int":
		return map[string]valueExpectation{"as": expectString, "radix": expectInt}
	case "parse-float":
		return map[string]valueExpectation{"as": expectString}
	case "parse-bool":
		return map[string]valueExpectation{"case-sensitive": expectBool, "true": expectString, "false": expectString}
	case "coalesce":
		return map[string]valueExpectation{"value": expectScalar}
	case "url-resolve":
		return map[string]valueExpectation{"base": expectString}
	case "url-query":
		return map[string]valueExpectation{"name": expectString, "index": expectNonNegativeInt}
	case "path-segment":
		return map[string]valueExpectation{"index": expectInt}
	case "assert-matches":
		return map[string]valueExpectation{"pattern": expectString, "flags": expectString}
	case "assert-min", "assert-max":
		return map[string]valueExpectation{"value": expectNumber}
	case "assert-enum":
		return map[string]valueExpectation{}
	default:
		return nil
	}
}

func builtinRequiredProperties(name string) []string {
	switch name {
	case "replace":
		return []string{"old", "new"}
	case "regex-replace":
		return []string{"pattern", "replacement"}
	case "regex-capture":
		return []string{"pattern"}
	case "substring":
		return []string{"start"}
	case "split", "join":
		return []string{"separator"}
	case "prepend", "append", "coalesce", "assert-min", "assert-max":
		return []string{"value"}
	case "parse-int", "parse-float":
		return []string{"as"}
	case "url-resolve":
		return []string{"base"}
	case "url-query":
		return []string{"name"}
	case "path-segment":
		return []string{"index"}
	case "assert-matches":
		return []string{"pattern"}
	default:
		return nil
	}
}

func (c *Compiler) validateBuiltinArguments(name string, input typesys.Type, node *kdl.Node, path string) {
	if flags, ok := stringProperty(node, "flags"); ok {
		seen := map[rune]bool{}
		for _, flag := range flags {
			if flag != 'i' && flag != 'm' && flag != 's' {
				c.diags.Add("E_REGEX_INVALID", diagnostic.SeverityError, fmt.Sprintf("unsupported regex flag %q", flag), node.Span, path)
			}
			if seen[flag] {
				c.diags.Add("E_REGEX_INVALID", diagnostic.SeverityError, fmt.Sprintf("duplicate regex flag %q", flag), node.Span, path)
			}
			seen[flag] = true
		}
	}
	if pattern, ok := stringProperty(node, "pattern"); ok {
		if strings.Contains(pattern, "(?P<") || strings.Contains(pattern, "(?<") {
			c.diags.Add("E_REGEX_INVALID", diagnostic.SeverityError, "named capture groups are outside the portable RE2 profile", node.Span, path)
		} else {
			flags, _ := stringProperty(node, "flags")
			prefix := ""
			if flags != "" {
				prefix = "(?" + flags + ")"
			}
			re, err := regexp.Compile(prefix + pattern)
			if err != nil {
				c.diags.Add("E_REGEX_INVALID", diagnostic.SeverityError, err.Error(), node.Span, path)
			} else if name == "regex-capture" {
				if group, ok := intProperty(node, "group"); ok && group > re.NumSubexp() {
					c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, fmt.Sprintf("capture group %d exceeds pattern capture count %d", group, re.NumSubexp()), node.Span, path)
				}
			}
		}
	}
	switch name {
	case "parse-int":
		if radix, ok := intProperty(node, "radix"); ok && (radix < 2 || radix > 36) {
			c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, "parse-int radix must be between 2 and 36", node.Span, path)
		}
	case "parse-bool":
		trueValue := "true"
		falseValue := "false"
		if value, ok := stringProperty(node, "true"); ok {
			trueValue = value
		}
		if value, ok := stringProperty(node, "false"); ok {
			falseValue = value
		}
		if trueValue == falseValue {
			c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, "parse-bool true and false values must differ", node.Span, path)
		}
	case "coalesce":
		if value, ok := node.Property("value"); ok && input.Kind == typesys.KindNullable && input.Inner != nil && !isValueAssignable(value, *input.Inner) {
			c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, fmt.Sprintf("coalesce value is not assignable to %s", input.Inner.String()), value.Span, path)
		}
	case "assert-enum":
		for _, value := range node.Arguments[1:] {
			if !isValueAssignable(value, input) {
				c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, fmt.Sprintf("enum value is not assignable to %s", input.String()), value.Span, path)
			}
		}
	case "url-resolve":
		if base, ok := stringProperty(node, "base"); ok {
			parsed, err := url.Parse(base)
			if err != nil || !parsed.IsAbs() {
				c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, "url-resolve base must be an absolute URL", node.Span, path)
			}
		}
	}
}
