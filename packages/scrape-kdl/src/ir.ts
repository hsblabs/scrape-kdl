import type { Span } from "./parser.js";

export type PrimitiveTypeName =
  | "string" | "bool" | "int" | "u8" | "u16" | "u32" | "u64"
  | "i8" | "i16" | "i32" | "i64" | "float" | "f32" | "f64"
  | "object" | "unknown";

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

export interface TemplateSegmentIR {
  readonly kind: "literal" | "input";
  readonly value?: string;
  readonly name?: string;
}

export interface TemplateIR {
  readonly raw: string;
  readonly segments: readonly TemplateSegmentIR[];
}

export interface FetchIR {
  readonly mode: "http" | "browser";
  readonly urlTemplate: TemplateIR;
  readonly span: Span;
}

export interface SourceIR {
  readonly kind: "html";
  readonly fetch: FetchIR;
  readonly sessionPolicy: "none" | "optional" | "required";
  readonly workflow: readonly never[];
  readonly span: Span;
}

export interface InputIR {
  readonly name: string;
  readonly type: "string" | "bool" | "int" | "float";
  readonly required: boolean;
  readonly default?: string | number | boolean | null;
  readonly span: Span;
}

export interface FieldSelectionIR {
  readonly selector: string;
  readonly match: "one" | "first";
  readonly span: Span;
}

export interface ValueSourceIR {
  readonly kind: "text" | "html" | "attribute";
  readonly name?: string;
  readonly rawType: PrimitiveTypeRef;
  readonly span: Span;
}

export interface TransformCallIR {
  readonly target: { readonly kind: "builtin"; readonly name: string };
  readonly positionalArguments: readonly (string | number | boolean | null)[];
  readonly namedArguments: readonly { readonly name: string; readonly value: string | number | boolean | null }[];
  readonly input: TypeRef;
  readonly output: TypeRef;
  readonly span: Span;
}

export interface FieldIR {
  readonly kind: "field";
  readonly id: string;
  readonly name: string;
  readonly successfulType: TypeRef;
  readonly effectiveType: TypeRef;
  readonly required: boolean;
  readonly default?: string | number | boolean | null;
  readonly selection?: FieldSelectionIR;
  readonly valueSource: ValueSourceIR;
  readonly transforms: readonly TransformCallIR[];
  readonly onError: "fail" | "null" | "warn" | "default";
  readonly span: Span;
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
  readonly span: Span;
}

export interface OutputObjectIR {
  readonly kind: "object";
  readonly members: readonly (FieldIR | CollectionIR)[];
}

export interface ExtractorIR {
  readonly kind: "extractor";
  readonly irVersion: "2026-07-15";
  readonly languageVersion: "2026-07-15";
  readonly name: string;
  readonly version: string;
  readonly files: readonly SourceFileIR[];
  readonly source: SourceIR;
  readonly inputs: readonly InputIR[];
  readonly transforms: readonly never[];
  readonly output: OutputObjectIR;
  readonly capabilities: readonly string[];
  readonly span: Span;
}
