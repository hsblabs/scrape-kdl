// Package builtincontract defines the static call surface shared by the compiler and executor.
package builtincontract

type Expectation string

const (
	String         Expectation = "string"
	Bool           Expectation = "bool"
	Int            Expectation = "int"
	NonNegativeInt Expectation = "non-negative-int"
	Number         Expectation = "number"
	Scalar         Expectation = "scalar"
)

type Definition struct {
	Properties    map[string]Expectation `json:"properties"`
	Required      []string               `json:"required"`
	MinPositional int                    `json:"minPositional"`
	MaxPositional int                    `json:"maxPositional"`
}

var definitions = map[string]Definition{
	"trim": {}, "normalize-whitespace": {}, "lowercase": {}, "uppercase": {},
	"replace":       {Properties: map[string]Expectation{"old": String, "new": String, "count": NonNegativeInt}, Required: []string{"old", "new"}},
	"regex-replace": {Properties: map[string]Expectation{"pattern": String, "replacement": String, "flags": String, "count": NonNegativeInt}, Required: []string{"pattern", "replacement"}},
	"regex-capture": {Properties: map[string]Expectation{"pattern": String, "group": NonNegativeInt, "flags": String}, Required: []string{"pattern"}},
	"substring":     {Properties: map[string]Expectation{"start": Int, "end": Int}, Required: []string{"start"}},
	"split":         {Properties: map[string]Expectation{"separator": String, "limit": NonNegativeInt}, Required: []string{"separator"}},
	"join":          {Properties: map[string]Expectation{"separator": String}, Required: []string{"separator"}},
	"prepend":       {Properties: map[string]Expectation{"value": String}, Required: []string{"value"}},
	"append":        {Properties: map[string]Expectation{"value": String}, Required: []string{"value"}},
	"parse-int":     {Properties: map[string]Expectation{"as": String, "radix": Int}, Required: []string{"as"}},
	"parse-float":   {Properties: map[string]Expectation{"as": String}, Required: []string{"as"}},
	"parse-bool":    {Properties: map[string]Expectation{"case-sensitive": Bool, "true": String, "false": String}},
	"to-string":     {}, "empty-to-null": {},
	"coalesce":       {Properties: map[string]Expectation{"value": Scalar}, Required: []string{"value"}},
	"url-resolve":    {Properties: map[string]Expectation{"base": String}, Required: []string{"base"}},
	"url-query":      {Properties: map[string]Expectation{"name": String, "index": NonNegativeInt}, Required: []string{"name"}},
	"url-path":       {},
	"path-segment":   {Properties: map[string]Expectation{"index": Int}, Required: []string{"index"}},
	"assert-matches": {Properties: map[string]Expectation{"pattern": String, "flags": String}, Required: []string{"pattern"}},
	"assert-enum":    {MinPositional: 1, MaxPositional: -1},
	"assert-min":     {Properties: map[string]Expectation{"value": Number}, Required: []string{"value"}},
	"assert-max":     {Properties: map[string]Expectation{"value": Number}, Required: []string{"value"}},
}

func Lookup(name string) (Definition, bool) {
	definition, ok := definitions[name]
	return definition, ok
}

func All() map[string]Definition {
	result := make(map[string]Definition, len(definitions))
	for name, definition := range definitions {
		result[name] = definition
	}
	return result
}
