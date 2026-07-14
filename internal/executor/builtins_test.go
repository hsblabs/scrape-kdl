package executor

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
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

func TestBuiltinNumericParsingBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		call  ir.TransformCall
		want  any
	}{
		{name: "signed plus", input: "+42", call: testCall(map[string]any{"as": "int"}), want: int64(42)},
		{name: "signed minimum", input: "-128", call: testCall(map[string]any{"as": "i8"}), want: int8(-128)},
		{name: "unsigned maximum", input: "18446744073709551615", call: testCall(map[string]any{"as": "u64"}), want: uint64(math.MaxUint64)},
		{name: "radix", input: "ff", call: testCall(map[string]any{"as": "u16", "radix": 16}), want: uint16(255)},
	} {
		t.Run("parse-int "+tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime("parse-int", tt.input, tt.call)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parse-int = %#v, %v", got, err)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		input string
		call  ir.TransformCall
	}{
		{name: "empty", input: "", call: testCall(map[string]any{"as": "int"})},
		{name: "underscore", input: "1_000", call: testCall(map[string]any{"as": "int"})},
		{name: "negative unsigned", input: "-1", call: testCall(map[string]any{"as": "u8"})},
		{name: "overflow", input: "128", call: testCall(map[string]any{"as": "i8"})},
		{name: "invalid radix", input: "1", call: testCall(map[string]any{"as": "int", "radix": 1})},
		{name: "unsupported target", input: "1", call: testCall(map[string]any{"as": "i128"})},
		{name: "whitespace", input: " 1", call: testCall(map[string]any{"as": "int"})},
	} {
		t.Run("parse-int "+tt.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime("parse-int", tt.input, tt.call); err == nil {
				t.Fatal("parse-int succeeded")
			}
		})
	}

	for _, tt := range []struct {
		name  string
		input string
		call  ir.TransformCall
		want  any
	}{
		{name: "exponent", input: "1.25e2", call: testCall(map[string]any{"as": "f64"}), want: float64(125)},
		{name: "float alias", input: "-0.5", call: testCall(map[string]any{"as": "float"}), want: float64(-0.5)},
		{name: "f32", input: "1.25", call: testCall(map[string]any{"as": "f32"}), want: float32(1.25)},
	} {
		t.Run("parse-float "+tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime("parse-float", tt.input, tt.call)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parse-float = %#v, %v", got, err)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		input string
		as    string
	}{
		{name: "whitespace", input: " 1", as: "f64"},
		{name: "hex", input: "0x1p2", as: "f64"},
		{name: "infinity", input: "Inf", as: "f64"},
		{name: "overflow", input: "1e1000", as: "f64"},
		{name: "trailing", input: "1x", as: "f64"},
		{name: "unsupported target", input: "1", as: "f16"},
	} {
		t.Run("parse-float "+tt.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime("parse-float", tt.input, testCall(map[string]any{"as": tt.as})); err == nil {
				t.Fatal("parse-float succeeded")
			}
		})
	}
}

func TestBuiltinNumericMalformedIRBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		call  ir.TransformCall
		want  any
	}{
		{name: "signed i32", input: "-2147483648", call: testCall(map[string]any{"as": "i32"}), want: int32(math.MinInt32)},
		{name: "unsigned u32", input: "4294967295", call: testCall(map[string]any{"as": "u32"}), want: uint32(math.MaxUint32)},
		{name: "minimum radix", input: "101", call: testCall(map[string]any{"as": "u8", "radix": 2}), want: uint8(5)},
		{name: "maximum radix", input: "z", call: testCall(map[string]any{"as": "u8", "radix": 36}), want: uint8(35)},
	} {
		t.Run("parse-int success/"+tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime("parse-int", tt.input, tt.call)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parse-int = %#v, %v", got, err)
			}
		})
	}

	malformedNamed := func(name string, raw string) ir.TransformCall {
		return ir.TransformCall{NamedArguments: []ir.NamedArgument{{Name: name, Value: json.RawMessage(raw)}}}
	}
	for _, tt := range []struct {
		name    string
		builtin string
		input   any
		call    ir.TransformCall
	}{
		{name: "integer wrong input", builtin: "parse-int", input: int64(1), call: testCall(map[string]any{"as": "int"})},
		{name: "integer missing target", builtin: "parse-int", input: "1", call: testCall(nil)},
		{name: "integer malformed target", builtin: "parse-int", input: "1", call: malformedNamed("as", `not-json`)},
		{name: "integer non-string target", builtin: "parse-int", input: "1", call: testCall(map[string]any{"as": 1})},
		{name: "integer malformed radix", builtin: "parse-int", input: "1", call: ir.TransformCall{NamedArguments: []ir.NamedArgument{
			{Name: "as", Value: json.RawMessage(`"int"`)},
			{Name: "radix", Value: json.RawMessage(`not-json`)},
		}}},
		{name: "integer fractional radix", builtin: "parse-int", input: "1", call: testCall(map[string]any{"as": "int", "radix": 10.5})},
		{name: "integer radix above maximum", builtin: "parse-int", input: "1", call: testCall(map[string]any{"as": "int", "radix": 37})},
		{name: "float wrong input", builtin: "parse-float", input: int64(1), call: testCall(map[string]any{"as": "float"})},
		{name: "float missing target", builtin: "parse-float", input: "1", call: testCall(nil)},
		{name: "float malformed target", builtin: "parse-float", input: "1", call: malformedNamed("as", `not-json`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime(tt.builtin, tt.input, tt.call); err == nil {
				t.Fatalf("%s succeeded", tt.builtin)
			}
		})
	}

	maximum := testCall(map[string]any{"value": uint64(math.MaxUint64)})
	if got, err := applyBuiltinRuntime("assert-min", uint64(math.MaxUint64), maximum); err != nil || got != uint64(math.MaxUint64) {
		t.Fatalf("assert-min maximum uint64 = %#v, %v", got, err)
	}
	for _, tt := range []struct {
		name  string
		input any
		call  ir.TransformCall
	}{
		{name: "non-numeric input", input: "1", call: testCall(map[string]any{"value": 1})},
		{name: "positive infinity input", input: math.Inf(1), call: testCall(map[string]any{"value": 1})},
		{name: "negative infinity input", input: math.Inf(-1), call: testCall(map[string]any{"value": 1})},
		{name: "NaN input", input: math.NaN(), call: testCall(map[string]any{"value": 1})},
		{name: "non-numeric bound", input: int64(1), call: testCall(map[string]any{"value": "1"})},
		{name: "malformed bound", input: int64(1), call: malformedNamed("value", `not-json`)},
	} {
		t.Run("assert-min failure/"+tt.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime("assert-min", tt.input, tt.call); err == nil {
				t.Fatal("assert-min succeeded")
			}
		})
	}
	if _, err := numericBigFloat(json.Number("not-a-number")); err == nil {
		t.Fatal("numericBigFloat accepted an invalid JSON number")
	}
}

func TestBuiltinBooleanParsingConfiguration(t *testing.T) {
	call := testCall(map[string]any{"true": "yes", "false": "no"})
	for input, want := range map[string]bool{"YES": true, "no": false} {
		got, err := applyBuiltinRuntime("parse-bool", input, call)
		if err != nil || got != want {
			t.Fatalf("parse-bool(%q) = %#v, %v", input, got, err)
		}
	}
	caseSensitive := testCall(map[string]any{"true": "yes", "false": "no", "case-sensitive": true})
	if _, err := applyBuiltinRuntime("parse-bool", "YES", caseSensitive); err == nil {
		t.Fatal("case-sensitive parse-bool succeeded")
	}
	invalidFlag := testCall(map[string]any{"case-sensitive": "true"})
	if _, err := applyBuiltinRuntime("parse-bool", "true", invalidFlag); err == nil {
		t.Fatal("parse-bool accepted non-boolean case-sensitive")
	}
	for _, tt := range []struct {
		name string
		call ir.TransformCall
	}{
		{name: "true value", call: testCall(map[string]any{"true": true})},
		{name: "false value", call: testCall(map[string]any{"false": false})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := applyBuiltinRuntime("parse-bool", "", tt.call); err == nil {
				t.Fatal("parse-bool accepted a non-string configured value")
			}
		})
	}
}

func TestBuiltinRegexCaptureGroupBounds(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pattern string
		group   int
		want    any
		wantErr bool
	}{
		{name: "whole match", input: "a", pattern: "(a)", group: 0, want: "a"},
		{name: "capture", input: "a", pattern: "(a)", group: 1, want: "a"},
		{name: "capture did not participate", input: "b", pattern: "(a)?b", group: 1},
		{name: "group exceeds pattern", input: "a", pattern: "(a)", group: 2, wantErr: true},
		{name: "negative", input: "a", pattern: "(a)", group: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime("regex-capture", tt.input, testCall(map[string]any{"pattern": tt.pattern, "group": tt.group}))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("regex-capture = %#v, want error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("regex-capture = %#v, %v; want %#v", got, err, tt.want)
			}
		})
	}
}

func TestBuiltinReplacementAndRegexBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		builtin string
		input   string
		call    ir.TransformCall
		want    any
		wantErr bool
	}{
		{name: "literal all", builtin: "replace", input: "a-a-a", call: testCall(map[string]any{"old": "a", "new": "x"}), want: "x-x-x"},
		{name: "literal zero", builtin: "replace", input: "a-a", call: testCall(map[string]any{"old": "a", "new": "x", "count": 0}), want: "a-a"},
		{name: "literal empty unicode", builtin: "replace", input: "é", call: testCall(map[string]any{"old": "", "new": "x", "count": 2}), want: "xéx"},
		{name: "literal negative count", builtin: "replace", input: "a", call: testCall(map[string]any{"old": "a", "new": "x", "count": -1}), wantErr: true},
		{name: "regex flags", builtin: "regex-replace", input: "A\nb", call: testCall(map[string]any{"pattern": "^a.b$", "replacement": "match", "flags": "ims"}), want: "match"},
		{name: "regex captures and dollar", builtin: "regex-replace", input: "a1", call: testCall(map[string]any{"pattern": "([a-z])([0-9])", "replacement": "$2$1$$"}), want: "1a$"},
		{name: "regex zero", builtin: "regex-replace", input: "aaa", call: testCall(map[string]any{"pattern": "a", "replacement": "x", "count": 0}), want: "aaa"},
		{name: "regex limited empty matches", builtin: "regex-replace", input: "ab", call: testCall(map[string]any{"pattern": "", "replacement": "x", "count": 2}), want: "xaxb"},
		{name: "regex negative count", builtin: "regex-replace", input: "a", call: testCall(map[string]any{"pattern": "a", "replacement": "x", "count": -1}), wantErr: true},
		{name: "regex duplicate flag", builtin: "regex-replace", input: "a", call: testCall(map[string]any{"pattern": "a", "replacement": "x", "flags": "ii"}), wantErr: true},
		{name: "regex unsupported flag", builtin: "regex-replace", input: "a", call: testCall(map[string]any{"pattern": "a", "replacement": "x", "flags": "x"}), wantErr: true},
		{name: "regex named capture", builtin: "regex-capture", input: "a", call: testCall(map[string]any{"pattern": "(?P<value>a)"}), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime(tt.builtin, tt.input, tt.call)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("result = %#v, want error", got)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("result = %#v, error = %v, want %#v", got, err, tt.want)
			}
		})
	}

	for _, builtin := range []string{"replace", "regex-replace"} {
		t.Run(builtin+" malformed count", func(t *testing.T) {
			call := testCall(map[string]any{"old": "a", "new": "x", "pattern": "a", "replacement": "x"})
			call.NamedArguments = append(call.NamedArguments, ir.NamedArgument{Name: "count", Value: json.RawMessage(`not-json`)})
			_, err := applyBuiltinRuntime(builtin, "a", call)
			if err == nil || !strings.Contains(err.Error(), "decode IR literal") {
				t.Fatalf("malformed count error = %#v", err)
			}
		})
	}
}

func TestBuiltinSubstringAndSplitBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		builtin string
		input   string
		call    ir.TransformCall
		want    any
		wantErr bool
	}{
		{name: "substring omitted end", builtin: "substring", input: "あいう", call: testCall(map[string]any{"start": 1}), want: "いう"},
		{name: "substring negative clamp", builtin: "substring", input: "abc", call: testCall(map[string]any{"start": -99}), want: "abc"},
		{name: "substring upper clamp", builtin: "substring", input: "abc", call: testCall(map[string]any{"start": 99}), want: ""},
		{name: "substring reversed", builtin: "substring", input: "abc", call: testCall(map[string]any{"start": 2, "end": 1}), want: ""},
		{name: "substring missing start", builtin: "substring", input: "abc", call: testCall(nil), wantErr: true},
		{name: "substring fractional start", builtin: "substring", input: "abc", call: testCall(map[string]any{"start": 1.5}), wantErr: true},
		{name: "split unlimited", builtin: "split", input: "a,b,c", call: testCall(map[string]any{"separator": ","}), want: []string{"a", "b", "c"}},
		{name: "split zero", builtin: "split", input: "a,b", call: testCall(map[string]any{"separator": ",", "limit": 0}), want: []string{}},
		{name: "split one", builtin: "split", input: "a,b", call: testCall(map[string]any{"separator": ",", "limit": 1}), want: []string{"a"}},
		{name: "split Unicode scalars", builtin: "split", input: "あい", call: testCall(map[string]any{"separator": "", "limit": 1}), want: []string{"あ"}},
		{name: "split empty input", builtin: "split", input: "", call: testCall(map[string]any{"separator": ""}), want: []string{}},
		{name: "split negative limit", builtin: "split", input: "a,b", call: testCall(map[string]any{"separator": ",", "limit": -1}), wantErr: true},
		{name: "split missing separator", builtin: "split", input: "a,b", call: testCall(nil), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime(tt.builtin, tt.input, tt.call)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("result = %#v, want error", got)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("result = %#v, error = %v, want %#v", got, err, tt.want)
			}
		})
	}

	for _, name := range []string{"start", "limit"} {
		t.Run("malformed "+name, func(t *testing.T) {
			builtin := "substring"
			call := ir.TransformCall{NamedArguments: []ir.NamedArgument{{Name: name, Value: json.RawMessage(`not-json`)}}}
			if name == "limit" {
				builtin = "split"
				call.NamedArguments = append(call.NamedArguments, ir.NamedArgument{Name: "separator", Value: json.RawMessage(`","`)})
			}
			_, err := applyBuiltinRuntime(builtin, "a,b", call)
			if err == nil || !strings.Contains(err.Error(), "decode IR literal") {
				t.Fatalf("malformed %s error = %#v", name, err)
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

func TestBuiltinRequiredArgumentErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		call    ir.TransformCall
		message string
	}{
		{name: "coalesce", input: nil, call: typedTestCall(typesys.Primitive("string"), nil), message: "coalesce requires value"},
		{name: "assert-min", input: int64(1), call: testCall(nil), message: "numeric bound is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/missing", func(t *testing.T) {
			_, err := applyBuiltinRuntime(tt.name, tt.input, tt.call)
			if err == nil || err.Error() != tt.message || errors.Unwrap(err) != nil {
				t.Fatalf("missing argument error = %#v", err)
			}
		})
		t.Run(tt.name+"/invalid JSON", func(t *testing.T) {
			call := tt.call
			call.NamedArguments = []ir.NamedArgument{{Name: "value", Value: json.RawMessage(`not-json`)}}
			_, err := applyBuiltinRuntime(tt.name, tt.input, call)
			if err == nil || !strings.Contains(err.Error(), tt.message) || !strings.Contains(err.Error(), "decode IR literal") || errors.Unwrap(err) == nil {
				t.Fatalf("invalid argument error = %#v", err)
			}
		})
	}
}

func TestBuiltinURLBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		builtin string
		input   any
		call    ir.TransformCall
		want    any
		wantErr bool
	}{
		{name: "resolve absolute", builtin: "url-resolve", input: "../item?q=1#part", call: testCall(map[string]any{"base": "https://example.invalid/a/b"}), want: "https://example.invalid/item?q=1#part"},
		{name: "resolve relative base", builtin: "url-resolve", input: "item", call: testCall(map[string]any{"base": "/root/"}), wantErr: true},
		{name: "resolve malformed reference", builtin: "url-resolve", input: "%zz", call: testCall(map[string]any{"base": "https://example.invalid/"}), wantErr: true},
		{name: "query decoded", builtin: "url-query", input: "https://example.invalid/?q=a+b", call: testCall(map[string]any{"name": "q"}), want: "a b"},
		{name: "query empty", builtin: "url-query", input: "https://example.invalid/?q=", call: testCall(map[string]any{"name": "q"}), want: ""},
		{name: "query missing", builtin: "url-query", input: "https://example.invalid/", call: testCall(map[string]any{"name": "q"}), want: nil},
		{name: "query out of range", builtin: "url-query", input: "https://example.invalid/?q=one", call: testCall(map[string]any{"name": "q", "index": 1}), want: nil},
		{name: "query negative index", builtin: "url-query", input: "https://example.invalid/?q=one", call: testCall(map[string]any{"name": "q", "index": -1}), wantErr: true},
		{name: "query malformed URL", builtin: "url-query", input: "https://example.invalid/?q=%zz", call: testCall(map[string]any{"name": "q"}), wantErr: true},
		{name: "decoded path", builtin: "url-path", input: "https://example.invalid/a%20b/%2F", call: testCall(nil), want: "/a b//"},
		{name: "malformed path", builtin: "url-path", input: "https://example.invalid/%zz", call: testCall(nil), wantErr: true},
		{name: "absolute segment", builtin: "path-segment", input: "https://example.invalid/a//b/", call: testCall(map[string]any{"index": 1}), want: "b"},
		{name: "decoded segment", builtin: "path-segment", input: "/a%2Fb/c", call: testCall(map[string]any{"index": 0}), want: "a/b"},
		{name: "negative segment", builtin: "path-segment", input: "/a/b", call: testCall(map[string]any{"index": -2}), want: "a"},
		{name: "segment out of range", builtin: "path-segment", input: "/a/b", call: testCall(map[string]any{"index": 2}), want: nil},
		{name: "segment missing index", builtin: "path-segment", input: "/a/b", call: testCall(nil), wantErr: true},
		{name: "segment malformed escape", builtin: "path-segment", input: "/a/%zz", call: testCall(map[string]any{"index": 1}), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBuiltinRuntime(tt.builtin, tt.input, tt.call)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("result = %#v, want error", got)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("result = %#v, error = %v, want %#v", got, err, tt.want)
			}
		})
	}
}

func FuzzBuiltinMalformedArgumentsDoNotPanic(f *testing.F) {
	f.Add(uint8(0), "a", []byte(`-1`))
	f.Add(uint8(1), "a", []byte(`"(a)"`))
	f.Add(uint8(5), "", []byte(`true`))
	f.Add(uint8(8), "1", []byte(`null`))
	f.Fuzz(func(t *testing.T, selector uint8, input string, raw []byte) {
		if len(input) > 256 || len(raw) > 256 {
			t.Skip()
		}
		argument := func(name string) ir.NamedArgument {
			return ir.NamedArgument{Name: name, Value: append(json.RawMessage(nil), raw...)}
		}
		var name string
		var call ir.TransformCall
		switch selector % 10 {
		case 0:
			name = "regex-capture"
			call = testCall(map[string]any{"pattern": "(a*)"})
			call.NamedArguments = append(call.NamedArguments, argument("group"))
		case 1:
			name = "regex-capture"
			call = testCall(map[string]any{"group": 0})
			call.NamedArguments = append(call.NamedArguments, argument("pattern"))
		case 2:
			name = "regex-replace"
			call = testCall(map[string]any{"pattern": "a", "replacement": "x"})
			call.NamedArguments = append(call.NamedArguments, argument("count"))
		case 3:
			name = "substring"
			call.NamedArguments = append(call.NamedArguments, argument("start"))
		case 4:
			name = "split"
			call = testCall(map[string]any{"separator": ""})
			call.NamedArguments = append(call.NamedArguments, argument("limit"))
		case 5:
			name = "parse-bool"
			call.NamedArguments = append(call.NamedArguments, argument("true"))
		case 6:
			name = "url-query"
			call = testCall(map[string]any{"name": "value"})
			call.NamedArguments = append(call.NamedArguments, argument("index"))
		case 7:
			name = "path-segment"
			call.NamedArguments = append(call.NamedArguments, argument("index"))
		case 8:
			name = "assert-min"
			call.NamedArguments = append(call.NamedArguments, argument("value"))
		case 9:
			name = "coalesce"
			call.Output = typesys.Primitive("unknown")
			call.NamedArguments = append(call.NamedArguments, argument("value"))
		}
		_, _ = applyBuiltinRuntime(name, input, call)
	})
}
