package executor

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func resolveInputs(definitions []ir.Input, provided map[string]any) (map[string]any, error) {
	known := make(map[string]ir.Input, len(definitions))
	for _, definition := range definitions {
		known[definition.Name] = definition
	}
	unknown := ""
	for name := range provided {
		if _, ok := known[name]; !ok && (unknown == "" || name < unknown) {
			unknown = name
		}
	}
	if unknown != "" {
		return nil, &ExecutionError{Code: "E_INPUT_UNKNOWN", Message: fmt.Sprintf("unknown input %q", unknown), Path: "input." + unknown}
	}
	resolved := make(map[string]any, len(definitions))
	for _, definition := range definitions {
		value, ok := provided[definition.Name]
		if !ok && definition.Default != nil {
			decoded, err := decodeJSON(*definition.Default)
			if err != nil {
				return nil, &ExecutionError{Code: "E_INPUT_DEFAULT", Message: err.Error(), Path: "input." + definition.Name, Cause: err}
			}
			value, ok = decoded, true
		}
		if !ok {
			if definition.Required {
				return nil, &ExecutionError{Code: "E_INPUT_REQUIRED", Message: fmt.Sprintf("required input %q is missing", definition.Name), Path: "input." + definition.Name}
			}
			continue
		}
		normalized, err := normalizeInput(definition.Type, value)
		if err != nil {
			return nil, &ExecutionError{Code: "E_INPUT_TYPE", Message: err.Error(), Path: "input." + definition.Name, Cause: err}
		}
		resolved[definition.Name] = normalized
	}
	return resolved, nil
}

func normalizeInput(typeName string, value any) (any, error) {
	switch typeName {
	case "string":
		stringValue, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}
		return stringValue, nil
	case "bool":
		boolValue, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", value)
		}
		return boolValue, nil
	case "int":
		switch typed := value.(type) {
		case int:
			return int64(typed), nil
		case int64:
			return typed, nil
		case json.Number:
			parsed, err := typed.Int64()
			if err != nil {
				return nil, fmt.Errorf("expected integer: %w", err)
			}
			return parsed, nil
		case float64:
			if math.Trunc(typed) != typed || typed < math.MinInt64 || typed >= math.MaxInt64 {
				return nil, fmt.Errorf("expected finite integer, got %v", typed)
			}
			return int64(typed), nil
		default:
			return nil, fmt.Errorf("expected integer, got %T", value)
		}
	case "float":
		var result float64
		switch typed := value.(type) {
		case float64:
			result = typed
		case float32:
			result = float64(typed)
		case int:
			result = float64(typed)
		case int64:
			result = float64(typed)
		case json.Number:
			parsed, err := typed.Float64()
			if err != nil {
				return nil, fmt.Errorf("expected float: %w", err)
			}
			result = parsed
		default:
			return nil, fmt.Errorf("expected float, got %T", value)
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return nil, fmt.Errorf("float must be finite")
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported input type %q", typeName)
	}
}

func inputString(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", fmt.Errorf("float must be finite")
		}
		return strconv.FormatFloat(typed, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported template value %T", value)
	}
}
