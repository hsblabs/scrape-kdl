// Package clisupport holds the CLI contract logic shared by the core CLI and
// adapter CLIs: typed runtime-input parsing, the session-file JSON schema,
// and rejection of plaintext secret flags. Keeping it in one place keeps the
// documented parity between the binaries compiler-enforced.
package clisupport

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// RepeatedFlag collects repeatable name=value flag occurrences.
type RepeatedFlag []string

func (values *RepeatedFlag) String() string         { return strings.Join(*values, ",") }
func (values *RepeatedFlag) Set(value string) error { *values = append(*values, value); return nil }

// PlaintextSecretFlagMessage explains why --header and --cookie are rejected.
const PlaintextSecretFlagMessage = "--header and --cookie were removed; put secrets in --session-file FILE or use --session-file -"

// HasPlaintextSecretFlag reports whether args contain a rejected plaintext
// secret flag spelling.
func HasPlaintextSecretFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--header" || strings.HasPrefix(arg, "--header=") || arg == "--cookie" || strings.HasPrefix(arg, "--cookie=") {
			return true
		}
	}
	return false
}

// InputDeclaration mirrors an input declaration from the Validated IR JSON
// contract.
type InputDeclaration struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// ParseRuntimeInputs converts repeated name=value flag values into typed
// runtime inputs according to the extractor's declarations.
func ParseRuntimeInputs(declarations []InputDeclaration, values []string) (map[string]any, error) {
	byName := make(map[string]InputDeclaration, len(declarations))
	for _, declaration := range declarations {
		byName[declaration.Name] = declaration
	}
	result := make(map[string]any, len(values))
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --input %q; expected name=value", raw)
		}
		declaration, exists := byName[name]
		if !exists {
			return nil, fmt.Errorf("unknown input %q", name)
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("duplicate input %q", name)
		}
		parsed, err := ParseInputValue(declaration.Type, value)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", name, err)
		}
		result[name] = parsed
	}
	return result, nil
}

// ParseInputValue converts one flag value to the declared input type.
func ParseInputValue(typeName, value string) (any, error) {
	switch typeName {
	case "string":
		return value, nil
	case "bool":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("expected bool: %w", err)
		}
		return parsed, nil
	case "int":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer: %w", err)
		}
		return parsed, nil
	case "float":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("expected finite float")
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported input type %q", typeName)
	}
}

type sessionDocument struct {
	Headers map[string][]string `json:"headers"`
	Cookies []sessionCookie     `json:"cookies"`
}

type sessionCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DecodeSessionDocument reads one strict session JSON document
// ({"headers": {...}, "cookies": [...]}) and returns its request state.
func DecodeSessionDocument(reader io.Reader) (http.Header, []*http.Cookie, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var document sessionDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, nil, fmt.Errorf("decode JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, nil, fmt.Errorf("decode JSON: multiple values")
		}
		return nil, nil, fmt.Errorf("decode JSON: %w", err)
	}
	headers := make(http.Header)
	for name, values := range document.Headers {
		if strings.TrimSpace(name) == "" {
			return nil, nil, fmt.Errorf("header name must be non-empty")
		}
		for _, value := range values {
			headers.Add(name, value)
		}
	}
	var cookies []*http.Cookie
	for _, cookie := range document.Cookies {
		if strings.TrimSpace(cookie.Name) == "" {
			return nil, nil, fmt.Errorf("cookie name must be non-empty")
		}
		cookies = append(cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return headers, cookies, nil
}
