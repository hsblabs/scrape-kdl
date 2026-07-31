import type { FetchMode, SessionPolicy } from "./index.js";

export type AuthoringScalar = string | boolean | number | null;
export type InputConstraint = "string" | "string-array" | "non-null-scalar" | "nullable" | "scalar" | "number";
export type OutputConstraint =
  | "string"
  | "nullable-string"
  | "string-array"
  | "bool"
  | "target-integer"
  | "target-float"
  | "inner-input"
  | "same-as-input";
export type NullabilityEffect = "preserved" | "introduced" | "removed";
export type ArgumentConstraint = "string" | "bool" | "int" | "non-negative-int" | "number" | "scalar";

export interface NamedArgument {
  readonly name: string;
  readonly constraint: ArgumentConstraint;
  readonly required: boolean;
  readonly default?: AuthoringScalar;
}

export interface PositionalArguments {
  readonly constraint: "" | "same-as-input";
  readonly min: number;
  readonly max: number;
}

export interface BuiltinDefinition {
  readonly name: string;
  readonly input: InputConstraint;
  readonly output: OutputConstraint;
  readonly nullabilityEffect: NullabilityEffect;
  readonly namedArguments: readonly NamedArgument[];
  readonly positionalArguments: PositionalArguments;
}

export interface BuiltinCatalog {
  readonly languageVersion: string;
  readonly builtins: readonly BuiltinDefinition[];
}

export interface AuthoringDocument {
  readonly languageVersion: string;
  readonly extractor: AuthoringExtractor;
}

export interface AuthoringExtractor {
  readonly name: string;
  readonly version: string;
  readonly source: AuthoringSource;
  readonly inputs: readonly AuthoringInput[];
  readonly members: readonly AuthoringMember[];
}

export interface AuthoringSource {
  readonly fetchMode: FetchMode;
  readonly urlTemplate: string;
  readonly sessionPolicy: SessionPolicy;
}

export interface AuthoringInput {
  readonly name: string;
  readonly type: "string" | "bool" | "int" | "float";
  readonly required: boolean;
}

export type AuthoringMember = AuthoringField | AuthoringCollection;

export interface AuthoringField {
  readonly kind: "field";
  readonly name: string;
  readonly type: string;
  readonly required: boolean;
  readonly selector: string;
  readonly match: "one" | "first";
  readonly value: AuthoringValueSource;
  readonly transforms: readonly BuiltinCall[];
  readonly onError: "fail" | "null" | "warn";
}

export interface AuthoringCollection {
  readonly kind: "collection";
  readonly name: string;
  readonly selector: string;
  readonly required: boolean;
  readonly minItems: number;
  readonly maxItems?: number;
  readonly onRowError: "fail" | "skip";
  readonly members: readonly AuthoringMember[];
}

export type AuthoringValueSource =
  | { readonly kind: "text" }
  | { readonly kind: "html" }
  | { readonly kind: "attribute"; readonly name: string };

export interface BuiltinCall {
  readonly name: string;
  readonly positional: readonly AuthoringScalar[];
  readonly named: Readonly<Record<string, AuthoringScalar>>;
}

export declare function supportedBuiltinCatalogVersions(): readonly string[];
export declare function builtinCatalog(languageVersion: string): BuiltinCatalog;
export declare function callBuiltin(
  definition: BuiltinDefinition,
  positional?: readonly AuthoringScalar[],
  named?: Readonly<Record<string, AuthoringScalar>>,
): BuiltinCall;
export declare function write(document: AuthoringDocument): string;
