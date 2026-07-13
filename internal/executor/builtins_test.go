package executor

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
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
		{"coalesce", nil, testCall(map[string]any{"value": "fallback"}), "fallback"},
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
