import type { CompileOptions, Source } from "./public-api.js";
import { parse, type Document, type Node, type Property, type Span, type Value } from "./parser.js";

export interface LoadedDocument {
  readonly path: string;
  readonly displayPath: string;
  readonly bytes: Uint8Array<ArrayBuffer>;
  readonly document: Document;
  readonly kind?: "extractor" | "module";
  readonly root?: Node;
  readonly imports: ReadonlyMap<string, LoadedDocument | undefined>;
}

export interface SourceGraph {
  readonly entry?: LoadedDocument;
  readonly documents: ReadonlyMap<string, LoadedDocument>;
  readonly diagnostics: readonly LoadDiagnostic[];
}

export interface LoadDiagnostic {
  readonly code: string;
  readonly severity: "error";
  readonly message: string;
  readonly span: Span;
  readonly path?: string;
}

interface MutableLoadedDocument {
  readonly path: string;
  readonly displayPath: string;
  readonly bytes: Uint8Array<ArrayBuffer>;
  readonly document: Document;
  kind?: "extractor" | "module";
  root?: Node;
  readonly imports: Map<string, MutableLoadedDocument | undefined>;
}

export async function loadSourceGraph(source: Source, options: CompileOptions = {}): Promise<SourceGraph> {
  options.signal?.throwIfAborted();
  const entryPath = cleanPath(source.path);
  const entryDirectory = dirname(entryPath);
  const initialBytes = copyBytes(source.data);
  const documents = new Map<string, MutableLoadedDocument>();
  const loading = new Set<string>();
  const diagnostics: LoadDiagnostic[] = [];

  const displayPath = (path: string): string => relativePath(entryDirectory, path);

  const load = async (path: string, expected: "extractor" | "module", fromPath?: string): Promise<MutableLoadedDocument | undefined> => {
    path = cleanPath(path);
    if (loading.has(path)) {
      diagnostics.push(diagnostic("E_IMPORT_CYCLE", `import cycle includes ${JSON.stringify(displayPath(path))}`, zeroSpan(displayPath(path)), ""));
      return undefined;
    }
    const cached = documents.get(path);
    if (cached !== undefined) {
      if (expected === "module" && cached.kind !== "module") {
        diagnostics.push(diagnostic("E_IMPORT_KIND", `imported file ${JSON.stringify(path)} is not a module document`, cached.document.span, ""));
      }
      return cached;
    }

    loading.add(path);
    try {
      options.signal?.throwIfAborted();
      let bytes: Uint8Array<ArrayBuffer>;
      if (path === entryPath) bytes = initialBytes;
      else if (options.loader === undefined) {
        diagnostics.push(readDiagnostic(displayPath(path), "no source loader configured"));
        return undefined;
      } else {
        try {
          const context = options.signal === undefined
            ? { fromPath: fromPath ?? entryPath }
            : { fromPath: fromPath ?? entryPath, signal: options.signal };
          const loaded = await options.loader.load(path, context);
          options.signal?.throwIfAborted();
          bytes = copyBytes(loaded);
        } catch (error) {
          options.signal?.throwIfAborted();
          diagnostics.push(readDiagnostic(displayPath(path), errorMessage(error)));
          return undefined;
        }
      }

      let text: string;
      try {
        text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
      } catch {
        diagnostics.push(diagnostic("E_KDL_SYNTAX", "document is not valid UTF-8", zeroSpan(displayPath(path)), ""));
        return undefined;
      }
      const parsed = parse(displayPath(path), text);
      diagnostics.push(...parsed.diagnostics);
      if (parsed.diagnostics.some((item) => item.severity === "error")) return undefined;

      const loaded: MutableLoadedDocument = {
        path, displayPath: displayPath(path), bytes, document: parsed.document, imports: new Map(),
      };
      documents.set(path, loaded);

      const roots: Node[] = [];
      let seenRoot = false;
      for (const node of parsed.document.nodes) {
        if (node.name === "import") {
          if (seenRoot) diagnostics.push(diagnostic("E_DOCUMENT_ROOT", "imports must appear before the document root", node.span, ""));
          await loadImport(loaded, node);
          continue;
        }
        seenRoot = true;
        if (node.name === "extractor" || node.name === "module") roots.push(node);
        else diagnostics.push(diagnostic("E_UNKNOWN_NODE", `top-level node ${JSON.stringify(node.name)} is not allowed`, node.span, ""));
      }
      if (roots.length !== 1) {
        diagnostics.push(diagnostic("E_DOCUMENT_ROOT", `document must contain exactly one extractor or module root; found ${roots.length}`, parsed.document.span, ""));
        return loaded;
      }
      loaded.root = roots[0];
      loaded.kind = roots[0]!.name as "extractor" | "module";
      if (expected === "module" && loaded.kind !== "module") {
        diagnostics.push(diagnostic("E_IMPORT_KIND", "import target must be a module document", loaded.root.span, ""));
      }
      if (expected === "extractor" && loaded.kind !== "extractor") {
        diagnostics.push(diagnostic("E_DOCUMENT_ROOT", "entry document must be an extractor document", loaded.root.span, ""));
      }
      return loaded;
    } finally {
      loading.delete(path);
    }
  };

  const loadImport = async (document: MutableLoadedDocument, node: Node): Promise<void> => {
    validateImportNode(node, diagnostics);
    const pathArgument = stringArgument(node, 0);
    if (pathArgument === undefined) {
      diagnostics.push(diagnostic("E_TYPE_MISMATCH", "import path must be a string", node.span, "imports"));
      return;
    }
    const alias = stringProperty(node, "as");
    if (alias === undefined || alias === "") {
      diagnostics.push(diagnostic("E_IMPORT_ALIAS_REQUIRED", "import requires non-empty string property as", node.span, "imports"));
      return;
    }
    if (!/^[a-z][a-z0-9_]*$/u.test(alias)) {
      diagnostics.push(diagnostic("E_IDENTIFIER_INVALID", `invalid identifier ${JSON.stringify(alias)}`, node.span, "imports"));
    }
    if (document.imports.has(alias)) {
      diagnostics.push(diagnostic("E_DUPLICATE_SYMBOL", `duplicate import alias ${JSON.stringify(alias)}`, node.span, "imports"));
      return;
    }
    if (pathArgument.includes("://") || pathArgument.startsWith("/")) {
      diagnostics.push(diagnostic("E_REMOTE_IMPORT_UNSUPPORTED", "import path must be relative", node.span, "imports"));
      return;
    }
    const resolved = cleanPath(joinPath(dirname(document.path), pathArgument));
    const imported = await load(resolved, "module", document.path);
    document.imports.set(alias, imported);
  };

  const entry = await load(entryPath, "extractor");
  options.signal?.throwIfAborted();
  const result: SourceGraph = {
    documents,
    diagnostics: sortDiagnostics(diagnostics),
  };
  return entry === undefined ? result : { ...result, entry };
}

function validateImportNode(node: Node, diagnostics: LoadDiagnostic[]): void {
  if (node.arguments.length !== 1) {
    diagnostics.push(diagnostic("E_ARGUMENT_COUNT", `node "import" expects 1..1 positional arguments, got ${node.arguments.length}`, node.span, "imports"));
  }
  const seen = new Set<string>();
  for (const property of node.properties) {
    if (seen.has(property.name)) diagnostics.push(diagnostic("E_DUPLICATE_PROPERTY", `duplicate property ${JSON.stringify(property.name)}`, property.span, "imports"));
    seen.add(property.name);
    if (property.name !== "as") {
      diagnostics.push(diagnostic("E_UNKNOWN_PROPERTY", `property ${JSON.stringify(property.name)} is not allowed on "import"`, property.span, "imports"));
    } else if (property.value.kind !== "string") {
      diagnostics.push(diagnostic("E_TYPE_MISMATCH", `property "as" has incompatible value kind ${property.value.kind}`, property.value.span, "imports"));
    }
  }
}

function property(node: Node, name: string): Property | undefined {
  return [...node.properties].reverse().find((item) => item.name === name);
}

function stringArgument(node: Node, index: number): string | undefined {
  const value = node.arguments[index];
  return value?.kind === "string" ? String(value.value) : undefined;
}

function stringProperty(node: Node, name: string): string | undefined {
  const value: Value | undefined = property(node, name)?.value;
  return value?.kind === "string" ? String(value.value) : undefined;
}

function copyBytes(data: string | Uint8Array): Uint8Array<ArrayBuffer> {
  const input = typeof data === "string" ? new TextEncoder().encode(data) : data;
  const copy = new Uint8Array(new ArrayBuffer(input.byteLength));
  copy.set(input);
  return copy;
}

function cleanPath(path: string): string {
  const absolute = path.startsWith("/");
  const parts: string[] = [];
  for (const part of path.split("/")) {
    if (part === "" || part === ".") continue;
    if (part === ".." && parts.at(-1) !== "..") {
      if (parts.length > 0) parts.pop();
      else if (!absolute) parts.push(part);
    } else parts.push(part);
  }
  const cleaned = parts.join("/");
  return absolute ? `/${cleaned}` : cleaned === "" ? "." : cleaned;
}

function dirname(path: string): string {
  const index = path.lastIndexOf("/");
  if (index < 0) return ".";
  if (index === 0) return "/";
  return path.slice(0, index);
}

function joinPath(directory: string, path: string): string {
  return directory === "." ? path : `${directory}/${path}`;
}

function relativePath(from: string, target: string): string {
  const fromParts = cleanPath(from).split("/").filter((part) => part !== "." && part !== "");
  const targetParts = cleanPath(target).split("/").filter((part) => part !== "." && part !== "");
  let shared = 0;
  while (shared < fromParts.length && shared < targetParts.length && fromParts[shared] === targetParts[shared]) shared++;
  const result = [...fromParts.slice(shared).map(() => ".."), ...targetParts.slice(shared)].join("/");
  return result === "" ? "." : result;
}

function zeroSpan(path: string): Span {
  const position = { offset: 0, line: 1, column: 1 };
  return { file: path, start: position, end: position };
}

function readDiagnostic(path: string, message: string): LoadDiagnostic {
  return diagnostic("E_KDL_SYNTAX", `read ${JSON.stringify(path)}: ${message}`, zeroSpan(path), "");
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function diagnostic(code: string, message: string, span: Span, path: string): LoadDiagnostic {
  const result: LoadDiagnostic = { code, severity: "error", message, span };
  return path === "" ? result : { ...result, path };
}

function sortDiagnostics(diagnostics: readonly LoadDiagnostic[]): readonly LoadDiagnostic[] {
  return [...diagnostics].sort((left, right) =>
    compareCodePoints(left.span.file, right.span.file)
    || left.span.start.offset - right.span.start.offset
    || compareCodePoints(left.code, right.code));
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
