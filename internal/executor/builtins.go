package executor

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func isKnownBuiltinRuntime(name string) bool {
	switch name {
	case "trim", "normalize-whitespace", "lowercase", "uppercase",
		"replace", "regex-replace", "regex-capture", "substring",
		"split", "join", "prepend", "append", "parse-int",
		"parse-float", "parse-bool", "to-string", "empty-to-null",
		"coalesce", "url-resolve", "url-query", "url-path",
		"path-segment", "assert-matches", "assert-enum", "assert-min", "assert-max":
		return true
	default:
		return false
	}
}

func applyBuiltinRuntime(name string, input any, call ir.TransformCall) (any, error) {
	arguments := make(map[string]json.RawMessage, len(call.NamedArguments))
	for _, argument := range call.NamedArguments {
		arguments[argument.Name] = argument.Value
	}
	switch name {
	case "trim":
		value, err := requireString(input)
		return strings.TrimSpace(value), err
	case "normalize-whitespace":
		value, err := requireString(input)
		if err != nil {
			return nil, err
		}
		return strings.Join(strings.Fields(value), " "), nil
	case "lowercase":
		value, err := requireString(input)
		return strings.ToLower(value), err
	case "uppercase":
		value, err := requireString(input)
		return strings.ToUpper(value), err
	case "replace":
		return builtinReplace(input, arguments)
	case "regex-replace":
		return builtinRegexReplace(input, arguments)
	case "regex-capture":
		return builtinRegexCapture(input, arguments)
	case "substring":
		return builtinSubstring(input, arguments)
	case "split":
		return builtinSplit(input, arguments)
	case "join":
		return builtinJoin(input, arguments)
	case "prepend":
		value, err := requireString(input)
		if err != nil {
			return nil, err
		}
		prefix, err := requiredStringArgument(arguments, "value")
		return prefix + value, err
	case "append":
		value, err := requireString(input)
		if err != nil {
			return nil, err
		}
		suffix, err := requiredStringArgument(arguments, "value")
		return value + suffix, err
	case "parse-int":
		return builtinParseInt(input, arguments)
	case "parse-float":
		return builtinParseFloat(input, arguments)
	case "parse-bool":
		return builtinParseBool(input, arguments)
	case "to-string":
		return builtinToString(input)
	case "empty-to-null":
		value, err := requireString(input)
		if err != nil {
			return nil, err
		}
		if value == "" {
			return nil, nil
		}
		return value, nil
	case "coalesce":
		if input != nil {
			return input, nil
		}
		value, ok, err := namedArgument(arguments, "value")
		if err != nil || !ok {
			return nil, argumentError("coalesce requires value", err)
		}
		normalized, compatible := normalizeJSONResult(value, call.Output)
		if !compatible {
			return nil, fmt.Errorf("coalesce value of type %T is not assignable to %s", value, call.Output.String())
		}
		return normalized, nil
	case "url-resolve":
		return builtinURLResolve(input, arguments)
	case "url-query":
		return builtinURLQuery(input, arguments)
	case "url-path":
		return builtinURLPath(input)
	case "path-segment":
		return builtinPathSegment(input, arguments)
	case "assert-matches":
		return builtinAssertMatches(input, arguments)
	case "assert-enum":
		return builtinAssertEnum(input, call.PositionalArguments)
	case "assert-min":
		return builtinAssertBound(input, arguments, true)
	case "assert-max":
		return builtinAssertBound(input, arguments, false)
	default:
		return nil, fmt.Errorf("unknown built-in %q", name)
	}
}

func requireString(input any) (string, error) {
	value, ok := input.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", input)
	}
	return value, nil
}

func requiredStringArgument(arguments map[string]json.RawMessage, name string) (string, error) {
	value, ok, err := namedArgument(arguments, name)
	if err != nil {
		return "", err
	}
	stringValue, typeOK := value.(string)
	if !ok || !typeOK {
		return "", fmt.Errorf("argument %q must be a string", name)
	}
	return stringValue, nil
}

func requiredIntArgument(arguments map[string]json.RawMessage, name string) (int, error) {
	value, ok, err := namedArgument(arguments, name)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("argument %q is required", name)
	}
	parsed, err := literalInt(value)
	if err != nil {
		return 0, fmt.Errorf("argument %q: %w", name, err)
	}
	return parsed, nil
}

func optionalIntArgument(arguments map[string]json.RawMessage, name string, fallback int) (int, error) {
	value, ok, err := namedArgument(arguments, name)
	if err != nil || !ok {
		return fallback, err
	}
	parsed, err := literalInt(value)
	if err != nil {
		return 0, fmt.Errorf("argument %q: %w", name, err)
	}
	return parsed, nil
}

func optionalNonNegativeIntArgument(arguments map[string]json.RawMessage, name string, fallback int) (int, error) {
	value, ok, err := namedArgument(arguments, name)
	if err != nil || !ok {
		return fallback, err
	}
	parsed, err := literalInt(value)
	if err != nil {
		return 0, fmt.Errorf("argument %q: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("argument %q must be non-negative", name)
	}
	return parsed, nil
}

func optionalBoolArgument(arguments map[string]json.RawMessage, name string, fallback bool) (bool, error) {
	value, ok, err := namedArgument(arguments, name)
	if err != nil || !ok {
		return fallback, err
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("argument %q must be boolean", name)
	}
	return parsed, nil
}

func optionalStringArgument(arguments map[string]json.RawMessage, name, fallback string) (string, error) {
	value, ok, err := namedArgument(arguments, name)
	if err != nil || !ok {
		return fallback, err
	}
	parsed, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", name)
	}
	return parsed, nil
}

func literalInt(value any) (int, error) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseInt(typed.String(), 10, 32)
		return int(parsed), err
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(typed), nil
	default:
		return 0, fmt.Errorf("must be an integer, got %T", value)
	}
}

func builtinReplace(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	oldValue, err := requiredStringArgument(arguments, "old")
	if err != nil {
		return nil, err
	}
	newValue, err := requiredStringArgument(arguments, "new")
	if err != nil {
		return nil, err
	}
	count, err := optionalNonNegativeIntArgument(arguments, "count", -1)
	if err != nil {
		return nil, err
	}
	return strings.Replace(value, oldValue, newValue, count), nil
}

func compileRuntimeRegex(arguments map[string]json.RawMessage) (*regexp.Regexp, error) {
	pattern, err := requiredStringArgument(arguments, "pattern")
	if err != nil {
		return nil, err
	}
	flags := ""
	if value, ok, err := namedArgument(arguments, "flags"); err != nil {
		return nil, err
	} else if ok {
		var typeOK bool
		flags, typeOK = value.(string)
		if !typeOK {
			return nil, fmt.Errorf("flags must be a string")
		}
	}
	if flags != "" {
		seen := map[rune]bool{}
		for _, flag := range flags {
			if flag != 'i' && flag != 'm' && flag != 's' {
				return nil, fmt.Errorf("unsupported regex flag %q", flag)
			}
			if seen[flag] {
				return nil, fmt.Errorf("duplicate regex flag %q", flag)
			}
			seen[flag] = true
		}
		pattern = "(?" + flags + ")" + pattern
	}
	if strings.Contains(pattern, "(?P<") || strings.Contains(pattern, "(?<") {
		return nil, fmt.Errorf("named capture groups are outside the portable RE2 profile")
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex: %w", err)
	}
	return compiled, nil
}

func builtinRegexReplace(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRuntimeRegex(arguments)
	if err != nil {
		return nil, err
	}
	replacement, err := requiredStringArgument(arguments, "replacement")
	if err != nil {
		return nil, err
	}
	count, err := optionalNonNegativeIntArgument(arguments, "count", -1)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return value, nil
	}
	if count < 0 {
		return compiled.ReplaceAllString(value, replacement), nil
	}
	indexes := compiled.FindAllStringSubmatchIndex(value, count)
	if len(indexes) == 0 {
		return value, nil
	}
	var builder strings.Builder
	cursor := 0
	for _, index := range indexes {
		builder.WriteString(value[cursor:index[0]])
		builder.Write(compiled.ExpandString(nil, replacement, value, index))
		cursor = index[1]
	}
	builder.WriteString(value[cursor:])
	return builder.String(), nil
}

func builtinRegexCapture(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRuntimeRegex(arguments)
	if err != nil {
		return nil, err
	}
	group, err := optionalIntArgument(arguments, "group", 0)
	if err != nil {
		return nil, err
	}
	if group < 0 {
		return nil, fmt.Errorf("argument %q must be non-negative", "group")
	}
	if group > compiled.NumSubexp() {
		return nil, fmt.Errorf("capture group %d exceeds pattern capture count %d", group, compiled.NumSubexp())
	}
	indexes := compiled.FindStringSubmatchIndex(value)
	if len(indexes) == 0 || indexes[group*2] < 0 {
		return nil, nil
	}
	return value[indexes[group*2]:indexes[group*2+1]], nil
}

func builtinSubstring(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	start, err := optionalIntArgument(arguments, "start", 0)
	if err != nil {
		return nil, err
	}
	runes := []rune(value)
	end, err := optionalIntArgument(arguments, "end", len(runes))
	if err != nil {
		return nil, err
	}
	start = normalizeIndex(start, len(runes))
	end = normalizeIndex(end, len(runes))
	if end < start {
		end = start
	}
	return string(runes[start:end]), nil
}

func normalizeIndex(index, length int) int {
	if index < 0 {
		index = length + index
	}
	if index < 0 {
		return 0
	}
	if index > length {
		return length
	}
	return index
}

func builtinSplit(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	separator, err := requiredStringArgument(arguments, "separator")
	if err != nil {
		return nil, err
	}
	limit, err := optionalIntArgument(arguments, "limit", -1)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		return []string{}, nil
	}
	if separator == "" {
		runes := []rune(value)
		if limit > 0 && limit < len(runes) {
			runes = runes[:limit]
		}
		result := make([]string, len(runes))
		for index, item := range runes {
			result[index] = string(item)
		}
		return result, nil
	}
	parts := strings.Split(value, separator)
	if limit >= 0 && limit < len(parts) {
		parts = parts[:limit]
	}
	return parts, nil
}

func builtinJoin(input any, arguments map[string]json.RawMessage) (any, error) {
	separator, err := requiredStringArgument(arguments, "separator")
	if err != nil {
		return nil, err
	}
	switch values := input.(type) {
	case []string:
		return strings.Join(values, separator), nil
	case []any:
		stringsValue := make([]string, len(values))
		for index, value := range values {
			stringValue, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("join element %d is %T, not string", index, value)
			}
			stringsValue[index] = stringValue
		}
		return strings.Join(stringsValue, separator), nil
	default:
		return nil, fmt.Errorf("expected string[], got %T", input)
	}
}

func builtinParseInt(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	as, err := requiredStringArgument(arguments, "as")
	if err != nil {
		return nil, err
	}
	radix, err := optionalIntArgument(arguments, "radix", 10)
	if err != nil {
		return nil, err
	}
	if strings.Contains(value, "_") || value == "" {
		return nil, fmt.Errorf("invalid integer %q", value)
	}
	signedBits := map[string]int{"int": 64, "i8": 8, "i16": 16, "i32": 32, "i64": 64}
	unsignedBits := map[string]int{"u8": 8, "u16": 16, "u32": 32, "u64": 64}
	if bits, ok := signedBits[as]; ok {
		parsed, err := strconv.ParseInt(value, radix, bits)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", as, err)
		}
		switch as {
		case "i8":
			return int8(parsed), nil
		case "i16":
			return int16(parsed), nil
		case "i32":
			return int32(parsed), nil
		default:
			return parsed, nil
		}
	}
	if bits, ok := unsignedBits[as]; ok {
		if strings.HasPrefix(value, "-") {
			return nil, fmt.Errorf("unsigned integer rejects negative input")
		}
		parsed, err := strconv.ParseUint(strings.TrimPrefix(value, "+"), radix, bits)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", as, err)
		}
		switch as {
		case "u8":
			return uint8(parsed), nil
		case "u16":
			return uint16(parsed), nil
		case "u32":
			return uint32(parsed), nil
		default:
			return parsed, nil
		}
	}
	return nil, fmt.Errorf("unsupported integer target %q", as)
}

func builtinParseFloat(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	as, err := requiredStringArgument(arguments, "as")
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(value, "nan") || strings.Contains(strings.ToLower(value), "inf") || strings.Contains(strings.ToLower(value), "0x") || strings.TrimSpace(value) != value {
		return nil, fmt.Errorf("invalid finite decimal float %q", value)
	}
	bits := 64
	if as == "f32" {
		bits = 32
	} else if as != "float" && as != "f64" {
		return nil, fmt.Errorf("unsupported float target %q", as)
	}
	parsed, err := strconv.ParseFloat(value, bits)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, fmt.Errorf("parse %s: %w", as, err)
	}
	if as == "f32" {
		return float32(parsed), nil
	}
	return parsed, nil
}

func builtinParseBool(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	caseSensitive, err := optionalBoolArgument(arguments, "case-sensitive", false)
	if err != nil {
		return nil, err
	}
	trueValue := "true"
	falseValue := "false"
	trueValue, err = optionalStringArgument(arguments, "true", trueValue)
	if err != nil {
		return nil, err
	}
	falseValue, err = optionalStringArgument(arguments, "false", falseValue)
	if err != nil {
		return nil, err
	}
	compare := value
	if !caseSensitive {
		compare = strings.ToLower(compare)
		trueValue = strings.ToLower(trueValue)
		falseValue = strings.ToLower(falseValue)
	}
	if compare == trueValue {
		return true, nil
	}
	if compare == falseValue {
		return false, nil
	}
	return nil, fmt.Errorf("%q is neither configured true nor false value", value)
}

func builtinToString(input any) (any, error) {
	switch value := input.(type) {
	case string:
		return value, nil
	case bool:
		return strconv.FormatBool(value), nil
	case int:
		return strconv.Itoa(value), nil
	case int8:
		return strconv.FormatInt(int64(value), 10), nil
	case int16:
		return strconv.FormatInt(int64(value), 10), nil
	case int32:
		return strconv.FormatInt(int64(value), 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float32:
		if float32IsInvalid(value) {
			return nil, fmt.Errorf("float must be finite")
		}
		return strconv.FormatFloat(float64(value), 'g', -1, 32), nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("float must be finite")
		}
		return strconv.FormatFloat(value, 'g', -1, 64), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to string", input)
	}
}

func builtinURLResolve(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	baseValue, err := requiredStringArgument(arguments, "base")
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(baseValue)
	if err != nil || !base.IsAbs() {
		return nil, fmt.Errorf("invalid absolute base URL %q", baseValue)
	}
	reference, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid URL reference %q: %w", value, err)
	}
	return base.ResolveReference(reference).String(), nil
}

func builtinURLQuery(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	name, err := requiredStringArgument(arguments, "name")
	if err != nil {
		return nil, err
	}
	index, err := optionalIntArgument(arguments, "index", 0)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, fmt.Errorf("argument %q must be non-negative", "index")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", value, err)
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid URL query in %q: %w", value, err)
	}
	values := query[name]
	if index >= len(values) {
		return nil, nil
	}
	return values[index], nil
}

func builtinURLPath(input any) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", value, err)
	}
	path, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return nil, fmt.Errorf("decode URL path: %w", err)
	}
	return path, nil
}

func builtinPathSegment(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	index, err := requiredIntArgument(arguments, "index")
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid URL or path %q: %w", value, err)
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = value
	}
	rawSegments := strings.Split(path, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" {
			continue
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return nil, err
		}
		segments = append(segments, decoded)
	}
	if index < 0 {
		index = len(segments) + index
	}
	if index < 0 || index >= len(segments) {
		return nil, nil
	}
	return segments[index], nil
}

func builtinAssertMatches(input any, arguments map[string]json.RawMessage) (any, error) {
	value, err := requireString(input)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRuntimeRegex(arguments)
	if err != nil {
		return nil, err
	}
	if !compiled.MatchString(value) {
		return nil, fmt.Errorf("value does not match required pattern")
	}
	return input, nil
}

func builtinAssertEnum(input any, rawValues []json.RawMessage) (any, error) {
	for _, raw := range rawValues {
		value, err := decodeJSON(raw)
		if err != nil {
			return nil, err
		}
		if equalScalar(input, value) {
			return input, nil
		}
	}
	return nil, fmt.Errorf("value is outside the allowed enum")
}

func builtinAssertBound(input any, arguments map[string]json.RawMessage, minimum bool) (any, error) {
	bound, ok, err := namedArgument(arguments, "value")
	if err != nil || !ok {
		return nil, argumentError("numeric bound is required", err)
	}
	left, err := numericBigFloat(input)
	if err != nil {
		return nil, err
	}
	right, err := numericBigFloat(bound)
	if err != nil {
		return nil, fmt.Errorf("bound: %w", err)
	}
	comparison := left.Cmp(right)
	if minimum && comparison < 0 {
		return nil, fmt.Errorf("value is less than minimum")
	}
	if !minimum && comparison > 0 {
		return nil, fmt.Errorf("value is greater than maximum")
	}
	return input, nil
}

func numericBigFloat(value any) (*big.Float, error) {
	text, ok := numberString(value)
	if !ok {
		return nil, fmt.Errorf("expected number, got %T", value)
	}
	parsed, _, err := big.ParseFloat(text, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q: %w", text, err)
	}
	return parsed, nil
}

func argumentError(message string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%s", message)
	}
	return fmt.Errorf("%s: %w", message, cause)
}
