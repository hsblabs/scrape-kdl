package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
	"github.com/hsblabs/scrape-kdl/internal/builtincontract"
)

func TestTracerDocumentWritesDeterministicallyAndCompiles(t *testing.T) {
	catalog, err := BuiltinCatalog("2026-07-15")
	if err != nil {
		t.Fatal(err)
	}
	normalize := mustBuiltin(t, catalog, "normalize-whitespace")
	emptyToNull := mustBuiltin(t, catalog, "empty-to-null")
	assertEnum := mustBuiltin(t, catalog, "assert-enum")
	parseFloat := mustBuiltin(t, catalog, "parse-float")
	prepend := mustBuiltin(t, catalog, "prepend")
	one := mustFloat(t, 1)
	negativeZero := mustFloat(t, math.Copysign(0, -1))
	document := Document{
		LanguageVersion: "2026-07-15",
		Extractor: Extractor{
			Name: "authoring-tracer", Version: "2026-08-01",
			Source: Source{
				FetchMode: scrapekdl.FetchModeHTTP, URLTemplate: "https://example.invalid/items/{item_id}",
				SessionPolicy: scrapekdl.SessionPolicyOptional,
			},
			Inputs: []Input{{Name: "item_id", Type: PrimitiveString, Required: true}},
			Members: []Member{
				Field{
					Name: "title", Type: "string", Required: true, Selector: "h1", Match: MatchOne,
					Value: TextValue{}, OnError: ErrorFail,
					Transforms: []BuiltinCall{
						normalize.Call(nil, nil),
						prepend.Call(nil, map[string]Scalar{"value": String("prefix \"quoted\"\n")}),
					},
				},
				Field{
					Name: "price", Type: "float", Required: true, Selector: ".price", Match: MatchOne,
					Value: TextValue{}, OnError: ErrorFail,
					Transforms: []BuiltinCall{
						parseFloat.Call(nil, map[string]Scalar{"as": String("float")}),
						assertEnum.Call([]Scalar{one, negativeZero}, nil),
					},
				},
				Collection{
					Name: "items", Selector: "ul > li", MinItems: 1, OnRowError: RowErrorSkip,
					Members: []Member{Field{
						Name: "label", Type: "string?", Selector: ".label", Match: MatchFirst,
						Value: AttributeValue{Name: "data-label"},
						Transforms: []BuiltinCall{
							normalize.Call(nil, nil),
							emptyToNull.Call(nil, nil),
							assertEnum.Call([]Scalar{String("known"), Null()}, nil),
						}, OnError: ErrorWarn,
					}},
				},
			},
		},
	}

	first, err := Write(document)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(document)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("writer is not deterministic\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	want, err := os.ReadFile("../fixtures/authoring/tracer.kdl")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, want) {
		t.Fatalf("writer output:\n%s\nwant:\n%s", first, want)
	}
	program, diagnostics, err := scrapekdl.Compile(context.Background(), scrapekdl.Source{Path: "authoring-tracer.kdl", Data: first}, scrapekdl.CompileOptions{})
	if err != nil || diagnostics.HasErrors() || program == nil {
		t.Fatalf("compiled program = %#v, diagnostics = %#v, error = %v", program, diagnostics, err)
	}
}

func TestBuiltinCatalogMatchesNormativeAndCompilerContracts(t *testing.T) {
	catalog, err := BuiltinCatalog("2026-07-15")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../docs/spec/builtins-v0.1.authoring.json")
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("authoring catalog differs from normative contract\n got: %s\nwant: %s", encoded, data)
	}

	contracts := builtincontract.All()
	if len(catalog.Builtins) != len(contracts) {
		t.Fatalf("catalog built-ins = %d, compiler registry = %d", len(catalog.Builtins), len(contracts))
	}
	for _, builtin := range catalog.Builtins {
		contract, ok := contracts[builtin.Name]
		if !ok {
			t.Fatalf("catalog contains unknown compiler built-in %q", builtin.Name)
		}
		if builtin.PositionalArguments.Min != contract.MinPositional || builtin.PositionalArguments.Max != contract.MaxPositional {
			t.Fatalf("%s positional contract = %#v, compiler = %#v", builtin.Name, builtin.PositionalArguments, contract)
		}
		if len(builtin.NamedArguments) != len(contract.Properties) {
			t.Fatalf("%s named arguments = %#v, compiler = %#v", builtin.Name, builtin.NamedArguments, contract.Properties)
		}
		required := map[string]bool{}
		for _, name := range contract.Required {
			required[name] = true
		}
		for _, argument := range builtin.NamedArguments {
			if string(contract.Properties[argument.Name]) != argument.Constraint || required[argument.Name] != argument.Required {
				t.Fatalf("%s.%s = %#v, compiler expectation = %q required=%v", builtin.Name, argument.Name, argument, contract.Properties[argument.Name], required[argument.Name])
			}
		}
	}
	parseInt := mustBuiltin(t, catalog, "parse-int")
	parseIntAs := mustNamedArgument(t, parseInt, "as")
	if !reflect.DeepEqual(parseIntAs.AllowedValues, stringScalars("int", "u8", "u16", "u32", "u64", "i8", "i16", "i32", "i64")) {
		t.Fatalf("parse-int.as allowed values = %#v", parseIntAs.AllowedValues)
	}
	radix := mustNamedArgument(t, parseInt, "radix")
	if radix.Minimum == nil || *radix.Minimum != 2 || radix.Maximum == nil || *radix.Maximum != 36 {
		t.Fatalf("parse-int.radix range = %#v..%#v", radix.Minimum, radix.Maximum)
	}
	parseFloat := mustBuiltin(t, catalog, "parse-float")
	if got := mustNamedArgument(t, parseFloat, "as").AllowedValues; !reflect.DeepEqual(got, stringScalars("float", "f32", "f64")) {
		t.Fatalf("parse-float.as allowed values = %#v", got)
	}
}

func TestBuiltinCatalogIsExplicitAndReturnsIndependentSnapshots(t *testing.T) {
	if _, err := BuiltinCatalog(""); err == nil {
		t.Fatal("implicit catalog version accepted")
	}
	if got := SupportedBuiltinCatalogVersions(); !reflect.DeepEqual(got, []string{"2026-07-15"}) {
		t.Fatalf("supported catalog versions = %v", got)
	}
	first, err := BuiltinCatalog("2026-07-15")
	if err != nil {
		t.Fatal(err)
	}
	first.Builtins[0].Name = "mutated"
	if len(first.Builtins[1].NamedArguments) > 0 {
		first.Builtins[1].NamedArguments[0].Name = "mutated"
	}
	for builtinIndex := range first.Builtins {
		if first.Builtins[builtinIndex].Name != "parse-int" {
			continue
		}
		for argumentIndex := range first.Builtins[builtinIndex].NamedArguments {
			argument := &first.Builtins[builtinIndex].NamedArguments[argumentIndex]
			switch argument.Name {
			case "as":
				argument.AllowedValues[0] = String("mutated")
			case "radix":
				*argument.Minimum = 0
			}
		}
	}
	second, err := BuiltinCatalog("2026-07-15")
	if err != nil {
		t.Fatal(err)
	}
	if second.Builtins[0].Name == "mutated" {
		t.Fatal("catalog mutation escaped returned snapshot")
	}
	secondParseInt := mustBuiltin(t, second, "parse-int")
	if mustNamedArgument(t, secondParseInt, "as").AllowedValues[0].Value() == "mutated" || *mustNamedArgument(t, secondParseInt, "radix").Minimum == 0 {
		t.Fatal("nested catalog mutation escaped returned snapshot")
	}
}

func TestWriterRejectsCallsOutsideSelectedCatalog(t *testing.T) {
	document := Document{
		LanguageVersion: "2026-07-15",
		Extractor: Extractor{
			Name: "invalid", Version: "2026-08-01",
			Source: Source{FetchMode: scrapekdl.FetchModeHTTP, URLTemplate: "https://example.invalid/", SessionPolicy: scrapekdl.SessionPolicyNone},
			Members: []Member{Field{
				Name: "value", Type: "string", Required: true, Selector: "body", Match: MatchOne,
				Value: TextValue{}, OnError: ErrorFail, Transforms: []BuiltinCall{{Name: "private-transform"}},
			}},
		},
	}
	if _, err := Write(document); err == nil {
		t.Fatal("unknown built-in accepted")
	}
	document.Extractor.Members = []Member{Field{
		Name: "value", Type: "string", Required: true, Selector: "body", Match: MatchOne,
		Value: TextValue{}, OnError: ErrorFail, Transforms: []BuiltinCall{{Name: "replace"}},
	}}
	if _, err := Write(document); err == nil {
		t.Fatal("missing required built-in arguments accepted")
	}
	parseInt := mustBuiltin(t, mustCatalog(t), "parse-int")
	document.Extractor.Members = []Member{Field{
		Name: "value", Type: "int", Required: true, Selector: "body", Match: MatchOne,
		Value: TextValue{}, OnError: ErrorFail,
		Transforms: []BuiltinCall{parseInt.Call(nil, map[string]Scalar{"as": String("decimal")})},
	}}
	if _, err := Write(document); err == nil {
		t.Fatal("built-in argument outside allowed values accepted")
	}
	document.Extractor.Members = []Member{Field{
		Name: "value", Type: "int", Required: true, Selector: "body", Match: MatchOne,
		Value: TextValue{}, OnError: ErrorFail,
		Transforms: []BuiltinCall{parseInt.Call(nil, map[string]Scalar{"as": String("int"), "radix": Int(1)})},
	}}
	if _, err := Write(document); err == nil {
		t.Fatal("built-in argument below catalog minimum accepted")
	}
	document.Extractor.Members = []Member{Field{
		Name: "value", Type: "int", Required: true, Selector: "body", Match: MatchOne,
		Value: TextValue{}, OnError: ErrorFail,
		Transforms: []BuiltinCall{parseInt.Call(nil, map[string]Scalar{"as": String("int"), "radix": Int(37)})},
	}}
	if _, err := Write(document); err == nil {
		t.Fatal("built-in argument above catalog maximum accepted")
	}
}

func mustCatalog(t *testing.T) Catalog {
	t.Helper()
	catalog, err := BuiltinCatalog("2026-07-15")
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func mustBuiltin(t *testing.T, catalog Catalog, name string) BuiltinDefinition {
	t.Helper()
	definition, ok := catalog.Lookup(name)
	if !ok {
		t.Fatalf("catalog is missing %q", name)
	}
	return definition
}

func mustNamedArgument(t *testing.T, definition BuiltinDefinition, name string) NamedArgument {
	t.Helper()
	for _, argument := range definition.NamedArguments {
		if argument.Name == name {
			return argument
		}
	}
	t.Fatalf("built-in %q is missing named argument %q", definition.Name, name)
	return NamedArgument{}
}

func mustFloat(t *testing.T, value float64) Scalar {
	t.Helper()
	scalar, err := Float(value)
	if err != nil {
		t.Fatal(err)
	}
	return scalar
}
