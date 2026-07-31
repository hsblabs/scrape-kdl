package authoring

import (
	"bytes"
	"context"
	"encoding/json"
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
	prepend := mustBuiltin(t, catalog, "prepend")
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
	second, err := BuiltinCatalog("2026-07-15")
	if err != nil {
		t.Fatal(err)
	}
	if second.Builtins[0].Name == "mutated" {
		t.Fatal("catalog mutation escaped returned snapshot")
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
}

func mustBuiltin(t *testing.T, catalog Catalog, name string) BuiltinDefinition {
	t.Helper()
	definition, ok := catalog.Lookup(name)
	if !ok {
		t.Fatalf("catalog is missing %q", name)
	}
	return definition
}
