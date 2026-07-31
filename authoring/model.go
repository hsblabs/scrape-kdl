package authoring

import scrapekdl "github.com/hsblabs/scrape-kdl"

type PrimitiveType string

const (
	PrimitiveString PrimitiveType = "string"
	PrimitiveBool   PrimitiveType = "bool"
	PrimitiveInt    PrimitiveType = "int"
	PrimitiveFloat  PrimitiveType = "float"
)

type MatchMode string

const (
	MatchOne   MatchMode = "one"
	MatchFirst MatchMode = "first"
)

type ErrorPolicy string

const (
	ErrorFail ErrorPolicy = "fail"
	ErrorNull ErrorPolicy = "null"
	ErrorWarn ErrorPolicy = "warn"
)

type RowErrorPolicy string

const (
	RowErrorFail RowErrorPolicy = "fail"
	RowErrorSkip RowErrorPolicy = "skip"
)

type Document struct {
	LanguageVersion string
	Extractor       Extractor
}

type Extractor struct {
	Name    string
	Version string
	Source  Source
	Inputs  []Input
	Members []Member
}

type Source struct {
	FetchMode     scrapekdl.FetchMode
	URLTemplate   string
	SessionPolicy scrapekdl.SessionPolicy
}

type Input struct {
	Name     string
	Type     PrimitiveType
	Required bool
}

type Member interface {
	authoringMember()
}

type Field struct {
	Name       string
	Type       string
	Required   bool
	Selector   string
	Match      MatchMode
	Value      ValueSource
	Transforms []BuiltinCall
	OnError    ErrorPolicy
}

func (Field) authoringMember() {}

type Collection struct {
	Name       string
	Selector   string
	Required   bool
	MinItems   int
	MaxItems   *int
	OnRowError RowErrorPolicy
	Members    []Member
}

func (Collection) authoringMember() {}

type ValueSource interface {
	authoringValueSource()
}

type TextValue struct{}

func (TextValue) authoringValueSource() {}

type HTMLValue struct{}

func (HTMLValue) authoringValueSource() {}

type AttributeValue struct {
	Name string
}

func (AttributeValue) authoringValueSource() {}

type BuiltinCall struct {
	Name       string
	Positional []Scalar
	Named      map[string]Scalar
}
