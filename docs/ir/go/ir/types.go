// Package ir defines the in-memory representation of Scraping KDL Validated IR 2026-07-15.
//
// The JSON Schema is the authoritative wire contract. Interfaces below model
// discriminated unions strongly; a production decoder must dispatch on each
// object's "kind" field when unmarshalling JSON.
package ir

import "encoding/json"

type JSONScalar = json.RawMessage

type SourcePosition struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type SourceSpan struct {
	File  string         `json:"file"`
	Start SourcePosition `json:"start"`
	End   SourcePosition `json:"end"`
}

type TypeRef interface {
	isTypeRef()
}

type PrimitiveTypeRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (PrimitiveTypeRef) isTypeRef() {}

type ArrayTypeRef struct {
	Kind    string  `json:"kind"`
	Element TypeRef `json:"element"`
}

func (ArrayTypeRef) isTypeRef() {}

type NullableTypeRef struct {
	Kind  string  `json:"kind"`
	Inner TypeRef `json:"inner"`
}

func (NullableTypeRef) isTypeRef() {}

type SourceFile struct {
	Path          string `json:"path"`
	ModuleName    string `json:"moduleName,omitempty"`
	ModuleVersion string `json:"moduleVersion,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

type Extractor struct {
	Kind            string       `json:"kind"`
	IRVersion       string       `json:"irVersion"`
	LanguageVersion string       `json:"languageVersion"`
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	Files           []SourceFile `json:"files"`
	Source          Source       `json:"source"`
	Inputs          []Input      `json:"inputs"`
	Transforms      []Transform  `json:"transforms"`
	Output          OutputObject `json:"output"`
	Capabilities    []string     `json:"capabilities"`
	Span            SourceSpan   `json:"span"`
}

type Input struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Default  *JSONScalar `json:"default,omitempty"`
	Span     SourceSpan  `json:"span"`
}

type Source struct {
	Kind          string         `json:"kind"`
	Fetch         Fetch          `json:"fetch"`
	SessionPolicy string         `json:"sessionPolicy"`
	Workflow      []WorkflowStep `json:"workflow"`
	Span          SourceSpan     `json:"span"`
}

type Fetch struct {
	Mode        string     `json:"mode"`
	URLTemplate Template   `json:"urlTemplate"`
	Span        SourceSpan `json:"span"`
}

type Template struct {
	Raw      string            `json:"raw"`
	Segments []TemplateSegment `json:"segments"`
}

type TemplateSegment interface {
	isTemplateSegment()
}

type LiteralTemplateSegment struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (LiteralTemplateSegment) isTemplateSegment() {}

type InputTemplateSegment struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (InputTemplateSegment) isTemplateSegment() {}

type WorkflowStep interface {
	isWorkflowStep()
}

type WaitForStep struct {
	Kind      string     `json:"kind"`
	Selector  string     `json:"selector"`
	State     string     `json:"state"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (WaitForStep) isWorkflowStep() {}

type ClickStep struct {
	Kind      string     `json:"kind"`
	Selector  string     `json:"selector"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (ClickStep) isWorkflowStep() {}

type FillStep struct {
	Kind      string     `json:"kind"`
	Selector  string     `json:"selector"`
	Value     string     `json:"value"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (FillStep) isWorkflowStep() {}

type PressStep struct {
	Kind      string     `json:"kind"`
	Selector  string     `json:"selector"`
	Key       string     `json:"key"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (PressStep) isWorkflowStep() {}

type ScrollStep struct {
	Kind string     `json:"kind"`
	X    float64    `json:"x"`
	Y    float64    `json:"y"`
	Span SourceSpan `json:"span"`
}

func (ScrollStep) isWorkflowStep() {}

type NetworkIdleStep struct {
	Kind      string     `json:"kind"`
	IdleMS    int        `json:"idleMs"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (NetworkIdleStep) isWorkflowStep() {}

type EvaluateJavaScriptStep struct {
	Kind      string     `json:"kind"`
	Source    string     `json:"source"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (EvaluateJavaScriptStep) isWorkflowStep() {}

type OutputObject struct {
	Kind    string         `json:"kind"`
	Members []OutputMember `json:"members"`
}

type OutputMember interface {
	isOutputMember()
}

type Field struct {
	Kind           string          `json:"kind"`
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	SuccessfulType TypeRef         `json:"successfulType"`
	EffectiveType  TypeRef         `json:"effectiveType"`
	Required       bool            `json:"required"`
	Default        *JSONScalar     `json:"default,omitempty"`
	Selection      *FieldSelection `json:"selection,omitempty"`
	ValueSource    ValueSource     `json:"valueSource"`
	Transforms     []TransformCall `json:"transforms"`
	OnError        string          `json:"onError"`
	Span           SourceSpan      `json:"span"`
}

func (Field) isOutputMember() {}

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
	Span       SourceSpan   `json:"span"`
}

func (Collection) isOutputMember() {}

type FieldSelection struct {
	Selector string     `json:"selector"`
	Match    string     `json:"match"`
	Span     SourceSpan `json:"span"`
}

type ValueSource interface {
	isValueSource()
}

type TextValueSource struct {
	Kind    string     `json:"kind"`
	RawType TypeRef    `json:"rawType"`
	Span    SourceSpan `json:"span"`
}

func (TextValueSource) isValueSource() {}

type HTMLValueSource struct {
	Kind    string     `json:"kind"`
	RawType TypeRef    `json:"rawType"`
	Span    SourceSpan `json:"span"`
}

func (HTMLValueSource) isValueSource() {}

type AttributeValueSource struct {
	Kind    string     `json:"kind"`
	Name    string     `json:"name"`
	RawType TypeRef    `json:"rawType"`
	Span    SourceSpan `json:"span"`
}

func (AttributeValueSource) isValueSource() {}

type JavaScriptValueSource struct {
	Kind      string     `json:"kind"`
	Scope     string     `json:"scope"`
	Source    string     `json:"source"`
	Returns   TypeRef    `json:"returns"`
	TimeoutMS *int       `json:"timeoutMs,omitempty"`
	Span      SourceSpan `json:"span"`
}

func (JavaScriptValueSource) isValueSource() {}

type Transform interface {
	isTransform()
}

type TransformBase struct {
	SymbolID string     `json:"symbolId"`
	Name     string     `json:"name"`
	Origin   string     `json:"origin"`
	Input    TypeRef    `json:"input"`
	Output   TypeRef    `json:"output"`
	Span     SourceSpan `json:"span"`
}

type PipelineTransform struct {
	Kind string `json:"kind"`
	TransformBase
	Calls []TransformCall `json:"calls"`
}

func (PipelineTransform) isTransform() {}

type MatchTransform struct {
	Kind string `json:"kind"`
	TransformBase
	Cases   []MatchCase `json:"cases"`
	Default JSONScalar  `json:"default"`
}

func (MatchTransform) isTransform() {}

type ExternalTransform struct {
	Kind string `json:"kind"`
	TransformBase
	Symbol string `json:"symbol"`
}

func (ExternalTransform) isTransform() {}

type MatchCase struct {
	When JSONScalar `json:"when"`
	Then JSONScalar `json:"then"`
	Span SourceSpan `json:"span"`
}

type TransformCall struct {
	Target              TransformTarget `json:"target"`
	PositionalArguments []JSONScalar    `json:"positionalArguments"`
	NamedArguments      []NamedArgument `json:"namedArguments"`
	Input               TypeRef         `json:"input"`
	Output              TypeRef         `json:"output"`
	Span                SourceSpan      `json:"span"`
}

type TransformTarget interface {
	isTransformTarget()
}

type BuiltinTransformTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (BuiltinTransformTarget) isTransformTarget() {}

type DeclaredTransformTarget struct {
	Kind     string `json:"kind"`
	SymbolID string `json:"symbolId"`
}

func (DeclaredTransformTarget) isTransformTarget() {}

type NamedArgument struct {
	Name  string     `json:"name"`
	Value JSONScalar `json:"value"`
}

type Diagnostic struct {
	Code     string      `json:"code"`
	Severity string      `json:"severity"`
	Message  string      `json:"message"`
	Path     string      `json:"path,omitempty"`
	Span     *SourceSpan `json:"span,omitempty"`
}
