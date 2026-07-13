package ir

import (
	"encoding/json"

	"github.com/hsblabs/scrape-kdl/internal/source"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

type SourceFile struct {
	Path          string `json:"path"`
	ModuleName    string `json:"moduleName,omitempty"`
	ModuleVersion int    `json:"moduleVersion,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

type Extractor struct {
	Kind            string       `json:"kind"`
	IRVersion       string       `json:"irVersion"`
	LanguageVersion string       `json:"languageVersion"`
	Name            string       `json:"name"`
	Version         int          `json:"version"`
	Files           []SourceFile `json:"files"`
	Source          Source       `json:"source"`
	Inputs          []Input      `json:"inputs"`
	Transforms      []Transform  `json:"transforms"`
	Output          OutputObject `json:"output"`
	Capabilities    []string     `json:"capabilities"`
	Span            source.Span  `json:"span"`
}

type Input struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Required bool             `json:"required"`
	Default  *json.RawMessage `json:"default,omitempty"`
	Span     source.Span      `json:"span"`
}

type Source struct {
	Kind          string         `json:"kind"`
	Fetch         Fetch          `json:"fetch"`
	SessionPolicy string         `json:"sessionPolicy"`
	Workflow      []WorkflowStep `json:"workflow"`
	Span          source.Span    `json:"span"`
}

type Fetch struct {
	Mode        string      `json:"mode"`
	URLTemplate Template    `json:"urlTemplate"`
	Span        source.Span `json:"span"`
}

type Template struct {
	Raw      string            `json:"raw"`
	Segments []TemplateSegment `json:"segments"`
}

type TemplateSegment interface{ templateSegment() }
type LiteralTemplateSegment struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (LiteralTemplateSegment) templateSegment() {}

type InputTemplateSegment struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (InputTemplateSegment) templateSegment() {}

type WorkflowStep interface{ workflowStep() }
type WaitForStep struct {
	Kind      string      `json:"kind"`
	Selector  string      `json:"selector"`
	State     string      `json:"state"`
	TimeoutMS *int        `json:"timeoutMs,omitempty"`
	Span      source.Span `json:"span"`
}

func (WaitForStep) workflowStep() {}

type ClickStep struct {
	Kind      string      `json:"kind"`
	Selector  string      `json:"selector"`
	TimeoutMS *int        `json:"timeoutMs,omitempty"`
	Span      source.Span `json:"span"`
}

func (ClickStep) workflowStep() {}

type FillStep struct {
	Kind      string      `json:"kind"`
	Selector  string      `json:"selector"`
	Value     string      `json:"value"`
	TimeoutMS *int        `json:"timeoutMs,omitempty"`
	Span      source.Span `json:"span"`
}

func (FillStep) workflowStep() {}

type PressStep struct {
	Kind      string      `json:"kind"`
	Selector  string      `json:"selector"`
	Key       string      `json:"key"`
	TimeoutMS *int        `json:"timeoutMs,omitempty"`
	Span      source.Span `json:"span"`
}

func (PressStep) workflowStep() {}

type ScrollStep struct {
	Kind string      `json:"kind"`
	X    float64     `json:"x"`
	Y    float64     `json:"y"`
	Span source.Span `json:"span"`
}

func (ScrollStep) workflowStep() {}

type NetworkIdleStep struct {
	Kind      string      `json:"kind"`
	IdleMS    int         `json:"idleMs"`
	TimeoutMS *int        `json:"timeoutMs,omitempty"`
	Span      source.Span `json:"span"`
}

func (NetworkIdleStep) workflowStep() {}

type EvaluateJavaScriptStep struct {
	Kind      string      `json:"kind"`
	Source    string      `json:"source"`
	TimeoutMS *int        `json:"timeoutMs,omitempty"`
	Span      source.Span `json:"span"`
}

func (EvaluateJavaScriptStep) workflowStep() {}

type OutputObject struct {
	Kind    string         `json:"kind"`
	Members []OutputMember `json:"members"`
}
type OutputMember interface{ outputMember() }

type Field struct {
	Kind           string           `json:"kind"`
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	SuccessfulType typesys.Type     `json:"successfulType"`
	EffectiveType  typesys.Type     `json:"effectiveType"`
	Required       bool             `json:"required"`
	Default        *json.RawMessage `json:"default,omitempty"`
	Selection      *FieldSelection  `json:"selection,omitempty"`
	ValueSource    ValueSource      `json:"valueSource"`
	Transforms     []TransformCall  `json:"transforms"`
	OnError        string           `json:"onError"`
	Span           source.Span      `json:"span"`
}

func (Field) outputMember() {}

type Collection struct {
	Kind       string       `json:"kind"`
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Selector   string       `json:"selector"`
	Required   bool         `json:"required"`
	MinItems   int          `json:"minItems"`
	MaxItems   *int         `json:"maxItems,omitempty"`
	OnRowError string       `json:"onRowError"`
	Row        OutputObject `json:"row"`
	Span       source.Span  `json:"span"`
}

func (Collection) outputMember() {}

type FieldSelection struct {
	Selector string      `json:"selector"`
	Match    string      `json:"match"`
	Span     source.Span `json:"span"`
}

type ValueSource interface{ valueSource() }
type TextValueSource struct {
	Kind    string       `json:"kind"`
	RawType typesys.Type `json:"rawType"`
	Span    source.Span  `json:"span"`
}

func (TextValueSource) valueSource() {}

type HTMLValueSource struct {
	Kind    string       `json:"kind"`
	RawType typesys.Type `json:"rawType"`
	Span    source.Span  `json:"span"`
}

func (HTMLValueSource) valueSource() {}

type AttributeValueSource struct {
	Kind    string       `json:"kind"`
	Name    string       `json:"name"`
	RawType typesys.Type `json:"rawType"`
	Span    source.Span  `json:"span"`
}

func (AttributeValueSource) valueSource() {}

type JavaScriptValueSource struct {
	Kind      string       `json:"kind"`
	Scope     string       `json:"scope"`
	Source    string       `json:"source"`
	Returns   typesys.Type `json:"returns"`
	TimeoutMS *int         `json:"timeoutMs,omitempty"`
	Span      source.Span  `json:"span"`
}

func (JavaScriptValueSource) valueSource() {}

type Transform interface{ transform() }
type TransformBase struct {
	SymbolID string       `json:"symbolId"`
	Name     string       `json:"name"`
	Origin   string       `json:"origin"`
	Input    typesys.Type `json:"input"`
	Output   typesys.Type `json:"output"`
	Span     source.Span  `json:"span"`
}
type PipelineTransform struct {
	Kind string `json:"kind"`
	TransformBase
	Calls []TransformCall `json:"calls"`
}

func (PipelineTransform) transform() {}

type MatchTransform struct {
	Kind string `json:"kind"`
	TransformBase
	Cases   []MatchCase     `json:"cases"`
	Default json.RawMessage `json:"default"`
}

func (MatchTransform) transform() {}

type ExternalTransform struct {
	Kind string `json:"kind"`
	TransformBase
	Symbol string `json:"symbol"`
}

func (ExternalTransform) transform() {}

type MatchCase struct {
	When json.RawMessage `json:"when"`
	Then json.RawMessage `json:"then"`
	Span source.Span     `json:"span"`
}

type TransformCall struct {
	Target              TransformTarget   `json:"target"`
	PositionalArguments []json.RawMessage `json:"positionalArguments"`
	NamedArguments      []NamedArgument   `json:"namedArguments"`
	Input               typesys.Type      `json:"input"`
	Output              typesys.Type      `json:"output"`
	Span                source.Span       `json:"span"`
}
type TransformTarget interface{ transformTarget() }
type BuiltinTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (BuiltinTarget) transformTarget() {}

type DeclaredTarget struct {
	Kind     string `json:"kind"`
	SymbolID string `json:"symbolId"`
}

func (DeclaredTarget) transformTarget() {}

type NamedArgument struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}
