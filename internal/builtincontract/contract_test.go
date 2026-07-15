package builtincontract

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRegistryMatchesNormativeContract(t *testing.T) {
	data, err := os.ReadFile("../../docs/spec/builtins-v0.1.contract.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Definition{}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	got := All()
	for name, definition := range got {
		if definition.Properties == nil {
			definition.Properties = map[string]Expectation{}
		}
		if definition.Required == nil {
			definition.Required = []string{}
		}
		got[name] = definition
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go built-in registry differs from normative contract\n got: %#v\nwant: %#v", got, want)
	}
}
