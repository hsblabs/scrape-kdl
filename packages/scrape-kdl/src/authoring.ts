import { BUILTIN_CONTRACT } from "./builtin-contract.js";
import type { FetchMode, SessionPolicy } from "./public-api.js";

const LANGUAGE_VERSION = "2026-07-15";

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
  { readonly kind: "text" } | { readonly kind: "html" } | { readonly kind: "attribute"; readonly name: string };

export interface BuiltinCall {
  readonly name: string;
  readonly positional: readonly AuthoringScalar[];
  readonly named: Readonly<Record<string, AuthoringScalar>>;
}

export function supportedBuiltinCatalogVersions(): readonly string[] {
  return Object.freeze([LANGUAGE_VERSION]);
}

export function builtinCatalog(languageVersion: string): BuiltinCatalog {
  if (languageVersion !== LANGUAGE_VERSION)
    throw new RangeError(`unsupported built-in catalog language version ${JSON.stringify(languageVersion)}`);
  return CATALOG;
}

export function callBuiltin(
  definition: BuiltinDefinition,
  positional: readonly AuthoringScalar[] = [],
  named: Readonly<Record<string, AuthoringScalar>> = {},
): BuiltinCall {
  assertRecord("built-in definition", definition);
  assertArray("built-in positional arguments", positional);
  assertRecord("built-in named arguments", named);
  return deepFreeze({ name: definition.name, positional: [...positional], named: { ...named } });
}

export function write(document: AuthoringDocument): string {
  assertRecord("authoring document", document);
  const catalog = builtinCatalog(document.languageVersion);
  const lines: string[] = [];
  const line = (depth: number, value: string): void => {
    lines.push(`${"  ".repeat(depth)}${value}`);
  };
  const writeCall = (depth: number, call: BuiltinCall): void => {
    assertRecord("built-in call", call);
    assertArray(`built-in ${JSON.stringify(call.name)} positional arguments`, call.positional);
    assertRecord(`built-in ${JSON.stringify(call.name)} named arguments`, call.named);
    const definition = lookupBuiltin(catalog, call.name);
    if (definition === undefined)
      throw new TypeError(
        `unknown built-in ${JSON.stringify(call.name)} for language version ${catalog.languageVersion}`,
      );
    const { min, max } = definition.positionalArguments;
    if (call.positional.length < min || (max >= 0 && call.positional.length > max))
      throw new TypeError(
        `built-in ${JSON.stringify(call.name)} accepts ${min}..${max < 0 ? "unbounded" : max} positional arguments, got ${call.positional.length}`,
      );
    const known = new Set(definition.namedArguments.map(({ name }) => name));
    const parts = [`apply ${quoteKDL(call.name)}`, ...call.positional.map((value) => scalarKDL(value))];
    for (const argument of definition.namedArguments) {
      const exists = Object.hasOwn(call.named, argument.name);
      if (argument.required && !exists)
        throw new TypeError(
          `built-in ${JSON.stringify(call.name)} requires named argument ${JSON.stringify(argument.name)}`,
        );
      if (!exists) continue;
      const value = call.named[argument.name];
      validateScalarConstraint(value, argument.constraint);
      parts.push(`${argument.name}=${scalarKDL(value)}`);
    }
    for (const name of Object.keys(call.named))
      if (!known.has(name))
        throw new TypeError(
          `built-in ${JSON.stringify(call.name)} does not accept named argument ${JSON.stringify(name)}`,
        );
    line(depth, parts.join(" "));
  };
  const writeMember = (depth: number, member: AuthoringMember): void => {
    assertRecord("authoring member", member);
    assertChoice("member kind", member.kind, ["field", "collection"]);
    if (member.kind === "field") {
      assertChoice(`field ${JSON.stringify(member.name)} match`, member.match, ["one", "first"]);
      assertChoice(`field ${JSON.stringify(member.name)} onError`, member.onError, ["fail", "null", "warn"]);
      assertRecord(`field ${JSON.stringify(member.name)} value`, member.value);
      assertChoice(`field ${JSON.stringify(member.name)} value kind`, member.value.kind, ["text", "html", "attribute"]);
      assertArray(`field ${JSON.stringify(member.name)} transforms`, member.transforms);
      line(
        depth,
        `field ${quoteKDL(member.name)} type=${quoteKDL(member.type)} required=${boolKDL(member.required)} {`,
      );
      line(depth + 1, `select ${quoteKDL(member.selector)} match=${quoteKDL(member.match)}`);
      if (member.value.kind === "attribute") line(depth + 1, `value "attr" name=${quoteKDL(member.value.name)}`);
      else line(depth + 1, `value ${quoteKDL(member.value.kind)}`);
      for (const call of member.transforms) writeCall(depth + 1, call);
      if (member.onError !== "fail") line(depth + 1, `on-error ${quoteKDL(member.onError)}`);
      line(depth, "}");
      return;
    }
    assertChoice(`collection ${JSON.stringify(member.name)} onRowError`, member.onRowError, ["fail", "skip"]);
    if (!Number.isSafeInteger(member.minItems) || member.minItems < 0)
      throw new TypeError(`collection ${JSON.stringify(member.name)} has invalid minItems`);
    if (member.maxItems !== undefined && (!Number.isSafeInteger(member.maxItems) || member.maxItems < 0))
      throw new TypeError(`collection ${JSON.stringify(member.name)} has invalid maxItems`);
    assertArray(`collection ${JSON.stringify(member.name)} members`, member.members);
    const maximum = member.maxItems === undefined ? "" : ` max-items=${member.maxItems}`;
    line(
      depth,
      `collection ${quoteKDL(member.name)} required=${boolKDL(member.required)} min-items=${member.minItems}${maximum} on-row-error=${quoteKDL(member.onRowError)} {`,
    );
    line(depth + 1, `select ${quoteKDL(member.selector)}`);
    for (const child of member.members) writeMember(depth + 1, child);
    line(depth, "}");
  };

  const extractor = document.extractor;
  assertRecord("authoring extractor", extractor);
  assertRecord("authoring source", extractor.source);
  assertArray("authoring inputs", extractor.inputs);
  assertArray("authoring members", extractor.members);
  assertChoice("source fetchMode", extractor.source.fetchMode, ["http", "browser"]);
  assertChoice("source sessionPolicy", extractor.source.sessionPolicy, ["none", "optional", "required"]);
  line(
    0,
    `extractor ${quoteKDL(extractor.name)} version=${quoteKDL(extractor.version)} language-version=${quoteKDL(document.languageVersion)} {`,
  );
  line(1, 'source "html" {');
  line(2, `fetch mode=${quoteKDL(extractor.source.fetchMode)} url=${quoteKDL(extractor.source.urlTemplate)}`);
  line(2, `session policy=${quoteKDL(extractor.source.sessionPolicy)}`);
  line(1, "}");
  for (const input of extractor.inputs) {
    assertRecord("authoring input", input);
    assertChoice(`input ${JSON.stringify(input.name)} type`, input.type, ["string", "bool", "int", "float"]);
    line(1, `input ${quoteKDL(input.name)} type=${quoteKDL(input.type)} required=${boolKDL(input.required)}`);
  }
  for (const member of extractor.members) writeMember(1, member);
  line(0, "}");
  return `${lines.join("\n")}\n`;
}

interface TransformMetadata {
  readonly input: InputConstraint;
  readonly output: OutputConstraint;
  readonly nullabilityEffect: NullabilityEffect;
  readonly defaults?: Readonly<Record<string, AuthoringScalar>>;
  readonly positionalConstraint?: "same-as-input";
}

const stringToString = (): TransformMetadata => ({
  input: "string",
  output: "string",
  nullabilityEffect: "preserved",
});
const nullableString = (defaults?: Readonly<Record<string, AuthoringScalar>>): TransformMetadata => {
  const metadata = { input: "string", output: "nullable-string", nullabilityEffect: "introduced" } as const;
  return defaults === undefined ? metadata : { ...metadata, defaults };
};

const METADATA: Readonly<Record<string, TransformMetadata>> = {
  trim: stringToString(),
  "normalize-whitespace": stringToString(),
  lowercase: stringToString(),
  uppercase: stringToString(),
  replace: stringToString(),
  "regex-replace": { ...stringToString(), defaults: { flags: "" } },
  "regex-capture": nullableString({ group: 0, flags: "" }),
  substring: stringToString(),
  split: { input: "string", output: "string-array", nullabilityEffect: "preserved" },
  join: { input: "string-array", output: "string", nullabilityEffect: "preserved" },
  prepend: stringToString(),
  append: stringToString(),
  "parse-int": {
    input: "string",
    output: "target-integer",
    nullabilityEffect: "preserved",
    defaults: { radix: 10 },
  },
  "parse-float": { input: "string", output: "target-float", nullabilityEffect: "preserved" },
  "parse-bool": {
    input: "string",
    output: "bool",
    nullabilityEffect: "preserved",
    defaults: { "case-sensitive": false, true: "true", false: "false" },
  },
  "to-string": { input: "non-null-scalar", output: "string", nullabilityEffect: "preserved" },
  "empty-to-null": nullableString(),
  coalesce: { input: "nullable", output: "inner-input", nullabilityEffect: "removed" },
  "url-resolve": stringToString(),
  "url-query": nullableString({ index: 0 }),
  "url-path": stringToString(),
  "path-segment": nullableString(),
  "assert-matches": { ...stringToString(), defaults: { flags: "" } },
  "assert-enum": {
    input: "scalar",
    output: "same-as-input",
    nullabilityEffect: "preserved",
    positionalConstraint: "same-as-input",
  },
  "assert-min": { input: "number", output: "same-as-input", nullabilityEffect: "preserved" },
  "assert-max": { input: "number", output: "same-as-input", nullabilityEffect: "preserved" },
};

function buildCatalog(): BuiltinCatalog {
  const contractNames = Object.keys(BUILTIN_CONTRACT).sort();
  const metadataNames = Object.keys(METADATA).sort();
  if (JSON.stringify(contractNames) !== JSON.stringify(metadataNames))
    throw new Error("built-in catalog metadata does not cover the compiler registry");
  const builtins = contractNames.map((name): BuiltinDefinition => {
    const contract = BUILTIN_CONTRACT[name];
    const metadata = METADATA[name];
    if (contract === undefined || metadata === undefined) throw new Error(`missing built-in metadata for ${name}`);
    const namedArguments = Object.keys(contract.properties)
      .sort()
      .map((argument): NamedArgument => {
        const constraint = contract.properties[argument];
        if (constraint === undefined) throw new Error(`missing argument constraint for ${name}.${argument}`);
        const base = { name: argument, constraint, required: contract.required.includes(argument) };
        return Object.hasOwn(metadata.defaults ?? {}, argument)
          ? { ...base, default: metadata.defaults?.[argument] as AuthoringScalar }
          : base;
      });
    return {
      name,
      input: metadata.input,
      output: metadata.output,
      nullabilityEffect: metadata.nullabilityEffect,
      namedArguments,
      positionalArguments: {
        constraint: metadata.positionalConstraint ?? "",
        min: contract.minPositional,
        max: contract.maxPositional,
      },
    };
  });
  return { languageVersion: LANGUAGE_VERSION, builtins };
}

function lookupBuiltin(catalog: BuiltinCatalog, name: string): BuiltinDefinition | undefined {
  return catalog.builtins.find((definition) => definition.name === name);
}

function validateScalarConstraint(value: AuthoringScalar, constraint: ArgumentConstraint): void {
  assertScalar("value", value);
  if (typeof value === "number" && !Number.isFinite(value)) throw new TypeError("numeric value must be finite");
  if (constraint === "string" && typeof value !== "string") throw new TypeError("value must be string");
  if (constraint === "bool" && typeof value !== "boolean") throw new TypeError("value must be bool");
  if (constraint === "int" && !Number.isSafeInteger(value)) throw new TypeError("value must be int");
  if (constraint === "non-negative-int" && (!Number.isSafeInteger(value) || Number(value) < 0))
    throw new TypeError("value must be a non-negative int");
  if (constraint === "number" && typeof value !== "number") throw new TypeError("value must be a number");
}

function assertChoice(label: string, value: string, choices: readonly string[]): void {
  if (typeof value !== "string" || !choices.includes(value))
    throw new TypeError(`${label} must be one of ${choices.join(", ")}`);
}

function assertArray(label: string, value: unknown): asserts value is readonly unknown[] {
  if (!Array.isArray(value)) throw new TypeError(`${label} must be an array`);
}

function assertRecord(label: string, value: unknown): asserts value is Readonly<Record<string, unknown>> {
  if (value === null || typeof value !== "object" || Array.isArray(value))
    throw new TypeError(`${label} must be an object`);
}

function assertScalar(label: string, value: unknown): asserts value is AuthoringScalar {
  if (value !== null && typeof value !== "string" && typeof value !== "boolean" && typeof value !== "number")
    throw new TypeError(`${label} must be a scalar`);
}

function scalarKDL(value: AuthoringScalar): string {
  assertScalar("value", value);
  if (value === null) return "#null";
  if (typeof value === "string") return quoteKDL(value);
  if (typeof value === "boolean") return boolKDL(value);
  if (!Number.isFinite(value)) throw new TypeError("numeric value must be finite");
  return Object.is(value, -0) ? "-0.0" : String(value);
}

function boolKDL(value: boolean): string {
  if (typeof value !== "boolean") throw new TypeError("boolean value must be bool");
  return value ? "#true" : "#false";
}

function quoteKDL(value: string): string {
  if (typeof value !== "string") throw new TypeError("KDL string value must be string");
  let output = '"';
  for (const character of value) {
    const codePoint = character.codePointAt(0);
    if (codePoint === undefined || (codePoint >= 0xd800 && codePoint <= 0xdfff))
      throw new TypeError("string contains an unpaired surrogate");
    if (character === '"') output += '\\"';
    else if (character === "\\") output += "\\\\";
    else if (character === "\b") output += "\\b";
    else if (character === "\f") output += "\\f";
    else if (character === "\n") output += "\\n";
    else if (character === "\r") output += "\\r";
    else if (character === "\t") output += "\\t";
    else if (codePoint < 0x20 || codePoint === 0x7f) output += `\\u{${codePoint.toString(16)}}`;
    else output += character;
  }
  return `${output}"`;
}

function deepFreeze<T>(value: T): T {
  if (value !== null && typeof value === "object" && !Object.isFrozen(value)) {
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
  }
  return value;
}

const CATALOG = deepFreeze(buildCatalog());
