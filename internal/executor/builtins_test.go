package executor

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

func testCall(named map[string]any, positional ...any) ir.TransformCall {
	call := ir.TransformCall{}
	for _, value := range positional {
		raw, _ := json.Marshal(value)
		call.PositionalArguments = append(call.PositionalArguments, raw)
	}
	for name, value := range named {
		raw, _ := json.Marshal(value)
		call.NamedArguments = append(call.NamedArguments, ir.NamedArgument{Name: name, Value: raw})
	}
	return call
}

func typedTestCall(output typesys.Type, named map[string]any) ir.TransformCall {
	call := testCall(named)
	call.Output = output
	return call
}

func TestBuiltins(t *testing.T) {
	tests := []struct {
		name  string
		input any
		call  ir.TransformCall
		want  any
	}{
		{"trim", "  a \n", testCall(nil), "a"},
		{"normalize-whitespace", " a\t b\n c ", testCall(nil), "a b c"},
		{"lowercase", "ÄBC", testCall(nil), "äbc"},
		{"uppercase", "äbc", testCall(nil), "ÄBC"},
		{"replace", "a-a-a", testCall(map[string]any{"old": "a", "new": "x", "count": 2}), "x-x-a"},
		{"regex-replace", "a1b2", testCall(map[string]any{"pattern": "([0-9])", "replacement": "<$1>", "count": 1}), "a<1>b2"},
		{"regex-capture", "/horse/abc", testCall(map[string]any{"pattern": "/horse/([^/]+)", "group": 1}), "abc"},
		{"regex-capture", "none", testCall(map[string]any{"pattern": "x"}), nil},
		{"substring", "あいうえ", testCall(map[string]any{"start": -3, "end": -1}), "いう"},
		{"split", "a,b,c", testCall(map[string]any{"separator": ",", "limit": 2}), []string{"a", "b"}},
		{"split", "あい", testCall(map[string]any{"separator": ""}), []string{"あ", "い"}},
		{"join", []string{"a", "b"}, testCall(map[string]any{"separator": "-"}), "a-b"},
		{"prepend", "b", testCall(map[string]any{"value": "a"}), "ab"},
		{"append", "a", testCall(map[string]any{"value": "b"}), "ab"},
		{"parse-int", "255", testCall(map[string]any{"as": "u8"}), uint8(255)},
		{"parse-int", "-12", testCall(map[string]any{"as": "i16"}), int16(-12)},
		{"parse-float", "1.25", testCall(map[string]any{"as": "f64"}), float64(1.25)},
		{"parse-bool", "YES", testCall(map[string]any{"true": "yes", "false": "no"}), true},
		{"to-string", int32(-4), testCall(nil), "-4"},
		{"empty-to-null", "", testCall(nil), nil},
		{"coalesce", nil, typedTestCall(typesys.Primitive("string"), map[string]any{"value": "fallback"}), "fallback"},
		{"url-resolve", "../x", testCall(map[string]any{"base": "https://example.com/a/b"}), "https://example.com/x"},
		{"url-query", "https://e.test/?a=x&a=y", testCall(map[string]any{"name": "a", "index": 1}), "y"},
		{"url-path", "https://e.test/a%20b", testCall(nil), "/a b"},
		{"path-segment", "/a/b/c/", testCall(map[string]any{"index": -1}), "c"},
		{"assert-matches", "abc", testCall(map[string]any{"pattern": "^a"}), "abc"},
		{"assert-enum", "b", testCall(nil, "a", "b"), "b"},
		{"assert-min", int64(5), testCall(map[string]any{"value": 4}), int64(5)},
		{"assert-max", float64(5), testCall(map[string]any{"value": 6}), float64(5)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime(test.name, test.input, test.call)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v (%T), want %#v (%T)", got, got, test.want, test.want)
			}
		})
	}
}

func TestBuiltinJoinRepresentations(t *testing.T) {
	call := testCall(map[string]any{"separator": "-"})
	for _, tt := range []struct {
		name  string
		input any
		want  string
	}{
		{name: "string slice", input: []string{"a", "b"}, want: "a-b"},
		{name: "JSON array", input: []any{"a", "b"}, want: "a-b"},
		{name: "empty", input: []any{}, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime("join", tt.input, call)
			if err != nil || got != tt.want {
				t.Fatalf("join = %#v, %v", got, err)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		input any
		call  ir.TransformCall
	}{
		{name: "non-string element", input: []any{"a", 2}, call: call},
		{name: "wrong input", input: "a", call: call},
		{name: "missing separator", input: []string{"a"}, call: testCall(nil)},
		{name: "invalid separator", input: []string{"a"}, call: testCall(map[string]any{"separator": 1})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime("join", tt.input, tt.call); err == nil {
				t.Fatal("join succeeded")
			}
		})
	}
}

func TestBuiltinToStringNumericBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input any
		want  string
	}{
		{name: "bool", input: true, want: "true"},
		{name: "int", input: int(-1), want: "-1"},
		{name: "int8", input: int8(-8), want: "-8"},
		{name: "int16", input: int16(-16), want: "-16"},
		{name: "int64", input: int64(-64), want: "-64"},
		{name: "uint", input: uint(1), want: "1"},
		{name: "uint16", input: uint16(16), want: "16"},
		{name: "uint64", input: uint64(math.MaxUint64), want: "18446744073709551615"},
		{name: "float32", input: float32(1.25), want: "1.25"},
		{name: "float64", input: 1.25, want: "1.25"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinToString(tt.input)
			if err != nil || got != tt.want {
				t.Fatalf("to-string = %#v, %v", got, err)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		input any
	}{
		{name: "float32 NaN", input: float32(math.NaN())},
		{name: "float32 infinity", input: float32(math.Inf(1))},
		{name: "float64 NaN", input: math.NaN()},
		{name: "float64 infinity", input: math.Inf(-1)},
		{name: "array", input: []any{}},
		{name: "nil", input: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := builtinToString(tt.input); err == nil {
				t.Fatal("to-string succeeded")
			}
		})
	}
}

func TestBuiltinFailures(t *testing.T) {
	tests := []struct {
		name  string
		input any
		call  ir.TransformCall
	}{
		{"parse-int", "256", testCall(map[string]any{"as": "u8"})},
		{"parse-float", "NaN", testCall(map[string]any{"as": "f64"})},
		{"parse-bool", "maybe", testCall(nil)},
		{"assert-matches", "abc", testCall(map[string]any{"pattern": "^z"})},
		{"assert-enum", "c", testCall(nil, "a", "b")},
		{"assert-min", int64(2), testCall(map[string]any{"value": 3})},
		{"assert-max", int64(4), testCall(map[string]any{"value": 3})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime(test.name, test.input, test.call); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
