// Package authoring provides a bounded semantic document model, a versioned
// built-in catalog, and deterministic KDL writing for authoring tools.
package authoring

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/hsblabs/scrape-kdl/internal/builtincontract"
)

const languageVersion20260715 = "2026-07-15"

type ScalarKind string

const (
	ScalarString ScalarKind = "string"
	ScalarBool   ScalarKind = "bool"
	ScalarInt    ScalarKind = "int"
	ScalarFloat  ScalarKind = "float"
	ScalarNull   ScalarKind = "null"
)

// Scalar is a KDL scalar accepted in built-in transform calls.
type Scalar struct {
	kind        ScalarKind
	stringValue string
	boolValue   bool
	intValue    int64
	floatValue  float64
}

func String(value string) Scalar { return Scalar{kind: ScalarString, stringValue: value} }
func Bool(value bool) Scalar     { return Scalar{kind: ScalarBool, boolValue: value} }
func Int(value int64) Scalar     { return Scalar{kind: ScalarInt, intValue: value} }
func Null() Scalar               { return Scalar{kind: ScalarNull} }

func Float(value float64) (Scalar, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Scalar{}, errors.New("authoring: float scalar must be finite")
	}
	return Scalar{kind: ScalarFloat, floatValue: value}, nil
}

func (scalar Scalar) Kind() ScalarKind { return scalar.kind }

func (scalar Scalar) Value() any {
	switch scalar.kind {
	case ScalarString:
		return scalar.stringValue
	case ScalarBool:
		return scalar.boolValue
	case ScalarInt:
		return scalar.intValue
	case ScalarFloat:
		return scalar.floatValue
	case ScalarNull:
		return nil
	default:
		return nil
	}
}

func (scalar Scalar) MarshalJSON() ([]byte, error) {
	if scalar.kind == "" {
		return nil, errors.New("authoring: scalar is uninitialized")
	}
	return json.Marshal(scalar.Value())
}

type InputConstraint string

const (
	InputString        InputConstraint = "string"
	InputStringArray   InputConstraint = "string-array"
	InputNonNullScalar InputConstraint = "non-null-scalar"
	InputNullable      InputConstraint = "nullable"
	InputScalar        InputConstraint = "scalar"
	InputNumber        InputConstraint = "number"
)

type OutputConstraint string

const (
	OutputString         OutputConstraint = "string"
	OutputNullableString OutputConstraint = "nullable-string"
	OutputStringArray    OutputConstraint = "string-array"
	OutputBool           OutputConstraint = "bool"
	OutputTargetInteger  OutputConstraint = "target-integer"
	OutputTargetFloat    OutputConstraint = "target-float"
	OutputInnerInput     OutputConstraint = "inner-input"
	OutputSameAsInput    OutputConstraint = "same-as-input"
)

type NullabilityEffect string

const (
	NullabilityPreserved  NullabilityEffect = "preserved"
	NullabilityIntroduced NullabilityEffect = "introduced"
	NullabilityRemoved    NullabilityEffect = "removed"
)

type NamedArgument struct {
	Name          string   `json:"name"`
	Constraint    string   `json:"constraint"`
	Required      bool     `json:"required"`
	Default       *Scalar  `json:"default,omitempty"`
	AllowedValues []Scalar `json:"allowedValues,omitempty"`
	Minimum       *int64   `json:"minimum,omitempty"`
	Maximum       *int64   `json:"maximum,omitempty"`
}

type PositionalArguments struct {
	Constraint string `json:"constraint"`
	Min        int    `json:"min"`
	Max        int    `json:"max"`
}

type BuiltinDefinition struct {
	Name                string              `json:"name"`
	Input               InputConstraint     `json:"input"`
	Output              OutputConstraint    `json:"output"`
	NullabilityEffect   NullabilityEffect   `json:"nullabilityEffect"`
	NamedArguments      []NamedArgument     `json:"namedArguments"`
	PositionalArguments PositionalArguments `json:"positionalArguments"`
}

func (definition BuiltinDefinition) Call(positional []Scalar, named map[string]Scalar) BuiltinCall {
	positionalCopy := append([]Scalar(nil), positional...)
	namedCopy := make(map[string]Scalar, len(named))
	for name, value := range named {
		namedCopy[name] = value
	}
	return BuiltinCall{Name: definition.Name, Positional: positionalCopy, Named: namedCopy}
}

type Catalog struct {
	LanguageVersion string              `json:"languageVersion"`
	Builtins        []BuiltinDefinition `json:"builtins"`
}

func (catalog Catalog) Lookup(name string) (BuiltinDefinition, bool) {
	index := sort.Search(len(catalog.Builtins), func(index int) bool { return catalog.Builtins[index].Name >= name })
	if index == len(catalog.Builtins) || catalog.Builtins[index].Name != name {
		return BuiltinDefinition{}, false
	}
	return cloneBuiltin(catalog.Builtins[index]), true
}

func SupportedBuiltinCatalogVersions() []string {
	return []string{languageVersion20260715}
}

func BuiltinCatalog(languageVersion string) (Catalog, error) {
	if languageVersion != languageVersion20260715 {
		return Catalog{}, fmt.Errorf("authoring: unsupported built-in catalog language version %q", languageVersion)
	}
	catalog, err := buildCatalog(languageVersion)
	if err != nil {
		return Catalog{}, err
	}
	return cloneCatalog(catalog), nil
}

type transformMetadata struct {
	input                InputConstraint
	output               OutputConstraint
	nullability          NullabilityEffect
	defaults             map[string]Scalar
	argumentRules        map[string]argumentRules
	positionalConstraint string
}

type argumentRules struct {
	allowedValues []Scalar
	minimum       *int64
	maximum       *int64
}

var transformMetadataByName = map[string]transformMetadata{
	"trim":                 stringToString(),
	"normalize-whitespace": stringToString(),
	"lowercase":            stringToString(),
	"uppercase":            stringToString(),
	"replace":              stringToString(),
	"regex-replace":        withDefaults(stringToString(), map[string]Scalar{"flags": String("")}),
	"regex-capture":        nullableString(map[string]Scalar{"group": Int(0), "flags": String("")}),
	"substring":            stringToString(),
	"split":                {input: InputString, output: OutputStringArray, nullability: NullabilityPreserved},
	"join":                 {input: InputStringArray, output: OutputString, nullability: NullabilityPreserved},
	"prepend":              stringToString(),
	"append":               stringToString(),
	"parse-int": {
		input: InputString, output: OutputTargetInteger, nullability: NullabilityPreserved,
		defaults: map[string]Scalar{"radix": Int(10)},
		argumentRules: map[string]argumentRules{
			"as":    {allowedValues: stringScalars("int", "u8", "u16", "u32", "u64", "i8", "i16", "i32", "i64")},
			"radix": integerRange(2, 36),
		},
	},
	"parse-float": {
		input: InputString, output: OutputTargetFloat, nullability: NullabilityPreserved,
		argumentRules: map[string]argumentRules{"as": {allowedValues: stringScalars("float", "f32", "f64")}},
	},
	"parse-bool":     {input: InputString, output: OutputBool, nullability: NullabilityPreserved, defaults: map[string]Scalar{"case-sensitive": Bool(false), "true": String("true"), "false": String("false")}},
	"to-string":      {input: InputNonNullScalar, output: OutputString, nullability: NullabilityPreserved},
	"empty-to-null":  nullableString(nil),
	"coalesce":       {input: InputNullable, output: OutputInnerInput, nullability: NullabilityRemoved},
	"url-resolve":    stringToString(),
	"url-query":      nullableString(map[string]Scalar{"index": Int(0)}),
	"url-path":       stringToString(),
	"path-segment":   nullableString(nil),
	"assert-matches": withDefaults(stringToString(), map[string]Scalar{"flags": String("")}),
	"assert-enum":    {input: InputScalar, output: OutputSameAsInput, nullability: NullabilityPreserved, positionalConstraint: "same-as-input"},
	"assert-min":     {input: InputNumber, output: OutputSameAsInput, nullability: NullabilityPreserved},
	"assert-max":     {input: InputNumber, output: OutputSameAsInput, nullability: NullabilityPreserved},
}

func stringToString() transformMetadata {
	return transformMetadata{input: InputString, output: OutputString, nullability: NullabilityPreserved}
}

func nullableString(defaults map[string]Scalar) transformMetadata {
	return transformMetadata{input: InputString, output: OutputNullableString, nullability: NullabilityIntroduced, defaults: defaults}
}

func withDefaults(metadata transformMetadata, defaults map[string]Scalar) transformMetadata {
	metadata.defaults = defaults
	return metadata
}

func stringScalars(values ...string) []Scalar {
	result := make([]Scalar, len(values))
	for index, value := range values {
		result[index] = String(value)
	}
	return result
}

func integerRange(minimum, maximum int64) argumentRules {
	return argumentRules{minimum: &minimum, maximum: &maximum}
}

func buildCatalog(languageVersion string) (Catalog, error) {
	contracts := builtincontract.All()
	if len(contracts) != len(transformMetadataByName) {
		return Catalog{}, errors.New("authoring: built-in catalog metadata does not cover the compiler registry")
	}
	names := make([]string, 0, len(contracts))
	for name := range contracts {
		if _, ok := transformMetadataByName[name]; !ok {
			return Catalog{}, fmt.Errorf("authoring: built-in %q is missing catalog metadata", name)
		}
		names = append(names, name)
	}
	for name := range transformMetadataByName {
		if _, ok := contracts[name]; !ok {
			return Catalog{}, fmt.Errorf("authoring: catalog metadata references unknown built-in %q", name)
		}
	}
	sort.Strings(names)
	builtins := make([]BuiltinDefinition, 0, len(names))
	for _, name := range names {
		contract := contracts[name]
		metadata := transformMetadataByName[name]
		required := make(map[string]struct{}, len(contract.Required))
		for _, argument := range contract.Required {
			required[argument] = struct{}{}
		}
		argumentNames := make([]string, 0, len(contract.Properties))
		for argument := range contract.Properties {
			argumentNames = append(argumentNames, argument)
		}
		sort.Strings(argumentNames)
		named := make([]NamedArgument, 0, len(argumentNames))
		for _, argument := range argumentNames {
			definition := NamedArgument{Name: argument, Constraint: string(contract.Properties[argument])}
			_, definition.Required = required[argument]
			if value, ok := metadata.defaults[argument]; ok {
				valueCopy := value
				definition.Default = &valueCopy
			}
			if rules, ok := metadata.argumentRules[argument]; ok {
				definition.AllowedValues = append([]Scalar(nil), rules.allowedValues...)
				definition.Minimum = cloneInt64(rules.minimum)
				definition.Maximum = cloneInt64(rules.maximum)
			}
			named = append(named, definition)
		}
		for argument := range metadata.defaults {
			if _, ok := contract.Properties[argument]; !ok {
				return Catalog{}, fmt.Errorf("authoring: built-in %q default references unknown argument %q", name, argument)
			}
		}
		for argument := range metadata.argumentRules {
			if _, ok := contract.Properties[argument]; !ok {
				return Catalog{}, fmt.Errorf("authoring: built-in %q rules reference unknown argument %q", name, argument)
			}
		}
		builtins = append(builtins, BuiltinDefinition{
			Name: name, Input: metadata.input, Output: metadata.output, NullabilityEffect: metadata.nullability,
			NamedArguments: named,
			PositionalArguments: PositionalArguments{
				Constraint: metadata.positionalConstraint, Min: contract.MinPositional, Max: contract.MaxPositional,
			},
		})
	}
	return Catalog{LanguageVersion: languageVersion, Builtins: builtins}, nil
}

func cloneCatalog(catalog Catalog) Catalog {
	clone := Catalog{LanguageVersion: catalog.LanguageVersion, Builtins: make([]BuiltinDefinition, len(catalog.Builtins))}
	for index, definition := range catalog.Builtins {
		clone.Builtins[index] = cloneBuiltin(definition)
	}
	return clone
}

func cloneBuiltin(definition BuiltinDefinition) BuiltinDefinition {
	clone := definition
	clone.NamedArguments = make([]NamedArgument, len(definition.NamedArguments))
	for index, argument := range definition.NamedArguments {
		clone.NamedArguments[index] = argument
		clone.NamedArguments[index].AllowedValues = append([]Scalar(nil), argument.AllowedValues...)
		if argument.Default != nil {
			value := *argument.Default
			clone.NamedArguments[index].Default = &value
		}
		clone.NamedArguments[index].Minimum = cloneInt64(argument.Minimum)
		clone.NamedArguments[index].Maximum = cloneInt64(argument.Maximum)
	}
	return clone
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
