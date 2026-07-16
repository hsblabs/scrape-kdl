export type IRVersion = "2026-07-15";
export type LanguageVersion = "2026-07-15";

export interface SourcePosition {
  readonly offset: number;
  readonly line: number;
  readonly column: number;
}

export interface SourceSpan {
  readonly file: string;
  readonly start: SourcePosition;
  readonly end: SourcePosition;
}

export type JsonScalar = string | number | boolean | null;
export type JsonValue = JsonScalar | readonly JsonValue[] | JsonObject;
export interface JsonObject {
  readonly [key: string]: JsonValue;
}

export type PrimitiveTypeName =
  | "string"
  | "bool"
  | "int"
  | "u8"
  | "u16"
  | "u32"
  | "u64"
  | "i8"
  | "i16"
  | "i32"
  | "i64"
  | "float"
  | "f32"
  | "f64"
  | "object"
  | "unknown";

export type TypeRef = PrimitiveTypeRef | ArrayTypeRef | NullableTypeRef;

export interface PrimitiveTypeRef {
  readonly kind: "primitive";
  readonly name: PrimitiveTypeName;
}

export interface ArrayTypeRef {
  readonly kind: "array";
  readonly element: TypeRef;
}

export interface NullableTypeRef {
  readonly kind: "nullable";
  readonly inner: TypeRef;
}

export interface SourceFileIR {
  readonly path: string;
  readonly moduleName?: string;
  readonly moduleVersion?: string;
  readonly sha256: string;
}

export interface ExtractorIR {
  readonly kind: "extractor";
  readonly irVersion: IRVersion;
  readonly languageVersion: LanguageVersion;
  readonly name: string;
  readonly version: string;
  readonly files: readonly SourceFileIR[];
  readonly source: SourceIR;
  readonly inputs: readonly InputIR[];
  readonly transforms: readonly TransformIR[];
  readonly output: OutputObjectIR;
  readonly capabilities: readonly string[];
  readonly span: SourceSpan;
}

export interface InputIR {
  readonly name: string;
  readonly type: "string" | "bool" | "int" | "float";
  readonly required: boolean;
  readonly default?: JsonScalar;
  readonly span: SourceSpan;
}

export interface SourceIR {
  readonly kind: "html";
  readonly fetch: FetchIR;
  readonly sessionPolicy: "none" | "optional" | "required";
  readonly workflow: readonly WorkflowStepIR[];
  readonly span: SourceSpan;
}

export interface FetchIR {
  readonly mode: "http" | "browser";
  readonly urlTemplate: TemplateIR;
  readonly span: SourceSpan;
}

export interface TemplateIR {
  readonly raw: string;
  readonly segments: readonly TemplateSegmentIR[];
}

export type TemplateSegmentIR = LiteralTemplateSegmentIR | InputTemplateSegmentIR;

export interface LiteralTemplateSegmentIR {
  readonly kind: "literal";
  readonly value: string;
}

export interface InputTemplateSegmentIR {
  readonly kind: "input";
  readonly name: string;
}

export type WorkflowStepIR =
  WaitForStepIR | ClickStepIR | FillStepIR | PressStepIR | ScrollStepIR | NetworkIdleStepIR | EvaluateJavaScriptStepIR;

interface WorkflowStepBase {
  readonly span: SourceSpan;
}

export interface WaitForStepIR extends WorkflowStepBase {
  readonly kind: "wait-for";
  readonly selector: string;
  readonly state: "attached" | "visible" | "hidden" | "detached";
  readonly timeoutMs?: number;
}

export interface ClickStepIR extends WorkflowStepBase {
  readonly kind: "click";
  readonly selector: string;
  readonly timeoutMs?: number;
}

export interface FillStepIR extends WorkflowStepBase {
  readonly kind: "fill";
  readonly selector: string;
  readonly value: string;
  readonly timeoutMs?: number;
}

export interface PressStepIR extends WorkflowStepBase {
  readonly kind: "press";
  readonly selector: string;
  readonly key: string;
  readonly timeoutMs?: number;
}

export interface ScrollStepIR extends WorkflowStepBase {
  readonly kind: "scroll";
  readonly x: number;
  readonly y: number;
}

export interface NetworkIdleStepIR extends WorkflowStepBase {
  readonly kind: "wait-for-network-idle";
  readonly idleMs: number;
  readonly timeoutMs?: number;
}

export interface EvaluateJavaScriptStepIR extends WorkflowStepBase {
  readonly kind: "evaluate-js";
  readonly source: string;
  readonly timeoutMs?: number;
}

export interface OutputObjectIR {
  readonly kind: "object";
  readonly members: readonly OutputMemberIR[];
}

export type OutputMemberIR = FieldIR | CollectionIR;

export interface FieldIR {
  readonly kind: "field";
  readonly id: string;
  readonly name: string;
  readonly successfulType: TypeRef;
  readonly effectiveType: TypeRef;
  readonly required: boolean;
  readonly default?: JsonScalar;
  readonly selection?: FieldSelectionIR;
  readonly valueSource: ValueSourceIR;
  readonly transforms: readonly ResolvedTransformCallIR[];
  readonly onError: "fail" | "null" | "warn" | "default";
  readonly span: SourceSpan;
}

export interface CollectionIR {
  readonly kind: "collection";
  readonly id: string;
  readonly name: string;
  readonly selector: string;
  readonly required: boolean;
  readonly minItems: number;
  readonly maxItems?: number;
  readonly onRowError: "fail" | "skip";
  readonly row: OutputObjectIR;
  readonly span: SourceSpan;
}

export interface FieldSelectionIR {
  readonly selector: string;
  readonly match: "one" | "first";
  readonly span: SourceSpan;
}

export type ValueSourceIR = TextValueSourceIR | HtmlValueSourceIR | AttributeValueSourceIR | JavaScriptValueSourceIR;

export interface TextValueSourceIR {
  readonly kind: "text";
  readonly rawType: PrimitiveTypeRef;
  readonly span: SourceSpan;
}

export interface HtmlValueSourceIR {
  readonly kind: "html";
  readonly rawType: PrimitiveTypeRef;
  readonly span: SourceSpan;
}

export interface AttributeValueSourceIR {
  readonly kind: "attribute";
  readonly name: string;
  /** Successful raw type. Attribute absence is handled as missing, not null pipeline input. */
  readonly rawType: PrimitiveTypeRef;
  readonly span: SourceSpan;
}

export interface JavaScriptValueSourceIR {
  readonly kind: "javascript";
  readonly scope: "document" | "current";
  readonly source: string;
  readonly returns: TypeRef;
  readonly timeoutMs?: number;
  readonly span: SourceSpan;
}

export type TransformIR = PipelineTransformIR | MatchTransformIR | ExternalTransformIR;

interface TransformBaseIR {
  readonly symbolId: string;
  readonly name: string;
  readonly origin: "local" | "imported";
  readonly input: TypeRef;
  readonly output: TypeRef;
  readonly span: SourceSpan;
}

export interface PipelineTransformIR extends TransformBaseIR {
  readonly kind: "pipeline";
  readonly calls: readonly ResolvedTransformCallIR[];
}

export interface MatchTransformIR extends TransformBaseIR {
  readonly kind: "match";
  readonly cases: readonly MatchCaseIR[];
  readonly default: JsonScalar;
}

export interface MatchCaseIR {
  readonly when: JsonScalar;
  readonly then: JsonScalar;
  readonly span: SourceSpan;
}

export interface ExternalTransformIR extends TransformBaseIR {
  readonly kind: "external";
  readonly symbol: string;
}

export interface ResolvedTransformCallIR {
  readonly target: TransformTargetIR;
  readonly positionalArguments: readonly JsonScalar[];
  readonly namedArguments: readonly NamedArgumentIR[];
  readonly input: TypeRef;
  readonly output: TypeRef;
  readonly span: SourceSpan;
}

export type TransformTargetIR = BuiltinTransformTargetIR | DeclaredTransformTargetIR;

export interface BuiltinTransformTargetIR {
  readonly kind: "builtin";
  readonly name: string;
}

export interface DeclaredTransformTargetIR {
  readonly kind: "declared";
  readonly symbolId: string;
}

export interface NamedArgumentIR {
  readonly name: string;
  readonly value: JsonScalar;
}

export interface DiagnosticIR {
  readonly code: string;
  readonly severity: "error" | "warning" | "info";
  readonly message: string;
  readonly path?: string;
  readonly span?: SourceSpan;
}
