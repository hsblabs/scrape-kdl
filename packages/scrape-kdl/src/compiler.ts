import type { CompileOptions, Source } from "./public-api.js";
import type { Node, Property, Span } from "./parser.js";
import { loadSourceGraph } from "./source-loader.js";
import type {
  CollectionIR, ExtractorIR, FieldIR, InputIR, OutputObjectIR, PrimitiveTypeName,
  SourceIR, TemplateSegmentIR, TransformCallIR, TypeRef,
} from "./ir.js";

const LANGUAGE_VERSION = "2026-07-15";
const IR_VERSION = "2026-07-15";

export interface Diagnostic {
  readonly code: string;
  readonly severity: "error" | "warning";
  readonly message: string;
  readonly span: Span;
  readonly path?: string;
}

export interface ContractSliceResult {
  readonly ir?: ExtractorIR;
  readonly diagnostics: readonly Diagnostic[];
}

export async function compileContractSlice(source: Source, options: CompileOptions = {}): Promise<ContractSliceResult> {
  const graph = await loadSourceGraph(source, options);
  const diagnostics: Diagnostic[] = [...graph.diagnostics];
  if (graph.entry?.root === undefined || graph.entry.kind !== "extractor" || diagnostics.some((item) => item.severity === "error")) {
    return { diagnostics: sortedDiagnostics(diagnostics) };
  }
  const bytes = graph.entry.bytes;
  const path = graph.entry.displayPath;
  const root = graph.entry.root;
  validateProperties(root, diagnostics, new Set(["version", "language-version"]), "extractor");
  const name = stringArgument(root, 0);
  if (name === undefined || !/^[a-z][a-z0-9-]*$/u.test(name)) {
    diagnostics.push(diagnostic("E_IDENTIFIER_INVALID", `invalid extractor name ${JSON.stringify(name ?? "")}`, root.span, "extractor"));
  }
  const version = validateDocumentVersion(root, diagnostics);
  const languageVersion = validateLanguageVersion(root, diagnostics);
  if (diagnostics.some((item) => item.severity === "error")) return { diagnostics: sortedDiagnostics(diagnostics) };

  const sourceNodes = childNodes(root, "source");
  if (sourceNodes.length !== 1) {
    diagnostics.push(diagnostic("E_DOCUMENT_ROOT", `extractor requires exactly one source; found ${sourceNodes.length}`, root.span, "source"));
    return { diagnostics: sortedDiagnostics(diagnostics) };
  }
  const inputNodes = childNodes(root, "input");
  const outputNodes = root.children.filter((child) => child.name === "field" || child.name === "collection");
  const inputs = inputNodes.map((node) => compileInput(node, diagnostics));
  const sourceIR = compileSource(sourceNodes[0]!, diagnostics);
  const output = compileOutput(outputNodes, "output", diagnostics);
  for (const child of root.children) {
    if (!["source", "input", "field", "collection"].includes(child.name)) {
      diagnostics.push(diagnostic("E_UNKNOWN_NODE", `node ${JSON.stringify(child.name)} is outside the contract slice`, child.span, "output"));
    }
  }
  if (diagnostics.some((item) => item.severity === "error")) return { diagnostics: sortedDiagnostics(diagnostics) };

  const digest = await crypto.subtle.digest("SHA-256", bytes);
  const sha256 = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
  return {
    diagnostics: sortedDiagnostics(diagnostics),
    ir: {
      kind: "extractor", irVersion: IR_VERSION, languageVersion: languageVersion as typeof LANGUAGE_VERSION,
      name: name!, version: version!, files: [{ path, sha256 }], source: sourceIR!, inputs: inputs as InputIR[],
      transforms: [], output, capabilities: [sourceIR!.fetch.mode === "http" ? "http.fetch" : "browser.navigate"], span: root.span,
    },
  };
}

export async function validateContractSlice(source: Source): Promise<readonly Diagnostic[]> {
  return (await compileContractSlice(source)).diagnostics;
}

export function supportedLanguageVersions(): readonly string[] { return [LANGUAGE_VERSION]; }
export function supportedIRVersions(): readonly string[] { return [IR_VERSION]; }

function validateDocumentVersion(root: Node, diagnostics: Diagnostic[]): string | undefined {
  const property = propertyValue(root, "version");
  if (property === undefined) {
    diagnostics.push(diagnostic("E_DOCUMENT_VERSION_REQUIRED", "document requires version", root.span, "extractor.version"));
    return undefined;
  }
  if (property.value.kind !== "string" || !isDateIdentifier(String(property.value.value))) {
    diagnostics.push(diagnostic("E_DOCUMENT_VERSION_INVALID", "version must be a real calendar date in YYYY-MM-DD form", property.value.span, "extractor.version"));
    return undefined;
  }
  return String(property.value.value);
}

function validateLanguageVersion(root: Node, diagnostics: Diagnostic[]): string | undefined {
  const property = propertyValue(root, "language-version");
  if (property === undefined) {
    diagnostics.push(diagnostic("E_LANGUAGE_VERSION_REQUIRED", "document requires language-version", root.span, "extractor.language-version"));
    return undefined;
  }
  const value = String(property.value.value);
  if (property.value.kind !== "string" || !isDateIdentifier(value)) {
    diagnostics.push(diagnostic("E_LANGUAGE_VERSION_INVALID", "language-version must be a real calendar date in YYYY-MM-DD form", property.value.span, "extractor.language-version"));
    return undefined;
  }
  if (value !== LANGUAGE_VERSION) {
    diagnostics.push(diagnostic("E_LANGUAGE_VERSION_UNSUPPORTED", `unsupported language-version ${JSON.stringify(value)}`, property.value.span, "extractor.language-version"));
    return undefined;
  }
  return value;
}

function compileInput(node: Node, diagnostics: Diagnostic[]): InputIR | undefined {
  validateProperties(node, diagnostics, new Set(["type", "required", "default"]), "inputs");
  const name = stringArgument(node, 0);
  const type = stringProperty(node, "type") as InputIR["type"] | undefined;
  if (name === undefined || !/^[a-z][a-z0-9_]*$/u.test(name)) diagnostics.push(diagnostic("E_IDENTIFIER_INVALID", "invalid input name", node.span, "inputs"));
  if (type === undefined || !["string", "bool", "int", "float"].includes(type)) diagnostics.push(diagnostic("E_TYPE_UNKNOWN", "input type must be string, bool, int, or float", node.span, `inputs.${name ?? ""}`));
  const required = booleanProperty(node, "required") ?? false;
  const defaultProperty = propertyValue(node, "default");
  if (required && defaultProperty !== undefined) diagnostics.push(diagnostic("E_INPUT_REQUIRED_DEFAULT", "required input must not declare default", defaultProperty.value.span, `inputs.${name ?? ""}`));
  if (name === undefined || type === undefined) return undefined;
  const input: InputIR = { name, type, required, span: node.span };
  return defaultProperty === undefined ? input : { ...input, default: defaultProperty.value.value };
}

function compileSource(node: Node, diagnostics: Diagnostic[]): SourceIR | undefined {
  const fetchNodes = childNodes(node, "fetch");
  if (fetchNodes.length !== 1) {
    diagnostics.push(diagnostic("E_DOCUMENT_ROOT", `source requires exactly one fetch; found ${fetchNodes.length}`, node.span, "source.fetch"));
    return undefined;
  }
  const fetch = fetchNodes[0]!;
  validateProperties(fetch, diagnostics, new Set(["mode", "url"]), "source.fetch");
  const mode = stringProperty(fetch, "mode");
  const raw = stringProperty(fetch, "url");
  if (mode !== "http") diagnostics.push(diagnostic("E_BROWSER_CAPABILITY_REQUIRED", "contract slice requires HTTP fetch mode", fetch.span, "source.fetch"));
  if (raw === undefined || raw === "") diagnostics.push(diagnostic("E_TEMPLATE_INVALID", "fetch requires non-empty string url", fetch.span, "source.fetch"));
  const segments: TemplateSegmentIR[] = [];
  if (raw !== undefined) {
    let cursor = 0;
    for (const match of raw.matchAll(/\{([a-z][a-z0-9_]*)\}/gu)) {
      const index = match.index;
      if (index > cursor) segments.push({ kind: "literal", value: raw.slice(cursor, index) });
      segments.push({ kind: "input", name: match[1]! });
      cursor = index + match[0].length;
    }
    if (cursor < raw.length) segments.push({ kind: "literal", value: raw.slice(cursor) });
  }
  if (mode !== "http" || raw === undefined) return undefined;
  return {
    kind: "html", fetch: { mode, urlTemplate: { raw, segments }, span: fetch.span },
    sessionPolicy: "none", workflow: [], span: node.span,
  };
}

function compileOutput(nodes: readonly Node[], path: string, diagnostics: Diagnostic[]): OutputObjectIR {
  return {
    kind: "object",
    members: nodes.map((node) => node.name === "field" ? compileField(node, `${path}.${stringArgument(node, 0) ?? ""}`, diagnostics) : compileCollection(node, `${path}.${stringArgument(node, 0) ?? ""}`, diagnostics)).filter((value): value is FieldIR | CollectionIR => value !== undefined),
  };
}

function compileField(node: Node, path: string, diagnostics: Diagnostic[]): FieldIR | undefined {
  validateProperties(node, diagnostics, new Set(["type", "required", "default"]), path);
  const name = stringArgument(node, 0);
  const typeName = stringProperty(node, "type");
  const successfulType = typeName === undefined ? undefined : parseType(typeName);
  const required = booleanProperty(node, "required") ?? false;
  const select = childNodes(node, "select")[0];
  const value = childNodes(node, "value")[0];
  if (name === undefined || successfulType === undefined || select === undefined || value === undefined) {
    diagnostics.push(diagnostic("E_VALUE_SOURCE_MISSING", "contract slice field is incomplete", node.span, path));
    return undefined;
  }
  const selector = stringArgument(select, 0);
  const valueKind = stringArgument(value, 0);
  if (selector === undefined || !["text", "html", "attr"].includes(valueKind ?? "")) {
    diagnostics.push(diagnostic("E_VALUE_SOURCE_MISSING", "contract slice field source is invalid", node.span, path));
    return undefined;
  }
  const defaultProperty = propertyValue(node, "default");
  const effectiveType = !required && defaultProperty === undefined && successfulType.kind !== "nullable"
    ? { kind: "nullable" as const, inner: successfulType }
    : successfulType;
  let current: TypeRef = { kind: "primitive", name: "string" };
  const transforms: TransformCallIR[] = [];
  for (const apply of childNodes(node, "apply")) {
    const compiled = compileBuiltinCall(apply, current, diagnostics, path);
    if (compiled !== undefined) {
      transforms.push(compiled);
      current = compiled.output;
    }
  }
  const field: FieldIR = {
    kind: "field", id: path, name, successfulType, effectiveType, required,
    selection: { selector, match: (stringProperty(select, "match") ?? "one") as "one" | "first", span: select.span },
    valueSource: valueKind === "attr"
      ? { kind: "attribute", name: stringProperty(value, "name") ?? "", rawType: { kind: "primitive", name: "string" }, span: value.span }
      : { kind: valueKind as "text" | "html", rawType: { kind: "primitive", name: "string" }, span: value.span },
    transforms, onError: "fail", span: node.span,
  };
  return defaultProperty === undefined ? field : { ...field, default: defaultProperty.value.value };
}

function compileCollection(node: Node, path: string, diagnostics: Diagnostic[]): CollectionIR | undefined {
  validateProperties(node, diagnostics, new Set(["required", "min-items", "max-items", "on-row-error"]), path);
  const name = stringArgument(node, 0);
  const select = childNodes(node, "select")[0];
  const selector = select === undefined ? undefined : stringArgument(select, 0);
  if (name === undefined || selector === undefined) {
    diagnostics.push(diagnostic("E_SELECTOR_REQUIRED", "collection requires exactly one select", node.span, `${path}.selection`));
    return undefined;
  }
  const required = booleanProperty(node, "required") ?? false;
  const minItems = numberProperty(node, "min-items") ?? (required ? 1 : 0);
  const maxItems = numberProperty(node, "max-items");
  const rowNodes = node.children.filter((child) => child.name === "field" || child.name === "collection");
  const collection: CollectionIR = {
    kind: "collection", id: path, name, selector, required, minItems,
    onRowError: (stringProperty(node, "on-row-error") ?? "fail") as "fail" | "skip",
    row: compileOutput(rowNodes, `${path}[]`, diagnostics), span: node.span,
  };
  return maxItems === undefined ? collection : { ...collection, maxItems };
}

function compileBuiltinCall(node: Node, input: TypeRef, diagnostics: Diagnostic[], path: string): TransformCallIR | undefined {
  const name = stringArgument(node, 0);
  if (name === undefined) return undefined;
  let output: TypeRef;
  if (name === "trim" || name === "normalize-whitespace") output = { kind: "primitive", name: "string" };
  else if (name === "parse-int") output = { kind: "primitive", name: (stringProperty(node, "as") ?? "int") as PrimitiveTypeName };
  else {
    diagnostics.push(diagnostic("E_TRANSFORM_UNKNOWN", `unknown transform ${JSON.stringify(name)}`, node.span, path));
    return undefined;
  }
  return {
    target: { kind: "builtin", name }, positionalArguments: node.arguments.slice(1).map((value) => value.value),
    namedArguments: node.properties.map((property) => ({ name: property.name, value: property.value.value })),
    input, output, span: node.span,
  };
}

function parseType(raw: string): TypeRef | undefined {
  let remaining = raw;
  const match = /^[a-z0-9]+/u.exec(remaining);
  if (match === null) return undefined;
  let result: TypeRef = { kind: "primitive", name: match[0] as PrimitiveTypeName };
  remaining = remaining.slice(match[0].length);
  while (remaining !== "") {
    if (remaining.startsWith("[]")) {
      result = { kind: "array", element: result };
      remaining = remaining.slice(2);
    } else if (remaining.startsWith("?")) {
      result = { kind: "nullable", inner: result };
      remaining = remaining.slice(1);
    } else return undefined;
  }
  return result;
}

function validateProperties(node: Node, diagnostics: Diagnostic[], allowed: ReadonlySet<string>, path: string): void {
  const seen = new Set<string>();
  for (const property of node.properties) {
    if (seen.has(property.name)) diagnostics.push(diagnostic("E_DUPLICATE_PROPERTY", `duplicate property ${JSON.stringify(property.name)}`, property.span, path));
    seen.add(property.name);
    if (!allowed.has(property.name)) diagnostics.push(diagnostic("E_UNKNOWN_PROPERTY", `property ${JSON.stringify(property.name)} is not allowed`, property.span, path));
  }
}

function propertyValue(node: Node, name: string): Property | undefined { return node.properties.find((property) => property.name === name); }
function childNodes(node: Node, name: string): readonly Node[] { return node.children.filter((child) => child.name === name); }
function stringArgument(node: Node, index: number): string | undefined { const value = node.arguments[index]; return value?.kind === "string" ? String(value.value) : undefined; }
function stringProperty(node: Node, name: string): string | undefined { const value = propertyValue(node, name)?.value; return value?.kind === "string" ? String(value.value) : undefined; }
function booleanProperty(node: Node, name: string): boolean | undefined { const value = propertyValue(node, name)?.value; return value?.kind === "bool" ? Boolean(value.value) : undefined; }
function numberProperty(node: Node, name: string): number | undefined { const value = propertyValue(node, name)?.value; return value?.kind === "int" ? Number(value.value) : undefined; }

function isDateIdentifier(value: string): boolean {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/u.exec(value);
  if (match === null) return false;
  const year = Number(match[1]); const month = Number(match[2]); const day = Number(match[3]);
  const date = new Date(0);
  date.setUTCHours(0, 0, 0, 0);
  date.setUTCFullYear(year, month - 1, day);
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}

function diagnostic(code: string, message: string, span: Span, path: string): Diagnostic {
  const result: Diagnostic = { code, severity: "error", message, span, path };
  return path === "" ? { code, severity: "error", message, span } : result;
}

function sortedDiagnostics(diagnostics: readonly Diagnostic[]): readonly Diagnostic[] {
  return [...diagnostics].sort((left, right) => compareCodePoints(left.span.file, right.span.file) || left.span.start.offset - right.span.start.offset || compareCodePoints(left.code, right.code));
}

function compareCodePoints(left: string, right: string): number {
  const leftPoints = Array.from(left, (character) => character.codePointAt(0)!);
  const rightPoints = Array.from(right, (character) => character.codePointAt(0)!);
  const length = Math.min(leftPoints.length, rightPoints.length);
  for (let index = 0; index < length; index++) {
    if (leftPoints[index] !== rightPoints[index]) return leftPoints[index]! - rightPoints[index]!;
  }
  return leftPoints.length - rightPoints.length;
}
