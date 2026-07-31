package scrapekdl_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestResultDecodeNestedStructsAndSlices(t *testing.T) {
	type decodedRow struct {
		ID    uint64  `json:"id"`
		Count int8    `json:"count"`
		Ratio float32 `json:"ratio"`
		Note  *string `json:"note"`
	}
	type decodedOutput struct {
		Title   string       `json:"title"`
		Rows    []decodedRow `json:"rows"`
		Missing *string      `json:"missing"`
	}
	result := &scrapekdl.Result{
		Value: map[string]any{
			"title": "Example",
			"rows": []any{map[string]any{
				"id": uint64(math.MaxUint64), "count": int8(-8), "ratio": float32(1.5), "note": nil,
			}},
		},
		Warnings: []scrapekdl.Warning{{Code: "W_ERROR_RECOVERED", Message: "recovered"}},
		Partial:  true,
	}
	var output decodedOutput
	if err := result.Decode(&output); err != nil {
		t.Fatal(err)
	}
	if output.Title != "Example" || len(output.Rows) != 1 || output.Rows[0].ID != math.MaxUint64 || output.Rows[0].Count != -8 || output.Rows[0].Ratio != 1.5 || output.Rows[0].Note != nil || output.Missing != nil {
		t.Fatalf("decoded output = %#v", output)
	}
	if !result.Partial || len(result.Warnings) != 1 {
		t.Fatalf("Decode changed result metadata: %#v", result)
	}
}

func TestResultDecodeEveryIntegerRepresentationExactly(t *testing.T) {
	type integers struct {
		Int    int    `json:"int"`
		Int8   int8   `json:"int8"`
		Int16  int16  `json:"int16"`
		Int32  int32  `json:"int32"`
		Int64  int64  `json:"int64"`
		Uint   uint   `json:"uint"`
		Uint8  uint8  `json:"uint8"`
		Uint16 uint16 `json:"uint16"`
		Uint32 uint32 `json:"uint32"`
		Uint64 uint64 `json:"uint64"`
		JSON   int64  `json:"json"`
	}
	result := &scrapekdl.Result{Value: map[string]any{
		"int": int(-1), "int8": int8(math.MinInt8), "int16": int16(math.MinInt16),
		"int32": int32(math.MinInt32), "int64": int64(math.MinInt64), "uint": uint(1),
		"uint8": uint8(math.MaxUint8), "uint16": uint16(math.MaxUint16),
		"uint32": uint32(math.MaxUint32), "uint64": uint64(math.MaxUint64),
		"json": json.Number("9.223372036854775807e18"),
	}}
	var output integers
	if err := result.Decode(&output); err != nil {
		t.Fatal(err)
	}
	if output.Int != -1 || output.Int8 != math.MinInt8 || output.Int16 != math.MinInt16 || output.Int32 != math.MinInt32 || output.Int64 != math.MinInt64 || output.Uint64 != math.MaxUint64 || output.JSON != math.MaxInt64 {
		t.Fatalf("decoded integers = %#v", output)
	}
}

func TestResultDecodeRejectsNumericLoss(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		destination any
	}{
		{name: "signed overflow", value: int16(128), destination: &map[string]int8{}},
		{name: "unsigned overflow", value: uint16(256), destination: &map[string]uint8{}},
		{name: "negative to unsigned", value: int8(-1), destination: &map[string]uint8{}},
		{name: "maximum unsigned to signed", value: uint64(math.MaxUint64), destination: &map[string]int64{}},
		{name: "fractional JSON number to integer", value: json.Number("1.5"), destination: &map[string]int64{}},
		{name: "float to integer", value: float64(1), destination: &map[string]int64{}},
		{name: "integer to float", value: int64(1), destination: &map[string]float64{}},
		{name: "float32 precision loss", value: float64(0.1), destination: &map[string]float32{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (&scrapekdl.Result{Value: map[string]any{"value": test.value}}).Decode(test.destination)
			if err == nil || !strings.Contains(err.Error(), "value.value") {
				t.Fatalf("Decode() error = %v", err)
			}
		})
	}
}

func TestResultDecodeDistinguishesMapMissingAndNull(t *testing.T) {
	var output map[string]*int
	if err := (&scrapekdl.Result{Value: map[string]any{"null": nil}}).Decode(&output); err != nil {
		t.Fatal(err)
	}
	value, hasNull := output["null"]
	_, hasMissing := output["missing"]
	if !hasNull || value != nil || hasMissing {
		t.Fatalf("decoded map = %#v", output)
	}

	type nullableOutput struct {
		Optional *string `json:"optional"`
	}
	var nullable nullableOutput
	if err := (&scrapekdl.Result{Value: map[string]any{}}).Decode(&nullable); err != nil || nullable.Optional != nil {
		t.Fatalf("missing nullable field = %#v, %v", nullable, err)
	}
	if err := (&scrapekdl.Result{Value: map[string]any{"optional": nil}}).Decode(&nullable); err != nil || nullable.Optional != nil {
		t.Fatalf("null nullable field = %#v, %v", nullable, err)
	}
}

func TestResultDecodeUsesExactFieldNamesAndJSONTags(t *testing.T) {
	type output struct {
		Exact  string
		Tagged string  `json:"tagged_name"`
		Extra  *string `json:"extra"`
	}
	var decoded output
	err := (&scrapekdl.Result{Value: map[string]any{"Exact": "one", "tagged_name": "two"}}).Decode(&decoded)
	if err != nil || decoded.Exact != "one" || decoded.Tagged != "two" {
		t.Fatalf("decoded output = %#v, error = %v", decoded, err)
	}
	if err := (&scrapekdl.Result{Value: map[string]any{"exact": "one", "tagged_name": "two"}}).Decode(&decoded); err == nil {
		t.Fatal("case-insensitive field match unexpectedly succeeded")
	}
}

func TestResultDecodeRejectsMissingUnknownAndNullFieldsAtomically(t *testing.T) {
	type output struct {
		Required string `json:"required"`
	}
	original := output{Required: "unchanged"}
	tests := []struct {
		name  string
		value map[string]any
	}{
		{name: "missing", value: map[string]any{}},
		{name: "unknown source", value: map[string]any{"required": "value", "unknown": true}},
		{name: "null", value: map[string]any{"required": nil}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destination := original
			if err := (&scrapekdl.Result{Value: test.value}).Decode(&destination); err == nil {
				t.Fatal("Decode() unexpectedly succeeded")
			}
			if !reflect.DeepEqual(destination, original) {
				t.Fatalf("destination changed after failure: %#v", destination)
			}
		})
	}
}

func TestResultDecodeRejectsInvalidDestinationsWithoutPanicking(t *testing.T) {
	result := &scrapekdl.Result{Value: map[string]any{}}
	var nilMap *map[string]any
	for _, destination := range []any{nil, 1, nilMap, new(string)} {
		if err := result.Decode(destination); err == nil {
			t.Fatalf("Decode(%T) unexpectedly succeeded", destination)
		}
	}
	var destination map[string]any
	if err := (*scrapekdl.Result)(nil).Decode(&destination); err == nil {
		t.Fatal("nil Result Decode unexpectedly succeeded")
	}
}

func TestResultDecodeCopiesInterfaceValues(t *testing.T) {
	source := map[string]any{"nested": map[string]any{"value": "original"}}
	var destination map[string]any
	if err := (&scrapekdl.Result{Value: source}).Decode(&destination); err != nil {
		t.Fatal(err)
	}
	destination["nested"].(map[string]any)["value"] = "mutated"
	if source["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("decoded interface aliases Result.Value: %#v", source)
	}
}
