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

func TestParseRejectsMalformedTypes(t *testing.T) {
	for _, input := range []string{"", "missing", "(string", "string)", "string??", "string!"} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("Parse(%q) succeeded", input)
			}
		})
	}
}

func TestTypeEqualityAndAssignability(t *testing.T) {
	stringType := Primitive("string")
	unknownType := Primitive("unknown")
	stringArray := Array(stringType)
	unknownArray := Array(unknownType)
	nullableString := Nullable(stringType)

	tests := []struct {
		name string
		from Type
		to   Type
		want bool
	}{
		{name: "equal", from: stringType, to: stringType, want: true},
		{name: "anything to unknown", from: stringArray, to: unknownType, want: true},
		{name: "nullable lift", from: stringType, to: nullableString, want: true},
		{name: "array element covariance", from: stringArray, to: unknownArray, want: true},
		{name: "unknown is not narrowed", from: unknownType, to: stringType, want: false},
		{name: "nullable is not narrowed", from: nullableString, to: stringType, want: false},
		{name: "scalar is not array", from: stringType, to: stringArray, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAssignable(tt.from, tt.to); got != tt.want {
				t.Fatalf("IsAssignable(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}

	if !Equal(stringArray, Array(stringType)) {
		t.Fatal("equivalent arrays are not equal")
	}
	if Equal(stringArray, unknownArray) {
		t.Fatal("different arrays compare equal")
	}
	if Equal(Type{Kind: KindArray}, Type{Kind: KindArray}) {
		t.Fatal("malformed arrays compare equal")
	}
}

func TestTypeClassifiers(t *testing.T) {
	stringType := Primitive("string")
	integerType := Primitive("u16")
	floatType := Primitive("f64")
	objectType := Primitive("object")
	nullableString := LiftNullable(stringType)

	if !IsNullable(nullableString) || !Equal(nullableString, Nullable(stringType)) {
		t.Fatalf("LiftNullable(string) = %s", nullableString)
	}
	if !Equal(LiftNullable(nullableString), nullableString) {
		t.Fatal("nullable lifting is not idempotent")
	}
	if !IsString(stringType) || IsString(nullableString) {
		t.Fatal("IsString misclassified a type")
	}
	if !IsStringArray(Array(stringType)) || IsStringArray(Array(integerType)) || IsStringArray(Type{Kind: KindArray}) {
		t.Fatal("IsStringArray misclassified a type")
	}
	if !IsNumeric(integerType) || !IsNumeric(floatType) || IsNumeric(stringType) {
		t.Fatal("IsNumeric misclassified a type")
	}
	if !IsInteger(integerType) || IsInteger(floatType) {
		t.Fatal("IsInteger misclassified a type")
	}
	if !IsScalar(nullableString) || IsScalar(objectType) || IsScalar(Array(stringType)) {
		t.Fatal("IsScalar misclassified a type")
	}
}

func TestMalformedTypeStringDoesNotPanic(t *testing.T) {
	for _, malformed := range []Type{
		{Kind: KindArray},
		{Kind: KindNullable},
		{Kind: KindArray, Element: &Type{Kind: KindNullable}},
	} {
		if got := malformed.String(); got == "" {
			t.Fatalf("String() returned an empty value for %#v", malformed)
		}
	}
}
