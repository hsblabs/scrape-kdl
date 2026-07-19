package clisupport

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractionRenderingContract(t *testing.T) {
	result := map[string]any{"value": map[string]any{"title": "ok"}}
	bare, err := MarshalExtractionResult(result, false)
	if err != nil {
		t.Fatal(err)
	}
	var bareDocument map[string]any
	if err := json.Unmarshal(bare, &bareDocument); err != nil || bareDocument["ok"] != nil {
		t.Fatalf("bare result = %q, %v", bare, err)
	}
	envelope, err := MarshalExtractionResult(result, true)
	if err != nil {
		t.Fatal(err)
	}
	var machineDocument struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(envelope, &machineDocument); err != nil || !machineDocument.OK || machineDocument.Result["value"] == nil {
		t.Fatalf("machine result = %q, %#v, %v", envelope, machineDocument, err)
	}
	if !strings.HasSuffix(string(bare), "\n") || !strings.HasSuffix(string(envelope), "\n") {
		t.Fatal("rendered documents are not newline terminated")
	}
}

func TestWarningFormattingContract(t *testing.T) {
	got := FormatWarnings([]Warning{
		{Code: "W_ONE", Message: "first"},
		{Code: "W_TWO", Path: "output.items", Message: "second"},
	})
	want := "W_ONE: first\nW_TWO at output.items: second\n"
	if got != want {
		t.Fatalf("warnings = %q, want %q", got, want)
	}
}

func TestHasJSONFlag(t *testing.T) {
	if !HasJSONFlag([]string{"--spec", "x", "--json"}) || HasJSONFlag([]string{"--spec", "x"}) {
		t.Fatal("JSON flag detection disagrees with CLI contract")
	}
}
