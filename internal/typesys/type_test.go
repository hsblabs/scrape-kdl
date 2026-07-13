package typesys

import "testing"

func TestParsePostfixTypes(t *testing.T) {
	tests := map[string]string{
		"string?[]":   "string?[]",
		"string[]?":   "string[]?",
		"(string[])?": "string[]?",
		"object?":     "object?",
	}
	for input, want := range tests {
		got, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if got.String() != want {
			t.Fatalf("Parse(%q) = %q, want %q", input, got.String(), want)
		}
	}
}

func TestNestedNullableRejected(t *testing.T) {
	if _, err := Parse("string??"); err == nil {
		t.Fatal("expected nested nullable error")
	}
}
