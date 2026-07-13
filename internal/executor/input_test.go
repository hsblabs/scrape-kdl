package executor

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func TestNormalizeInputRepresentations(t *testing.T) {
	maxFloatBelowInt64 := math.Nextafter(float64(math.MaxInt64), math.Inf(-1))
	tests := []struct {
		name     string
		typeName string
		value    any
		want     any
	}{
		{name: "string", typeName: "string", value: "value", want: "value"},
		{name: "bool", typeName: "bool", value: true, want: true},
		{name: "int", typeName: "int", value: int(-1), want: int64(-1)},
		{name: "int64", typeName: "int", value: int64(2), want: int64(2)},
		{name: "JSON integer", typeName: "int", value: json.Number("9223372036854775807"), want: int64(math.MaxInt64)},
		{name: "float integer", typeName: "int", value: float64(42), want: int64(42)},
		{name: "maximum representable float below int64 limit", typeName: "int", value: maxFloatBelowInt64, want: int64(maxFloatBelowInt64)},
		{name: "minimum int64 float", typeName: "int", value: float64(math.MinInt64), want: int64(math.MinInt64)},
		{name: "float64", typeName: "float", value: 1.25, want: 1.25},
		{name: "float32", typeName: "float", value: float32(1.25), want: float64(1.25)},
		{name: "float from int", typeName: "float", value: int(2), want: float64(2)},
		{name: "float from int64", typeName: "float", value: int64(-2), want: float64(-2)},
		{name: "JSON float", typeName: "float", value: json.Number("1.5"), want: 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeInput(tt.typeName, tt.value)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeInput(%q, %#v) = %#v, %v; want %#v", tt.typeName, tt.value, got, err, tt.want)
			}
		})
	}
}

func TestNormalizeInputRejectsInvalidValues(t *testing.T) {
	for _, tt := range []struct {
		name     string
		typeName string
		value    any
	}{
		{name: "string type", typeName: "string", value: true},
		{name: "bool type", typeName: "bool", value: "true"},
		{name: "integer type", typeName: "int", value: uint64(1)},
		{name: "fractional integer", typeName: "int", value: 1.5},
		{name: "integer NaN", typeName: "int", value: math.NaN()},
		{name: "integer positive infinity", typeName: "int", value: math.Inf(1)},
		{name: "integer upper limit", typeName: "int", value: float64(math.MaxInt64)},
		{name: "integer below lower limit", typeName: "int", value: math.Nextafter(float64(math.MinInt64), math.Inf(-1))},
		{name: "JSON fractional integer", typeName: "int", value: json.Number("1.5")},
		{name: "JSON integer overflow", typeName: "int", value: json.Number("9223372036854775808")},
		{name: "float type", typeName: "float", value: "1"},
		{name: "float NaN", typeName: "float", value: math.NaN()},
		{name: "float infinity", typeName: "float", value: math.Inf(-1)},
		{name: "JSON float overflow", typeName: "float", value: json.Number("1e1000")},
		{name: "unsupported declaration", typeName: "object", value: map[string]any{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := normalizeInput(tt.typeName, tt.value); err == nil {
				t.Fatalf("normalizeInput(%q, %#v) = %#v, want error", tt.typeName, tt.value, got)
			}
		})
	}
}

func TestResolveInputsRejectsRoundedIntegerOverflow(t *testing.T) {
	definitions := []ir.Input{{Name: "value", Type: "int", Required: true}}
	maximumValidFloat := math.Nextafter(float64(math.MaxInt64), math.Inf(-1))
	resolved, err := resolveInputs(definitions, map[string]any{"value": maximumValidFloat})
	if err != nil || resolved["value"] != int64(maximumValidFloat) {
		t.Fatalf("maximum valid input = %#v, error = %v", resolved, err)
	}
	_, err = resolveInputs(definitions, map[string]any{"value": float64(math.MaxInt64)})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_INPUT_TYPE" || execution.Path != "input.value" || execution.Cause == nil {
		t.Fatalf("overflow error = %#v", err)
	}
}

func TestResolveInputsReportsUnknownNamesDeterministically(t *testing.T) {
	provided := map[string]any{"zeta": 1, "alpha": 2, "middle": 3}
	for iteration := 0; iteration < 1000; iteration++ {
		_, err := resolveInputs(nil, provided)
		var execution *ExecutionError
		if !errors.As(err, &execution) || execution.Code != "E_INPUT_UNKNOWN" || execution.Path != "input.alpha" || execution.Message != `unknown input "alpha"` {
			t.Fatalf("iteration %d error = %#v", iteration, err)
		}
	}
	resolved, err := resolveInputs([]ir.Input{{Name: "alpha", Type: "int", Required: true}}, map[string]any{"alpha": int64(1)})
	if err != nil || resolved["alpha"] != int64(1) {
		t.Fatalf("known input = %#v, error = %v", resolved, err)
	}
}

func TestResolveInputsDefaultsAndFailures(t *testing.T) {
	validDefault := json.RawMessage(`true`)
	definitions := []ir.Input{{Name: "enabled", Type: "bool", Default: &validDefault}}
	resolved, err := resolveInputs(definitions, nil)
	if err != nil || resolved["enabled"] != true {
		t.Fatalf("default input = %#v, error = %v", resolved, err)
	}
	resolved, err = resolveInputs(definitions, map[string]any{"enabled": false})
	if err != nil || resolved["enabled"] != false {
		t.Fatalf("provided input = %#v, error = %v", resolved, err)
	}

	malformedDefault := json.RawMessage(`true false`)
	wrongTypeDefault := json.RawMessage(`"value"`)
	for _, tt := range []struct {
		name       string
		definition ir.Input
		wantCode   string
	}{
		{name: "trailing default data", definition: ir.Input{Name: "value", Type: "bool", Default: &malformedDefault}, wantCode: "E_INPUT_DEFAULT"},
		{name: "default type", definition: ir.Input{Name: "value", Type: "int", Default: &wrongTypeDefault}, wantCode: "E_INPUT_TYPE"},
		{name: "required missing", definition: ir.Input{Name: "value", Type: "string", Required: true}, wantCode: "E_INPUT_REQUIRED"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveInputs([]ir.Input{tt.definition}, nil)
			var execution *ExecutionError
			if !errors.As(err, &execution) || execution.Code != tt.wantCode || execution.Path != "input.value" {
				t.Fatalf("error = %#v", err)
			}
			if tt.wantCode != "E_INPUT_REQUIRED" && execution.Cause == nil {
				t.Fatal("error cause is nil")
			}
		})
	}
}
