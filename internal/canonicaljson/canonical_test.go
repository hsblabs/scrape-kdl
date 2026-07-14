package canonicaljson

import (
	"math"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	input := []byte(`{"z":-0.0,"\uE000":1.2300e2,"\uD800\uDC00":[null,true],"a":{"present":null}}`)
	got, err := Canonicalize(input)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":{"present":null},"z":0,"":123,"𐀀":[null,true]}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s", got, want)
	}
	again, err := Canonicalize(got)
	if err != nil || string(again) != want {
		t.Fatalf("second canonicalization = %s, %v", again, err)
	}
}

func TestCanonicalNumberForms(t *testing.T) {
	tests := map[string]string{
		`1.0`: "1", `1e3`: "1000", `1.25e-3`: "0.00125", `-0`: "0", `-0e20`: "0",
		`9223372036854775807`: "9223372036854775807",
	}
	for input, want := range tests {
		got, err := Canonicalize([]byte(input))
		if err != nil || string(got) != want {
			t.Fatalf("Canonicalize(%s) = %s, %v; want %s", input, got, err, want)
		}
	}
}

func TestCanonicalizeRejectsAmbiguousOrInvalidJSON(t *testing.T) {
	for _, input := range [][]byte{
		[]byte(`{"a":1,"a":2}`),
		[]byte("{} true"),
		[]byte("1e999"),
		{0xff},
	} {
		if _, err := Canonicalize(input); err == nil {
			t.Fatalf("Canonicalize(%q) succeeded", input)
		}
	}
	if _, err := Marshal(math.Inf(1)); err == nil {
		t.Fatal("Marshal(+Inf) succeeded")
	}
}

func TestCanonicalizePreservesOmissionAndNull(t *testing.T) {
	withNull, err := Canonicalize([]byte(`{"optional":null}`))
	if err != nil {
		t.Fatal(err)
	}
	omitted, err := Canonicalize([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(withNull) != `{"optional":null}` || string(omitted) != `{}` || string(withNull) == string(omitted) {
		t.Fatalf("with null = %s, omitted = %s", withNull, omitted)
	}
}

func TestCanonicalizeUsesPortableMinimalStringEscapes(t *testing.T) {
	input := []byte(`{"html":"\u003cscript\u003e&","controls":"\u0000\b\f\n\r\t","unicode":"\u2028"}`)
	want := "{\"controls\":\"\\u0000\\b\\f\\n\\r\\t\",\"html\":\"<script>&\",\"unicode\":\"\u2028\"}"
	got, err := Canonicalize(input)
	if err != nil || string(got) != want {
		t.Fatalf("Canonicalize() = %q, %v; want %q", got, err, want)
	}
	if _, err := Marshal(string([]byte{0xff})); err == nil {
		t.Fatal("Marshal(invalid UTF-8 string) succeeded")
	}
}
