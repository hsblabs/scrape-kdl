package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/dom"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/kdl"
	"github.com/hsblabs/scrape-kdl/internal/source"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

type valueExpectation int

const (
	expectString valueExpectation = iota
	expectBool
	expectInt
	expectNonNegativeInt
	expectNumber
	expectScalar
)

var userNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var rootNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func stringProperty(n *kdl.Node, name string) (string, bool) {
	v, ok := n.Property(name)
	if !ok || v.Kind != kdl.ValueString {
		return "", false
	}
	s, ok := v.Value.(string)
	return s, ok
}

func boolProperty(n *kdl.Node, name string, def bool) (bool, bool) {
	v, ok := n.Property(name)
	if !ok {
		return def, true
	}
	b, ok := v.Value.(bool)
	return b, ok && v.Kind == kdl.ValueBool
}

func intProperty(n *kdl.Node, name string) (int, bool) {
	v, ok := n.Property(name)
	if !ok || v.Kind != kdl.ValueInt {
		return 0, false
	}
	i, ok := v.Value.(int64)
	return int(i), ok
}

func stringArg(n *kdl.Node, idx int) (string, bool) {
	if idx < 0 || idx >= len(n.Arguments) || n.Arguments[idx].Kind != kdl.ValueString {
		return "", false
	}
	s, ok := n.Arguments[idx].Value.(string)
	return s, ok
}

func floatArg(n *kdl.Node, idx int) (float64, bool) {
	if idx < 0 || idx >= len(n.Arguments) {
		return 0, false
	}
	switch n.Arguments[idx].Kind {
	case kdl.ValueInt:
		i, _ := n.Arguments[idx].Value.(int64)
		return float64(i), true
	case kdl.ValueFloat:
		f, _ := n.Arguments[idx].Value.(float64)
		return f, true
	default:
		return 0, false
	}
}

func rawJSON(v kdl.Value) (json.RawMessage, error) {
	return json.Marshal(v.Value)
}

func rawJSONPtr(v kdl.Value) (*json.RawMessage, error) {
	raw, err := rawJSON(v)
	if err != nil {
		return nil, err
	}
	return &raw, nil
}

func valueType(v kdl.Value) typesys.Type {
	switch v.Kind {
	case kdl.ValueString:
		return typesys.Primitive("string")
	case kdl.ValueBool:
		return typesys.Primitive("bool")
	case kdl.ValueInt:
		return typesys.Primitive("int")
	case kdl.ValueFloat:
		return typesys.Primitive("float")
	case kdl.ValueNull:
		return typesys.Nullable(typesys.Primitive("unknown"))
	default:
		return typesys.Primitive("unknown")
	}
}

func isValueAssignable(v kdl.Value, target typesys.Type) bool {
	if v.Kind == kdl.ValueNull {
		return typesys.IsNullable(target) || (target.Kind == typesys.KindPrimitive && target.Name == "unknown")
	}
	return typesys.IsAssignable(valueType(v), target)
}

func validateNode(diags *diagnostic.List, n *kdl.Node, minArgs, maxArgs int, allowedProps map[string]valueExpectation, path string) {
	if len(n.Arguments) < minArgs || (maxArgs >= 0 && len(n.Arguments) > maxArgs) {
		diags.Add("E_ARGUMENT_COUNT", diagnostic.SeverityError, fmt.Sprintf("node %q expects %d..%d positional arguments, got %d", n.Name, minArgs, maxArgs, len(n.Arguments)), n.Span, path)
	}
	seen := map[string]bool{}
	for _, p := range n.Properties {
		if seen[p.Name] {
			diags.Add("E_DUPLICATE_PROPERTY", diagnostic.SeverityError, fmt.Sprintf("duplicate property %q", p.Name), p.Span, path)
		}
		seen[p.Name] = true
		exp, ok := allowedProps[p.Name]
		if !ok {
			diags.Add("E_UNKNOWN_PROPERTY", diagnostic.SeverityError, fmt.Sprintf("property %q is not allowed on %q", p.Name, n.Name), p.Span, path)
			continue
		}
		if !matchesExpectation(p.Value, exp) {
			diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("property %q has incompatible value kind %s", p.Name, p.Value.Kind), p.Value.Span, path)
		}
	}
}

func matchesExpectation(v kdl.Value, exp valueExpectation) bool {
	switch exp {
	case expectString:
		return v.Kind == kdl.ValueString
	case expectBool:
		return v.Kind == kdl.ValueBool
	case expectInt:
		return v.Kind == kdl.ValueInt
	case expectNonNegativeInt:
		if v.Kind != kdl.ValueInt {
			return false
		}
		i, _ := v.Value.(int64)
		return i >= 0
	case expectNumber:
		return v.Kind == kdl.ValueInt || v.Kind == kdl.ValueFloat
	case expectScalar:
		return v.Kind == kdl.ValueString || v.Kind == kdl.ValueBool || v.Kind == kdl.ValueInt || v.Kind == kdl.ValueFloat || v.Kind == kdl.ValueNull
	default:
		return false
	}
}

func validateRequiredProperties(diags *diagnostic.List, n *kdl.Node, names []string, path string) {
	for _, name := range names {
		if _, ok := n.Property(name); !ok {
			diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, fmt.Sprintf("built-in %q requires property %q", firstStringArg(n), name), n.Span, path)
		}
	}
}

func firstStringArg(n *kdl.Node) string { s, _ := stringArg(n, 0); return s }

func validateIdentifier(diags *diagnostic.List, name string, span source.Span, root bool, path string) {
	pattern := userNamePattern
	if root {
		pattern = rootNamePattern
	}
	if !pattern.MatchString(name) {
		diags.Add("E_IDENTIFIER_INVALID", diagnostic.SeverityError, fmt.Sprintf("invalid identifier %q", name), span, path)
	}
}

func validateSelector(selector string) error {
	if strings.TrimSpace(selector) == "" {
		return fmt.Errorf("selector must not be empty")
	}
	_, err := dom.ParseSelector(selector)
	return err
}

func absClean(path string) string  { p, _ := filepath.Abs(path); return filepath.Clean(p) }
func sha256Hex(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func sortedSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
