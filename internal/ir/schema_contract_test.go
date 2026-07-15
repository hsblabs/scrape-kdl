package ir_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/canonicaljson"
	compilerpkg "github.com/hsblabs/scrape-kdl/internal/compiler"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaURL = "https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json"

func TestIRArtifactsValidateAgainstDatedSchema(t *testing.T) {
	root := filepath.Join("..", "..")
	datedPath := filepath.Join(root, "docs", "ir", "2026-07-15", "schema.json")
	aliasPath := filepath.Join(root, "docs", "ir", "schema.json")
	dated, err := os.ReadFile(datedPath)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dated, alias) {
		t.Fatal("docs/ir/schema.json drifted from the dated schema")
	}

	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(dated))
	if err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(schemaURL, schemaDocument); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	goldens, err := filepath.Glob(filepath.Join(root, "fixtures", "expected-ir", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths := append([]string{filepath.Join(root, "docs", "ir", "example.ir.json")}, goldens...)
	if len(paths) < 2 {
		t.Fatal("expected the documented example and at least one golden IR")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode IR: %v", err)
			}
			if err := schema.Validate(instance); err != nil {
				t.Fatalf("schema validation: %v", err)
			}
			canonical, err := canonicaljson.Canonicalize(data)
			if err != nil {
				t.Fatalf("canonicalize: %v", err)
			}
			roundTrip, err := canonicaljson.Marshal(instance)
			if err != nil {
				t.Fatalf("round-trip: %v", err)
			}
			if !bytes.Equal(canonical, roundTrip) {
				t.Fatalf("canonical round-trip drift\ncanonical: %s\nround-trip: %s", canonical, roundTrip)
			}
			if filepath.Dir(path) == filepath.Join(root, "fixtures", "expected-ir") && strings.HasSuffix(path, ".ir.json") {
				fixtureName := strings.TrimSuffix(filepath.Base(path), ".ir.json") + ".kdl"
				extractor, diagnostics := compilerpkg.CompileFile(filepath.Join(root, "fixtures", "valid", fixtureName))
				if diagnostics.HasErrors() || extractor == nil {
					t.Fatalf("compile %s: %#v", fixtureName, diagnostics)
				}
				typedRoundTrip, err := canonicaljson.Marshal(extractor)
				if err != nil {
					t.Fatalf("marshal typed Go IR: %v", err)
				}
				if !bytes.Equal(canonical, typedRoundTrip) {
					t.Fatalf("typed Go IR round-trip drift\ncanonical: %s\nround-trip: %s", canonical, typedRoundTrip)
				}
			}
		})
	}
}

func TestIRSchemaRejectsContractDrift(t *testing.T) {
	root := filepath.Join("..", "..")
	schemaData, err := os.ReadFile(filepath.Join(root, "docs", "ir", "2026-07-15", "schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(schemaURL, schemaDocument); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatal(err)
	}
	goldenData, err := os.ReadFile(filepath.Join(root, "fixtures", "expected-ir", "race-detail.ir.json"))
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]any
	if err := json.Unmarshal(goldenData, &original); err != nil {
		t.Fatal(err)
	}

	tests := map[string]func(map[string]any){
		"legacy IR version": func(value map[string]any) { value["irVersion"] = "0.1" },
		"unknown property":  func(value map[string]any) { value["future"] = true },
		"missing file hash": func(value map[string]any) {
			delete(value["files"].([]any)[0].(map[string]any), "sha256")
		},
		"unregistered capability": func(value map[string]any) {
			value["capabilities"] = append(value["capabilities"].([]any), "browser.future")
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var candidate map[string]any
			if err := json.Unmarshal(encoded, &candidate); err != nil {
				t.Fatal(err)
			}
			mutate(candidate)
			if err := schema.Validate(candidate); err == nil {
				t.Fatal("schema validation succeeded")
			}
		})
	}
}
