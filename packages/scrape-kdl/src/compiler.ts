import type { CompileOptions, Source } from "./public-api.js";
import type { Node, Property, Span, Value } from "./parser.js";
import { loadSourceGraph, type LoadedDocument } from "./source-loader.js";
import { parseSelector } from "./selector.js";
import { compilePortableRegex } from "./portable-regex.js";
import { BUILTIN_CONTRACT, BUILTIN_NAMES, type BuiltinExpectation } from "./builtin-contract.js";
import type {
  CollectionIR,
  ExtractorIR,
  FieldIR,
  InputIR,
  JsonScalar,
  NamedArgumentIR,
  OutputObjectIR,
  PrimitiveTypeName,
  ResolvedTransformCallIR,
  SourceFileIR,
  SourceIR,
  TemplateSegmentIR,
  TransformIR,
  TypeRef,
  ValueSourceIR,
  WorkflowStepIR,
} from "./ir.js";

const LANGUAGE_VERSION = "2026-07-15";
const IR_VERSION = "2026-07-15";
const MAX_MILLISECONDS = 9_223_372_036_854n;
const USER_NAME = /^[a-z][a-z0-9_]*$/u;
const ROOT_NAME = /^[a-z][a-z0-9-]*$/u;

export interface Diagnostic {
  readonly code: string;
  readonly severity: "error" | "warning";
  readonly message: string;
  readonly span: Span;
  readonly path?: string;
}

export interface CompilerResult {
  readonly ir?: ExtractorIR;
  readonly diagnostics: readonly Diagnostic[];
}

type Expectation = "string" | "bool" | "int" | "non-negative-int" | "number" | "scalar";
type Origin = "local" | "imported";

interface DocumentState {
  readonly loaded: LoadedDocument;
  readonly imports: ReadonlyMap<string, DocumentState | undefined>;
  readonly transforms: Map<string, TransformDeclaration>;
  moduleName?: string;
  moduleVersion?: string;
}

interface TransformDeclaration {
  readonly document: DocumentState;
  readonly node: Node;
  readonly name: string;
  readonly symbolId: string;
  readonly input: TypeRef;
  readonly output: TypeRef;
  compiled?: TransformIR;
  compiling: boolean;
}

class SemanticCompiler {
  readonly diagnostics: Diagnostic[];
  readonly capabilities = new Set<string>();
  readonly states = new Map<string, DocumentState>();
  jsPresent = false;

  constructor(
    readonly entry: LoadedDocument,
    graphDiagnostics: readonly Diagnostic[],
  ) {
    this.diagnostics = [...graphDiagnostics];
    this.buildStates(entry);
  }

  async compile(): Promise<CompilerResult> {
    const entry = this.states.get(this.entry.path);
    if (entry === undefined || this.hasErrors()) return { diagnostics: sortedDiagnostics(this.diagnostics) };
    for (const state of this.states.values()) this.compileRootHeader(state);
    for (const state of this.states.values()) this.collectTransformDeclarations(state);
    if (this.hasErrors()) return { diagnostics: sortedDiagnostics(this.diagnostics) };

    const ir = await this.compileExtractor(entry);
    if (this.jsPresent)
      this.add(
        "W_JAVASCRIPT_PRESENT",
        "warning",
        "specification contains trusted JavaScript execution",
        this.entry.root!.span,
        "source",
      );
    if (this.hasErrors()) return { diagnostics: sortedDiagnostics(this.diagnostics) };
    return { ir, diagnostics: sortedDiagnostics(this.diagnostics) };
  }

  private buildStates(document: LoadedDocument): DocumentState {
    const cached = this.states.get(document.path);
    if (cached !== undefined) return cached;
    const state: DocumentState = { loaded: document, imports: new Map(), transforms: new Map() };
    this.states.set(document.path, state);
    const imports = state.imports as Map<string, DocumentState | undefined>;
    for (const [alias, imported] of document.imports)
      imports.set(alias, imported === undefined ? undefined : this.buildStates(imported));
    return state;
  }

  private compileRootHeader(state: DocumentState): void {
    const root = state.loaded.root;
    if (root === undefined) return;
    validateNode(this.diagnostics, root, 1, 1, { version: "scalar", "language-version": "scalar" }, root.name);
    const name = stringArgument(root, 0);
    if (name === undefined) {
      this.add("E_TYPE_MISMATCH", "error", `${root.name} name must be a string`, root.span, root.name);
      return;
    }
    validateIdentifier(this.diagnostics, name, root.span, true, root.name);
    const version = this.documentVersion(root);
    this.languageVersion(root);
    if (state.loaded.kind === "module") {
      for (const child of root.children) {
        if (child.name !== "transform")
          this.add(
            "E_UNKNOWN_NODE",
            "error",
            `node ${JSON.stringify(child.name)} is not allowed in module`,
            child.span,
            "module",
          );
      }
      state.moduleName = name;
      state.moduleVersion = version;
    }
  }

  private documentVersion(root: Node): string {
    const property = nodeProperty(root, "version");
    if (property === undefined) {
      this.add("E_DOCUMENT_VERSION_REQUIRED", "error", "document requires version", root.span, `${root.name}.version`);
      return "";
    }
    const version = property.value.kind === "string" ? String(property.value.value) : "";
    if (!isDateIdentifier(version)) {
      this.add(
        "E_DOCUMENT_VERSION_INVALID",
        "error",
        "version must be a real calendar date in YYYY-MM-DD form",
        property.value.span,
        `${root.name}.version`,
      );
      return "";
    }
    return version;
  }

  private languageVersion(root: Node): string {
    const property = nodeProperty(root, "language-version");
    if (property === undefined) {
      this.add(
        "E_LANGUAGE_VERSION_REQUIRED",
        "error",
        "document requires language-version",
        root.span,
        `${root.name}.language-version`,
      );
      return "";
    }
    const version = property.value.kind === "string" ? String(property.value.value) : "";
    if (!isDateIdentifier(version)) {
      this.add(
        "E_LANGUAGE_VERSION_INVALID",
        "error",
        "language-version must be a real calendar date in YYYY-MM-DD form",
        property.value.span,
        `${root.name}.language-version`,
      );
      return "";
    }
    if (version !== LANGUAGE_VERSION) {
      this.add(
        "E_LANGUAGE_VERSION_UNSUPPORTED",
        "error",
        `unsupported language-version ${JSON.stringify(version)}`,
        property.value.span,
        `${root.name}.language-version`,
      );
      return "";
    }
    return version;
  }

  private collectTransformDeclarations(state: DocumentState): void {
    const root = state.loaded.root;
    if (root === undefined || state.transforms.size > 0) return;
    for (const child of root.children) {
      if (child.name !== "transform") continue;
      validateNode(this.diagnostics, child, 1, 1, { input: "string", output: "string" }, "transforms");
      const name = stringArgument(child, 0);
      if (name === undefined) {
        this.add("E_TYPE_MISMATCH", "error", "transform name must be a string", child.span, "transforms");
        continue;
      }
      validateIdentifier(this.diagnostics, name, child.span, false, `transforms.${name}`);
      if (BUILTIN_NAMES.has(name))
        this.add(
          "E_TRANSFORM_SHADOWS_BUILTIN",
          "error",
          `transform ${JSON.stringify(name)} shadows built-in`,
          child.span,
          `transforms.${name}`,
        );
      if (state.transforms.has(name)) {
        this.add(
          "E_DUPLICATE_SYMBOL",
          "error",
          `duplicate transform ${JSON.stringify(name)}`,
          child.span,
          `transforms.${name}`,
        );
        continue;
      }
      const inputRaw = stringProperty(child, "input");
      const outputRaw = stringProperty(child, "output");
      if (inputRaw === undefined || outputRaw === undefined) {
        this.add(
          "E_TYPE_UNKNOWN",
          "error",
          "transform requires string input and output properties",
          child.span,
          `transforms.${name}`,
        );
        continue;
      }
      const input = parseType(inputRaw);
      const output = parseType(outputRaw);
      if (input instanceof Error) {
        this.add("E_TYPE_UNKNOWN", "error", input.message, child.span, `transforms.${name}.input`);
        continue;
      }
      if (output instanceof Error) {
        this.add("E_TYPE_UNKNOWN", "error", output.message, child.span, `transforms.${name}.output`);
        continue;
      }
      state.transforms.set(name, {
        document: state,
        node: child,
        name,
        symbolId: `${state.loaded.displayPath}#transform:${name}`,
        input,
        output,
        compiling: false,
      });
    }
  }

  private async compileExtractor(state: DocumentState): Promise<ExtractorIR> {
    const root = state.loaded.root!;
    const [inputs, inputMap] = this.compileInputs(state);
    const source = this.compileSource(state, inputMap);
    const transforms = this.compileReachableTransforms(state);
    const output = this.compileOutputObject(state, root.children, "output", state);
    const files: SourceFileIR[] = [];
    for (const document of this.states.values()) {
      const digest = await crypto.subtle.digest("SHA-256", document.loaded.bytes);
      const file: SourceFileIR = {
        path: document.loaded.displayPath,
        sha256: Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join(""),
      };
      files.push(
        document.moduleName === undefined
          ? file
          : { ...file, moduleName: document.moduleName, moduleVersion: document.moduleVersion! },
      );
    }
    files.sort((left, right) => compareCodePoints(left.path, right.path));
    return {
      kind: "extractor",
      irVersion: IR_VERSION,
      languageVersion: stringProperty(root, "language-version") as typeof LANGUAGE_VERSION,
      name: stringArgument(root, 0)!,
      version: stringProperty(root, "version")!,
      files,
      source,
      inputs,
      transforms,
      output,
      capabilities: [...this.capabilities].sort(compareCodePoints),
      span: root.span,
    };
  }

  private compileInputs(state: DocumentState): [InputIR[], Map<string, InputIR>] {
    const output: InputIR[] = [];
    const byName = new Map<string, InputIR>();
    for (const node of state.loaded.root!.children) {
      if (node.name !== "input") continue;
      validateNode(this.diagnostics, node, 1, 1, { type: "string", required: "bool", default: "scalar" }, "inputs");
      const name = stringArgument(node, 0);
      if (name === undefined) {
        this.add("E_TYPE_MISMATCH", "error", "input name must be a string", node.span, "inputs");
        continue;
      }
      validateIdentifier(this.diagnostics, name, node.span, false, `inputs.${name}`);
      if (byName.has(name)) {
        this.add("E_DUPLICATE_SYMBOL", "error", `duplicate input ${JSON.stringify(name)}`, node.span, `inputs.${name}`);
        continue;
      }
      const type = stringProperty(node, "type");
      if (type === undefined || !["string", "bool", "int", "float"].includes(type)) {
        this.add(
          "E_TYPE_UNKNOWN",
          "error",
          "input type must be string, bool, int, or float",
          node.span,
          `inputs.${name}`,
        );
        continue;
      }
      const required = booleanProperty(node, "required", true);
      const base: InputIR = { name, type: type as InputIR["type"], required, span: node.span };
      const defaultProperty = nodeProperty(node, "default");
      let item = base;
      if (defaultProperty !== undefined) {
        if (required)
          this.add(
            "E_INPUT_REQUIRED_DEFAULT",
            "error",
            "required input must not declare default",
            defaultProperty.value.span,
            `inputs.${name}`,
          );
        if (!inputDefaultCompatible(defaultProperty.value, type))
          this.add(
            "E_DEFAULT_INVALID",
            "error",
            `default is incompatible with input type ${type}`,
            defaultProperty.value.span,
            `inputs.${name}`,
          );
        item = { ...base, default: scalar(defaultProperty.value) };
      }
      output.push(item);
      byName.set(name, item);
    }
    return [output, byName];
  }

  private compileSource(state: DocumentState, inputs: ReadonlyMap<string, InputIR>): SourceIR {
    const root = state.loaded.root!;
    const sourceNodes = root.children.filter((node) => node.name === "source");
    if (sourceNodes.length !== 1) {
      this.add(
        "E_DOCUMENT_ROOT",
        "error",
        `extractor requires exactly one source; found ${sourceNodes.length}`,
        root.span,
        "source",
      );
      return {
        kind: "html",
        fetch: { mode: "http", urlTemplate: { raw: "", segments: [] }, span: root.span },
        sessionPolicy: "none",
        workflow: [],
        span: root.span,
      };
    }
    const node = sourceNodes[0]!;
    validateNode(this.diagnostics, node, 1, 1, {}, "source");
    const kind = stringArgument(node, 0);
    if (kind !== "html") this.add("E_TYPE_MISMATCH", "error", 'source argument must be "html"', node.span, "source");
    const fetchNodes: Node[] = [];
    const sessionNodes: Node[] = [];
    const workflowNodes: Node[] = [];
    for (const child of node.children) {
      if (child.name === "fetch") fetchNodes.push(child);
      else if (child.name === "session") sessionNodes.push(child);
      else if (child.name === "workflow") workflowNodes.push(child);
      else
        this.add(
          "E_UNKNOWN_NODE",
          "error",
          `node ${JSON.stringify(child.name)} is not allowed in source`,
          child.span,
          "source",
        );
    }
    let fetch: SourceIR["fetch"] = { mode: "http", urlTemplate: { raw: "", segments: [] }, span: node.span };
    if (fetchNodes.length !== 1)
      this.add(
        "E_DOCUMENT_ROOT",
        "error",
        `source requires exactly one fetch; found ${fetchNodes.length}`,
        node.span,
        "source.fetch",
      );
    else fetch = this.compileFetch(fetchNodes[0]!, inputs);
    let sessionPolicy: SourceIR["sessionPolicy"] = "none";
    if (sessionNodes.length > 1)
      this.add(
        "E_DOCUMENT_ROOT",
        "error",
        "source allows at most one session",
        sessionNodes[1]!.span,
        "source.session",
      );
    if (sessionNodes[0] !== undefined) {
      const session = sessionNodes[0];
      validateNode(this.diagnostics, session, 0, 0, { policy: "string" }, "source.session");
      const policy = stringProperty(session, "policy");
      if (policy !== "none" && policy !== "optional" && policy !== "required")
        this.add(
          "E_TYPE_MISMATCH",
          "error",
          "session policy must be none, optional, or required",
          session.span,
          "source.session",
        );
      else sessionPolicy = policy;
    }
    let workflow: readonly WorkflowStepIR[] = [];
    if (workflowNodes.length > 1)
      this.add(
        "E_DOCUMENT_ROOT",
        "error",
        "source allows at most one workflow",
        workflowNodes[1]!.span,
        "source.workflow",
      );
    if (workflowNodes[0] !== undefined) {
      if (fetch.mode !== "browser")
        this.add(
          "E_BROWSER_CAPABILITY_REQUIRED",
          "error",
          "workflow requires browser fetch mode",
          workflowNodes[0].span,
          "source.workflow",
        );
      workflow = this.compileWorkflow(workflowNodes[0]);
    }
    this.capabilities.add(fetch.mode === "http" ? "http.fetch" : "browser.navigate");
    return { kind: "html", fetch, sessionPolicy, workflow, span: node.span };
  }

  private compileFetch(node: Node, inputs: ReadonlyMap<string, InputIR>): SourceIR["fetch"] {
    validateNode(this.diagnostics, node, 0, 0, { mode: "string", url: "string" }, "source.fetch");
    let mode = stringProperty(node, "mode");
    if (mode !== "http" && mode !== "browser") {
      this.add("E_TYPE_MISMATCH", "error", "fetch mode must be http or browser", node.span, "source.fetch");
      mode = "http";
    }
    let raw = stringProperty(node, "url");
    if (raw === undefined || raw === "") {
      this.add("E_TEMPLATE_INVALID", "error", "fetch requires non-empty string url", node.span, "source.fetch");
      raw = "";
    }
    return { mode: mode as "http" | "browser", urlTemplate: this.compileTemplate(raw, node, inputs), span: node.span };
  }

  private compileTemplate(
    raw: string,
    node: Node,
    inputs: ReadonlyMap<string, InputIR>,
  ): SourceIR["fetch"]["urlTemplate"] {
    const segments: TemplateSegmentIR[] = [];
    let literal = "";
    const flush = (): void => {
      if (literal !== "") {
        segments.push({ kind: "literal", value: literal });
        literal = "";
      }
    };
    for (let index = 0; index < raw.length;) {
      const character = raw[index]!;
      if (character === "{") {
        if (raw[index + 1] === "{") {
          literal += "{";
          index += 2;
          continue;
        }
        flush();
        const end = raw.indexOf("}", index + 1);
        if (end < 0) {
          this.add("E_TEMPLATE_INVALID", "error", "unmatched { in URL template", node.span, "source.fetch.url");
          literal += raw.slice(index);
          break;
        }
        const name = raw.slice(index + 1, end);
        if (!USER_NAME.test(name))
          this.add(
            "E_TEMPLATE_INVALID",
            "error",
            `invalid placeholder ${JSON.stringify(name)}`,
            node.span,
            "source.fetch.url",
          );
        else {
          const input = inputs.get(name);
          if (input === undefined)
            this.add(
              "E_INPUT_UNDECLARED",
              "error",
              `URL template references undeclared input ${JSON.stringify(name)}`,
              node.span,
              "source.fetch.url",
            );
          else if (!input.required && input.default === undefined)
            this.add(
              "E_TEMPLATE_OPTIONAL_INPUT",
              "error",
              `optional input ${JSON.stringify(name)} used by URL template requires default`,
              node.span,
              "source.fetch.url",
            );
        }
        segments.push({ kind: "input", name });
        index = end + 1;
      } else if (character === "}") {
        if (raw[index + 1] === "}") {
          literal += "}";
          index += 2;
          continue;
        }
        this.add("E_TEMPLATE_INVALID", "error", "unmatched } in URL template", node.span, "source.fetch.url");
        index++;
      } else {
        literal += character;
        index++;
      }
    }
    flush();
    const probe = segments.map((segment) => (segment.kind === "literal" ? segment.value : "x")).join("");
    try {
      const parsed = new URL(probe);
      if ((parsed.protocol !== "http:" && parsed.protocol !== "https:") || parsed.host === "") throw new Error();
    } catch {
      this.add(
        "E_TEMPLATE_INVALID",
        "error",
        "expanded URL template must be an absolute http or https URL",
        node.span,
        "source.fetch.url",
      );
    }
    return { raw, segments };
  }

  private compileWorkflow(node: Node): readonly WorkflowStepIR[] {
    validateNode(this.diagnostics, node, 0, 0, {}, "source.workflow");
    const output: WorkflowStepIR[] = [];
    node.children.forEach((child, index) => {
      const path = `source.workflow[${index}]`;
      if (child.name === "wait-for") {
        validateNode(this.diagnostics, child, 1, 1, { state: "string", "timeout-ms": "non-negative-int" }, path);
        const selector = stringArgument(child, 0) ?? "";
        this.checkSelector(selector, child, path);
        let state = stringProperty(child, "state") ?? "visible";
        if (!new Set(["attached", "visible", "hidden", "detached"]).has(state)) {
          this.add("E_TYPE_MISMATCH", "error", "invalid wait-for state", child.span, path);
          state = "visible";
        }
        output.push(
          withOptional(
            {
              kind: "wait-for",
              selector,
              state: state as "attached" | "visible" | "hidden" | "detached",
              span: child.span,
            },
            "timeoutMs",
            this.duration(child, "timeout-ms", path),
          ),
        );
        this.capabilities.add("browser.wait");
      } else if (child.name === "click") {
        validateNode(this.diagnostics, child, 1, 1, { "timeout-ms": "non-negative-int" }, path);
        const selector = stringArgument(child, 0) ?? "";
        this.checkSelector(selector, child, path);
        output.push(
          withOptional(
            { kind: "click", selector, span: child.span },
            "timeoutMs",
            this.duration(child, "timeout-ms", path),
          ),
        );
        this.capabilities.add("browser.input");
      } else if (child.name === "fill" || child.name === "press") {
        validateNode(this.diagnostics, child, 2, 2, { "timeout-ms": "non-negative-int" }, path);
        const selector = stringArgument(child, 0) ?? "";
        this.checkSelector(selector, child, path);
        const timeout = this.duration(child, "timeout-ms", path);
        if (child.name === "fill")
          output.push(
            withOptional(
              { kind: "fill", selector, value: stringArgument(child, 1) ?? "", span: child.span },
              "timeoutMs",
              timeout,
            ),
          );
        else
          output.push(
            withOptional(
              { kind: "press", selector, key: stringArgument(child, 1) ?? "", span: child.span },
              "timeoutMs",
              timeout,
            ),
          );
        this.capabilities.add("browser.input");
      } else if (child.name === "scroll") {
        validateNode(this.diagnostics, child, 2, 2, {}, path);
        const x = numberArgument(child, 0);
        const y = numberArgument(child, 1);
        if (x === undefined || y === undefined)
          this.add("E_TYPE_MISMATCH", "error", "scroll requires numeric x and y", child.span, path);
        output.push({ kind: "scroll", x: x ?? 0, y: y ?? 0, span: child.span });
        this.capabilities.add("browser.scroll");
      } else if (child.name === "wait-for-network-idle") {
        validateNode(
          this.diagnostics,
          child,
          0,
          0,
          { "idle-ms": "non-negative-int", "timeout-ms": "non-negative-int" },
          path,
        );
        let idleMs = integerProperty(child, "idle-ms") ?? 500;
        if (idleMs < 1) this.add("E_TYPE_MISMATCH", "error", "idle-ms must be positive", child.span, path);
        else if ((nodeProperty(child, "idle-ms")?.value.integerValue ?? BigInt(idleMs)) > MAX_MILLISECONDS)
          this.add("E_TYPE_MISMATCH", "error", `idle-ms must not exceed ${MAX_MILLISECONDS}`, child.span, path);
        output.push(
          withOptional(
            { kind: "wait-for-network-idle", idleMs, span: child.span },
            "timeoutMs",
            this.duration(child, "timeout-ms", path),
          ),
        );
        this.capabilities.add("browser.network-idle");
      } else if (child.name === "evaluate-js") {
        validateNode(this.diagnostics, child, 1, 1, { "timeout-ms": "non-negative-int" }, path);
        output.push(
          withOptional(
            { kind: "evaluate-js", source: stringArgument(child, 0) ?? "", span: child.span },
            "timeoutMs",
            this.duration(child, "timeout-ms", path),
          ),
        );
        this.capabilities.add("browser.evaluate-js");
        this.jsPresent = true;
      } else
        this.add("E_UNKNOWN_NODE", "error", `unknown workflow step ${JSON.stringify(child.name)}`, child.span, path);
    });
    return output;
  }

  private compileReachableTransforms(root: DocumentState): TransformIR[] {
    const seen = new Set<string>();
    const visitedDocuments = new Set<DocumentState>();
    const output: TransformIR[] = [];
    const visit = (state: DocumentState | undefined, origin: Origin): void => {
      if (state === undefined) return;
      if (visitedDocuments.has(state)) return;
      visitedDocuments.add(state);
      for (const imported of state.imports.values()) visit(imported, "imported");
      for (const name of [...state.transforms.keys()].sort(compareCodePoints)) {
        const declaration = state.transforms.get(name)!;
        if (seen.has(declaration.symbolId)) continue;
        seen.add(declaration.symbolId);
        const transform = this.compileTransform(declaration, origin);
        if (transform !== undefined) output.push(transform);
      }
    };
    for (const imported of root.imports.values()) visit(imported, "imported");
    for (const child of root.loaded.root!.children) {
      if (child.name !== "transform") continue;
      const declaration = root.transforms.get(stringArgument(child, 0) ?? "");
      if (declaration === undefined || seen.has(declaration.symbolId)) continue;
      seen.add(declaration.symbolId);
      const transform = this.compileTransform(declaration, "local");
      if (transform !== undefined) output.push(transform);
    }
    return output;
  }

  private compileTransform(declaration: TransformDeclaration, origin: Origin): TransformIR | undefined {
    if (declaration.compiled !== undefined) return { ...declaration.compiled, origin };
    if (declaration.compiling) return undefined;
    declaration.compiling = true;
    try {
      const bodies: Node[] = [];
      for (const child of declaration.node.children) {
        if (["pipeline", "match", "external"].includes(child.name)) bodies.push(child);
        else
          this.add(
            "E_UNKNOWN_NODE",
            "error",
            `node ${JSON.stringify(child.name)} is not allowed in transform`,
            child.span,
            `transforms.${declaration.name}`,
          );
      }
      if (bodies.length !== 1) {
        this.add(
          "E_DOCUMENT_ROOT",
          "error",
          `transform requires exactly one body; found ${bodies.length}`,
          declaration.node.span,
          `transforms.${declaration.name}`,
        );
        return undefined;
      }
      const body = bodies[0]!;
      const base = {
        symbolId: declaration.symbolId,
        name: declaration.name,
        origin,
        input: declaration.input,
        output: declaration.output,
        span: declaration.node.span,
      };
      if (body.name === "pipeline") {
        validateNode(this.diagnostics, body, 0, 0, {}, `transforms.${declaration.name}.pipeline`);
        let current = declaration.input;
        const calls: ResolvedTransformCallIR[] = [];
        body.children.forEach((child, index) => {
          if (child.name !== "apply") {
            this.add(
              "E_UNKNOWN_NODE",
              "error",
              "pipeline only allows apply nodes",
              child.span,
              `transforms.${declaration.name}.pipeline[${index}]`,
            );
            return;
          }
          const [call, next] = this.compileTransformCall(
            declaration.document,
            child,
            current,
            `transforms.${declaration.name}.pipeline.calls[${index}]`,
          );
          calls.push(call);
          current = next;
        });
        if (calls.length === 0)
          this.add(
            "E_ARGUMENT_COUNT",
            "error",
            "pipeline requires at least one apply",
            body.span,
            `transforms.${declaration.name}.pipeline`,
          );
        if (!isAssignable(current, declaration.output))
          this.add(
            "E_TRANSFORM_TYPE_MISMATCH",
            "error",
            `pipeline output ${typeString(current)} is not assignable to declared output ${typeString(declaration.output)}`,
            body.span,
            `transforms.${declaration.name}`,
          );
        const transform: TransformIR = { kind: "pipeline", ...base, calls };
        declaration.compiled = transform;
        return transform;
      }
      if (body.name === "match") {
        validateNode(this.diagnostics, body, 0, 0, {}, `transforms.${declaration.name}.match`);
        if (!isScalarType(declaration.input) || !isScalarType(declaration.output))
          this.add(
            "E_TRANSFORM_TYPE_MISMATCH",
            "error",
            "match input and output must be scalar or nullable scalar",
            body.span,
            `transforms.${declaration.name}`,
          );
        const cases: { when: JsonScalar; then: JsonScalar; span: Span }[] = [];
        let defaultValue: JsonScalar = null;
        let defaults = 0;
        const seen = new Set<string>();
        for (const child of body.children) {
          if (child.name === "case") {
            validateNode(this.diagnostics, child, 2, 2, {}, `transforms.${declaration.name}.match`);
            if (child.arguments.length < 2) continue;
            const when = scalar(child.arguments[0]!);
            const then = scalar(child.arguments[1]!);
            const key = JSON.stringify(when);
            if (seen.has(key))
              this.add(
                "E_MATCH_DUPLICATE_CASE",
                "error",
                "duplicate match case",
                child.span,
                `transforms.${declaration.name}.match`,
              );
            seen.add(key);
            if (!valueAssignable(child.arguments[0]!, declaration.input))
              this.add(
                "E_TYPE_MISMATCH",
                "error",
                "case input incompatible with transform input",
                child.span,
                `transforms.${declaration.name}.match`,
              );
            if (!valueAssignable(child.arguments[1]!, declaration.output))
              this.add(
                "E_TYPE_MISMATCH",
                "error",
                "case result incompatible with transform output",
                child.span,
                `transforms.${declaration.name}.match`,
              );
            cases.push({ when, then, span: child.span });
          } else if (child.name === "default") {
            validateNode(this.diagnostics, child, 1, 1, {}, `transforms.${declaration.name}.match`);
            defaults++;
            if (child.arguments[0] !== undefined) {
              defaultValue = scalar(child.arguments[0]);
              if (!valueAssignable(child.arguments[0], declaration.output))
                this.add(
                  "E_TYPE_MISMATCH",
                  "error",
                  "default result incompatible with transform output",
                  child.span,
                  `transforms.${declaration.name}.match`,
                );
            }
          } else
            this.add(
              "E_UNKNOWN_NODE",
              "error",
              `node ${JSON.stringify(child.name)} is not allowed in match`,
              child.span,
              `transforms.${declaration.name}.match`,
            );
        }
        if (defaults !== 1)
          this.add(
            "E_MATCH_DEFAULT",
            "error",
            `match requires exactly one default; found ${defaults}`,
            body.span,
            `transforms.${declaration.name}.match`,
          );
        const transform: TransformIR = { kind: "match", ...base, cases, default: defaultValue };
        declaration.compiled = transform;
        return transform;
      }
      validateNode(this.diagnostics, body, 0, 0, { symbol: "string" }, `transforms.${declaration.name}.external`);
      const symbol = stringProperty(body, "symbol") ?? "";
      if (symbol === "")
        this.add(
          "E_EXTERNAL_TRANSFORM_MISSING",
          "error",
          "external transform requires non-empty symbol",
          body.span,
          `transforms.${declaration.name}.external`,
        );
      this.capabilities.add(`transform.external:${symbol}`);
      const transform: TransformIR = { kind: "external", ...base, symbol };
      declaration.compiled = transform;
      return transform;
    } finally {
      declaration.compiling = false;
    }
  }

  private compileTransformCall(
    state: DocumentState,
    node: Node,
    input: TypeRef,
    path: string,
  ): [ResolvedTransformCallIR, TypeRef] {
    if (node.arguments.length < 1)
      this.add("E_ARGUMENT_COUNT", "error", "apply requires a transform name", node.span, path);
    let name = stringArgument(node, 0);
    if (name === undefined) {
      name = "";
      this.add("E_TRANSFORM_UNKNOWN", "error", "apply target must be a string", node.span, path);
    }
    const positionalArguments = node.arguments.slice(1).map(scalar);
    const namedArguments: NamedArgumentIR[] = node.properties.map((property) => ({
      name: property.name,
      value: scalar(property.value),
    }));
    if (BUILTIN_NAMES.has(name)) {
      validateNode(this.diagnostics, node, 1, -1, builtinAllowedProperties(name), path);
      for (const property of builtinRequiredProperties(name))
        if (nodeProperty(node, property) === undefined)
          this.add(
            "E_TRANSFORM_ARGUMENT",
            "error",
            `built-in ${JSON.stringify(name)} requires property ${JSON.stringify(property)}`,
            node.span,
            path,
          );
      this.validateBuiltinArguments(name, input, node, path);
      if (name !== "assert-enum" && node.arguments.length > 1)
        this.add(
          "E_TRANSFORM_ARGUMENT",
          "error",
          `built-in ${JSON.stringify(name)} does not accept positional arguments`,
          node.span,
          path,
        );
      if (name === "assert-enum" && node.arguments.length < 2)
        this.add("E_TRANSFORM_ARGUMENT", "error", "assert-enum requires at least one allowed value", node.span, path);
      const applied = applyBuiltin(name, input, node);
      let output = input;
      if (applied instanceof Error)
        this.add("E_TRANSFORM_TYPE_MISMATCH", "error", `${name}: ${applied.message}`, node.span, path);
      else output = applied;
      return [
        { target: { kind: "builtin", name }, positionalArguments, namedArguments, input, output, span: node.span },
        output,
      ];
    }
    const declaration = this.resolveTransform(state, name);
    if (declaration === undefined) {
      this.add("E_TRANSFORM_UNKNOWN", "error", `unknown transform ${JSON.stringify(name)}`, node.span, path);
      return [
        {
          target: { kind: "declared", symbolId: `unresolved:${name}` },
          positionalArguments,
          namedArguments,
          input,
          output: input,
          span: node.span,
        },
        input,
      ];
    }
    if (node.arguments.length > 1 || node.properties.length > 0)
      this.add(
        "E_TRANSFORM_ARGUMENT",
        "error",
        "declared transforms accept no call arguments or properties",
        node.span,
        path,
      );
    if (!isAssignable(input, declaration.input))
      this.add(
        "E_TRANSFORM_TYPE_MISMATCH",
        "error",
        `transform ${JSON.stringify(name)} requires ${typeString(declaration.input)}, got ${typeString(input)}`,
        node.span,
        path,
      );
    return [
      {
        target: { kind: "declared", symbolId: declaration.symbolId },
        positionalArguments,
        namedArguments,
        input,
        output: declaration.output,
        span: node.span,
      },
      declaration.output,
    ];
  }

  private resolveTransform(state: DocumentState, name: string): TransformDeclaration | undefined {
    if (!name.includes(".")) return state.transforms.get(name);
    const parts = name.split(".");
    return parts.length === 2 ? state.imports.get(parts[0]!)?.transforms.get(parts[1]!) : undefined;
  }

  private compileOutputObject(
    owner: DocumentState,
    nodes: readonly Node[],
    path: string,
    scope: DocumentState,
  ): OutputObjectIR {
    const members: (FieldIR | CollectionIR)[] = [];
    const seen = new Set<string>();
    for (const node of nodes) {
      if (node.name !== "field" && node.name !== "collection") {
        if (path === "output" && !["source", "input", "transform"].includes(node.name))
          this.add(
            "E_UNKNOWN_NODE",
            "error",
            `node ${JSON.stringify(node.name)} is not allowed in extractor`,
            node.span,
            path,
          );
        continue;
      }
      const name = stringArgument(node, 0) ?? "";
      const memberPath = `${path}.${name}`;
      if (seen.has(name))
        this.add(
          "E_DUPLICATE_SYMBOL",
          "error",
          `duplicate output member ${JSON.stringify(name)}`,
          node.span,
          memberPath,
        );
      seen.add(name);
      members.push(
        node.name === "field"
          ? this.compileField(owner, node, memberPath, scope)
          : this.compileCollection(owner, node, memberPath, scope),
      );
    }
    return { kind: "object", members };
  }

  private compileField(owner: DocumentState, node: Node, path: string, scope: DocumentState): FieldIR {
    validateNode(this.diagnostics, node, 1, 1, { type: "string", required: "bool", default: "scalar" }, path);
    let name = stringArgument(node, 0);
    if (name === undefined) {
      name = "invalid";
      this.add("E_TYPE_MISMATCH", "error", "field name must be a string", node.span, path);
    }
    validateIdentifier(this.diagnostics, name, node.span, false, path);
    const rawType = stringProperty(node, "type");
    let successfulType: TypeRef = primitive("unknown");
    if (rawType === undefined)
      this.add("E_FIELD_TYPE_REQUIRED", "error", "field requires string property type", node.span, path);
    else {
      const parsed = parseType(rawType);
      if (parsed instanceof Error) this.add("E_TYPE_UNKNOWN", "error", parsed.message, node.span, path);
      else successfulType = parsed;
    }
    const required = booleanProperty(node, "required", false);
    const defaultProperty = nodeProperty(node, "default");
    if (defaultProperty !== undefined && !valueAssignable(defaultProperty.value, successfulType))
      this.add(
        "E_DEFAULT_INVALID",
        "error",
        `default type ${typeString(valueType(defaultProperty.value))} is not assignable to ${typeString(successfulType)}`,
        defaultProperty.value.span,
        path,
      );
    if (required && defaultProperty !== undefined)
      this.add("E_DEFAULT_INVALID", "error", "required field must not declare default", node.span, path);

    const selects = node.children.filter((child) => child.name === "select");
    const values = node.children.filter((child) => child.name === "value");
    const evaluations = node.children.filter((child) => child.name === "evaluate-js");
    const onErrors = node.children.filter((child) => child.name === "on-error");
    const applies = node.children.filter((child) => child.name === "apply");
    for (const child of node.children)
      if (!["select", "value", "evaluate-js", "on-error", "apply"].includes(child.name))
        this.add(
          "E_UNKNOWN_NODE",
          "error",
          `node ${JSON.stringify(child.name)} is not allowed in field`,
          child.span,
          path,
        );
    if (selects.length > 1)
      this.add("E_ARGUMENT_COUNT", "error", "field allows at most one select", selects[1]!.span, `${path}.selection`);
    if (values.length + evaluations.length === 0)
      this.add("E_VALUE_SOURCE_MISSING", "error", "field requires value or evaluate-js", node.span, path);
    if (values.length + evaluations.length > 1)
      this.add("E_VALUE_SOURCE_MULTIPLE", "error", "field has multiple value sources", node.span, path);
    if (onErrors.length > 1)
      this.add("E_ARGUMENT_COUNT", "error", "field allows at most one on-error", onErrors[1]!.span, `${path}.onError`);

    let selection: FieldIR["selection"];
    if (selects[0] !== undefined) {
      const select = selects[0];
      validateNode(this.diagnostics, select, 1, 1, { match: "string" }, `${path}.selection`);
      const selector = stringArgument(select, 0) ?? "";
      this.checkSelector(selector, select, `${path}.selection`);
      let match = stringProperty(select, "match") ?? "one";
      if (match !== "one" && match !== "first") {
        this.add("E_TYPE_MISMATCH", "error", "select match must be one or first", select.span, `${path}.selection`);
        match = "one";
      }
      selection = { selector, match: match as "one" | "first", span: select.span };
    }

    let current: TypeRef = primitive("string");
    let valueSource: ValueSourceIR = { kind: "text", rawType: primitive("string"), span: node.span };
    if (values[0] !== undefined) {
      const value = values[0];
      const kind = stringArgument(value, 0);
      validateNode(this.diagnostics, value, 1, 1, kind === "attr" ? { name: "string" } : {}, `${path}.valueSource`);
      if (selection === undefined)
        this.add("E_SELECTOR_REQUIRED", "error", "value source requires select", value.span, `${path}.selection`);
      if (kind === "text" || kind === "html") valueSource = { kind, rawType: primitive("string"), span: value.span };
      else if (kind === "attr") {
        const attribute = stringProperty(value, "name") ?? "";
        if (attribute === "")
          this.add(
            "E_ATTRIBUTE_NAME_REQUIRED",
            "error",
            "attr value requires non-empty name",
            value.span,
            `${path}.valueSource`,
          );
        valueSource = { kind: "attribute", name: attribute, rawType: primitive("string"), span: value.span };
      } else {
        this.add(
          "E_TYPE_MISMATCH",
          "error",
          "value kind must be text, html, or attr",
          value.span,
          `${path}.valueSource`,
        );
      }
      this.addReadCapability(owner, kind ?? "");
    }
    if (evaluations[0] !== undefined) {
      const evaluation = evaluations[0];
      validateNode(
        this.diagnostics,
        evaluation,
        1,
        1,
        { scope: "string", returns: "string", "timeout-ms": "non-negative-int" },
        `${path}.valueSource`,
      );
      const source = stringArgument(evaluation, 0) ?? "";
      let jsScope = stringProperty(evaluation, "scope");
      if (jsScope !== "document" && jsScope !== "current") {
        this.add(
          "E_TYPE_MISMATCH",
          "error",
          "evaluate-js scope must be document or current",
          evaluation.span,
          `${path}.valueSource`,
        );
        jsScope = "document";
      }
      const returnsRaw = stringProperty(evaluation, "returns");
      let returns: TypeRef = primitive("unknown");
      if (returnsRaw === undefined)
        this.add(
          "E_TYPE_UNKNOWN",
          "error",
          "evaluate-js requires returns type",
          evaluation.span,
          `${path}.valueSource`,
        );
      else {
        const parsed = parseType(returnsRaw);
        if (parsed instanceof Error)
          this.add("E_TYPE_UNKNOWN", "error", parsed.message, evaluation.span, `${path}.valueSource`);
        else returns = parsed;
      }
      if (jsScope === "document" && selection !== undefined)
        this.add(
          "E_SELECTOR_FORBIDDEN",
          "error",
          "document-scoped evaluate-js forbids select",
          evaluation.span,
          `${path}.selection`,
        );
      if (jsScope === "current" && selection === undefined && !path.includes("[]"))
        this.add(
          "E_CURRENT_SCOPE_UNAVAILABLE",
          "error",
          "top-level current scope requires select",
          evaluation.span,
          `${path}.valueSource`,
        );
      valueSource = withOptional(
        { kind: "javascript", scope: jsScope as "document" | "current", source, returns, span: evaluation.span },
        "timeoutMs",
        this.duration(evaluation, "timeout-ms", `${path}.valueSource`),
      );
      current = returns;
      this.capabilities.add("browser.evaluate-js");
      this.jsPresent = true;
      if (selection !== undefined && this.sourceMode(owner) === "browser") this.capabilities.add("browser.query");
      if (this.sourceMode(owner) !== "browser")
        this.add(
          "E_BROWSER_CAPABILITY_REQUIRED",
          "error",
          "evaluate-js requires browser fetch mode",
          evaluation.span,
          `${path}.valueSource`,
        );
    }
    const transforms: ResolvedTransformCallIR[] = [];
    applies.forEach((apply, index) => {
      const [call, next] = this.compileTransformCall(scope, apply, current, `${path}.transforms[${index}]`);
      transforms.push(call);
      current = next;
    });
    if (!isAssignable(current, successfulType))
      this.add(
        "E_TYPE_MISMATCH",
        "error",
        `field pipeline output ${typeString(current)} is not assignable to declared type ${typeString(successfulType)}`,
        node.span,
        path,
      );
    const effectiveType = !required && defaultProperty === undefined ? nullable(successfulType) : successfulType;
    let onError: FieldIR["onError"] = required ? "fail" : "null";
    if (onErrors[0] !== undefined) {
      const errorNode = onErrors[0];
      validateNode(this.diagnostics, errorNode, 1, 1, {}, `${path}.onError`);
      const policy = stringArgument(errorNode, 0);
      if (policy !== "fail" && policy !== "null" && policy !== "warn" && policy !== "default")
        this.add(
          "E_ERROR_POLICY_INVALID",
          "error",
          "on-error must be fail, null, warn, or default",
          errorNode.span,
          `${path}.onError`,
        );
      else onError = policy;
    }
    if ((onError === "null" || onError === "warn") && effectiveType.kind !== "nullable")
      this.add(
        "E_ERROR_POLICY_INVALID",
        "error",
        `${onError} requires nullable effective type`,
        node.span,
        `${path}.onError`,
      );
    if (onError === "default" && defaultProperty === undefined)
      this.add(
        "E_ERROR_POLICY_INVALID",
        "error",
        "default policy requires field default",
        node.span,
        `${path}.onError`,
      );
    const field: FieldIR = {
      kind: "field",
      id: path,
      name,
      successfulType,
      effectiveType,
      required,
      valueSource,
      transforms,
      onError,
      span: node.span,
    };
    const withSelection = selection === undefined ? field : { ...field, selection };
    return defaultProperty === undefined ? withSelection : { ...withSelection, default: scalar(defaultProperty.value) };
  }

  private compileCollection(owner: DocumentState, node: Node, path: string, scope: DocumentState): CollectionIR {
    validateNode(
      this.diagnostics,
      node,
      1,
      1,
      { required: "bool", "min-items": "non-negative-int", "max-items": "non-negative-int", "on-row-error": "string" },
      path,
    );
    let name = stringArgument(node, 0);
    if (name === undefined) {
      name = "invalid";
      this.add("E_TYPE_MISMATCH", "error", "collection name must be a string", node.span, path);
    }
    validateIdentifier(this.diagnostics, name, node.span, false, path);
    const required = booleanProperty(node, "required", false);
    let minItems = integerProperty(node, "min-items") ?? 0;
    if (required && minItems < 1) minItems = 1;
    const maxItems = integerProperty(node, "max-items");
    if (maxItems !== undefined && maxItems < minItems)
      this.add("E_COLLECTION_BOUNDS", "error", "max-items must be >= effective min-items", node.span, path);
    let onRowError = stringProperty(node, "on-row-error") ?? "fail";
    if (onRowError !== "fail" && onRowError !== "skip") {
      this.add("E_TYPE_MISMATCH", "error", "on-row-error must be fail or skip", node.span, path);
      onRowError = "fail";
    }
    const selects = node.children.filter((child) => child.name === "select");
    const members = node.children.filter((child) => child.name === "field" || child.name === "collection");
    for (const child of node.children)
      if (child.name !== "select" && child.name !== "field" && child.name !== "collection")
        this.add(
          "E_UNKNOWN_NODE",
          "error",
          `node ${JSON.stringify(child.name)} is not allowed in collection`,
          child.span,
          path,
        );
    let selector = "";
    if (selects.length !== 1)
      this.add(
        "E_SELECTOR_REQUIRED",
        "error",
        `collection requires exactly one select; found ${selects.length}`,
        node.span,
        `${path}.selection`,
      );
    else {
      validateNode(this.diagnostics, selects[0]!, 1, 1, {}, `${path}.selection`);
      selector = stringArgument(selects[0]!, 0) ?? "";
      this.checkSelector(selector, selects[0]!, `${path}.selection`);
    }
    if (members.length === 0)
      this.add("E_COLLECTION_EMPTY_SCHEMA", "error", "collection requires at least one output member", node.span, path);
    if (this.sourceMode(owner) === "browser") this.capabilities.add("browser.query");
    const base: CollectionIR = {
      kind: "collection",
      id: path,
      name,
      selector,
      required,
      minItems,
      onRowError: onRowError as "fail" | "skip",
      row: this.compileOutputObject(owner, members, `${path}[]`, scope),
      span: node.span,
    };
    return maxItems === undefined ? base : { ...base, maxItems };
  }

  private addReadCapability(owner: DocumentState, kind: string): void {
    if (this.sourceMode(owner) !== "browser") return;
    this.capabilities.add("browser.query");
    this.capabilities.add(`browser.read-${kind}`);
  }

  private sourceMode(state: DocumentState): string {
    const source = state.loaded.root?.children.find((node) => node.name === "source");
    const fetch = source?.children.find((node) => node.name === "fetch");
    return fetch === undefined ? "" : (stringProperty(fetch, "mode") ?? "");
  }

  private validateBuiltinArguments(name: string, input: TypeRef, node: Node, path: string): void {
    const flags = stringProperty(node, "flags");
    if (flags !== undefined) {
      const seen = new Set<string>();
      for (const flag of flags) {
        if (!["i", "m", "s"].includes(flag))
          this.add("E_REGEX_INVALID", "error", `unsupported regex flag ${JSON.stringify(flag)}`, node.span, path);
        if (seen.has(flag))
          this.add("E_REGEX_INVALID", "error", `duplicate regex flag ${JSON.stringify(flag)}`, node.span, path);
        seen.add(flag);
      }
    }
    const pattern = stringProperty(node, "pattern");
    if (pattern !== undefined) {
      if (pattern.includes("(?P<") || pattern.includes("(?<"))
        this.add(
          "E_REGEX_INVALID",
          "error",
          "named capture groups are outside the portable RE2 profile",
          node.span,
          path,
        );
      else {
        const lookaround = /\(\?(?:[=!]|<[=!])/u.exec(pattern)?.[0];
        if (lookaround !== undefined)
          this.add(
            "E_REGEX_INVALID",
            "error",
            `error parsing regexp: invalid or unsupported Perl syntax: \`${lookaround}\``,
            node.span,
            path,
          );
        else {
          try {
            compilePortableRegex(pattern, flags ?? "");
          } catch (error) {
            this.add(
              "E_REGEX_INVALID",
              "error",
              error instanceof Error ? error.message : String(error),
              node.span,
              path,
            );
          }
        }
      }
    }
    if (name === "parse-int") {
      const radix = integerProperty(node, "radix");
      if (radix !== undefined && (radix < 2 || radix > 36))
        this.add("E_TRANSFORM_ARGUMENT", "error", "parse-int radix must be between 2 and 36", node.span, path);
    } else if (name === "parse-bool") {
      if ((stringProperty(node, "true") ?? "true") === (stringProperty(node, "false") ?? "false"))
        this.add("E_TRANSFORM_ARGUMENT", "error", "parse-bool true and false values must differ", node.span, path);
    } else if (name === "coalesce") {
      const value = nodeProperty(node, "value")?.value;
      if (value !== undefined && input.kind === "nullable" && !valueAssignable(value, input.inner))
        this.add(
          "E_TRANSFORM_ARGUMENT",
          "error",
          `coalesce value is not assignable to ${typeString(input.inner)}`,
          value.span,
          path,
        );
    } else if (name === "assert-enum") {
      for (const value of node.arguments.slice(1))
        if (!valueAssignable(value, input))
          this.add(
            "E_TRANSFORM_ARGUMENT",
            "error",
            `enum value is not assignable to ${typeString(input)}`,
            value.span,
            path,
          );
    } else if (name === "url-resolve") {
      const base = stringProperty(node, "base");
      if (base !== undefined) {
        try {
          new URL(base);
        } catch {
          this.add("E_TRANSFORM_ARGUMENT", "error", "url-resolve base must be an absolute URL", node.span, path);
        }
      }
    }
  }

  private duration(node: Node, name: string, path: string): number | undefined {
    const property = nodeProperty(node, name);
    if (property?.value.kind !== "int") return undefined;
    const exact = property.value.integerValue ?? BigInt(Number(property.value.value));
    if (exact < 1n) {
      this.add("E_TYPE_MISMATCH", "error", `${name} must be positive`, node.span, path);
      return undefined;
    }
    if (exact > MAX_MILLISECONDS) {
      this.add("E_TYPE_MISMATCH", "error", `${name} must not exceed ${MAX_MILLISECONDS}`, node.span, path);
      return undefined;
    }
    return Number(exact);
  }

  private checkSelector(selector: string, node: Node, path: string): void {
    try {
      parseSelector(selector);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.add(
        message.includes("unsupported") ? "E_SELECTOR_UNSUPPORTED" : "E_SELECTOR_INVALID",
        "error",
        message,
        node.span,
        path,
      );
    }
  }

  private add(code: string, severity: Diagnostic["severity"], message: string, span: Span, path: string): void {
    this.diagnostics.push(path === "" ? { code, severity, message, span } : { code, severity, message, span, path });
  }

  private hasErrors(): boolean {
    return this.diagnostics.some((item) => item.severity === "error");
  }
}

export async function compileSource(source: Source, options: CompileOptions = {}): Promise<CompilerResult> {
  const graph = await loadSourceGraph(source, options);
  if (graph.entry === undefined) return { diagnostics: sortedDiagnostics(graph.diagnostics) };
  return new SemanticCompiler(graph.entry, graph.diagnostics).compile();
}

export async function validateSource(source: Source, options: CompileOptions = {}): Promise<readonly Diagnostic[]> {
  return (await compileSource(source, options)).diagnostics;
}

export function supportedLanguageVersions(): readonly string[] {
  return [LANGUAGE_VERSION];
}
export function supportedIRVersions(): readonly string[] {
  return [IR_VERSION];
}

function validateNode(
  diagnostics: Diagnostic[],
  node: Node,
  minArguments: number,
  maxArguments: number,
  allowed: Readonly<Record<string, Expectation>>,
  path: string,
): void {
  if (node.arguments.length < minArguments || (maxArguments >= 0 && node.arguments.length > maxArguments))
    diagnostics.push(
      diagnostic(
        "E_ARGUMENT_COUNT",
        `node ${JSON.stringify(node.name)} expects ${minArguments}..${maxArguments} positional arguments, got ${node.arguments.length}`,
        node.span,
        path,
      ),
    );
  const seen = new Set<string>();
  for (const property of node.properties) {
    if (seen.has(property.name))
      diagnostics.push(
        diagnostic("E_DUPLICATE_PROPERTY", `duplicate property ${JSON.stringify(property.name)}`, property.span, path),
      );
    seen.add(property.name);
    const expectation = allowed[property.name];
    if (expectation === undefined)
      diagnostics.push(
        diagnostic(
          "E_UNKNOWN_PROPERTY",
          `property ${JSON.stringify(property.name)} is not allowed on ${JSON.stringify(node.name)}`,
          property.span,
          path,
        ),
      );
    else if (!matchesExpectation(property.value, expectation))
      diagnostics.push(
        diagnostic(
          "E_TYPE_MISMATCH",
          `property ${JSON.stringify(property.name)} has incompatible value kind ${property.value.kind}`,
          property.value.span,
          path,
        ),
      );
  }
}

function matchesExpectation(value: Value, expectation: Expectation): boolean {
  if (expectation === "string") return value.kind === "string";
  if (expectation === "bool") return value.kind === "bool";
  if (expectation === "int") return value.kind === "int";
  if (expectation === "non-negative-int")
    return value.kind === "int" && (value.integerValue ?? BigInt(Number(value.value))) >= 0n;
  if (expectation === "number") return value.kind === "int" || value.kind === "float";
  return true;
}

function builtinAllowedProperties(name: string): Readonly<Record<string, Expectation>> {
  return (BUILTIN_CONTRACT[name]?.properties ?? {}) as Readonly<Record<string, BuiltinExpectation>>;
}

function builtinRequiredProperties(name: string): readonly string[] {
  return BUILTIN_CONTRACT[name]?.required ?? [];
}

function applyBuiltin(name: string, input: TypeRef, node: Node): TypeRef | Error {
  const stringToString = (): TypeRef | Error =>
    isString(input) ? primitive("string") : new Error(`requires string input, got ${typeString(input)}`);
  if (
    [
      "trim",
      "normalize-whitespace",
      "lowercase",
      "uppercase",
      "replace",
      "regex-replace",
      "substring",
      "prepend",
      "append",
      "url-resolve",
      "url-path",
      "assert-matches",
    ].includes(name)
  )
    return stringToString();
  if (["regex-capture", "empty-to-null", "url-query", "path-segment"].includes(name))
    return isString(input)
      ? nullable(primitive("string"))
      : new Error(`requires string input, got ${typeString(input)}`);
  if (name === "split")
    return isString(input)
      ? { kind: "array", element: primitive("string") }
      : new Error(`requires string input, got ${typeString(input)}`);
  if (name === "join")
    return input.kind === "array" && isString(input.element)
      ? primitive("string")
      : new Error(`requires string[] input, got ${typeString(input)}`);
  if (name === "parse-int") {
    if (!isString(input)) return new Error(`requires string input, got ${typeString(input)}`);
    const parsed = parseType(stringProperty(node, "as") ?? "");
    return parsed instanceof Error || !isIntegerType(parsed)
      ? new Error("property as must be an integer type")
      : parsed;
  }
  if (name === "parse-float") {
    if (!isString(input)) return new Error(`requires string input, got ${typeString(input)}`);
    const as = stringProperty(node, "as");
    return as === "float" || as === "f32" || as === "f64"
      ? primitive(as)
      : new Error("property as must be float, f32, or f64");
  }
  if (name === "parse-bool")
    return isString(input) ? primitive("bool") : new Error(`requires string input, got ${typeString(input)}`);
  if (name === "to-string")
    return isScalarType(input) && input.kind !== "nullable"
      ? primitive("string")
      : new Error(`requires non-null scalar input, got ${typeString(input)}`);
  if (name === "coalesce")
    return input.kind === "nullable" ? input.inner : new Error(`requires nullable input, got ${typeString(input)}`);
  if (name === "assert-enum")
    return isScalarType(input) ? input : new Error(`requires scalar input, got ${typeString(input)}`);
  if (name === "assert-min" || name === "assert-max")
    return isNumericType(input) ? input : new Error(`requires numeric input, got ${typeString(input)}`);
  return new Error(`unknown built-in ${JSON.stringify(name)}`);
}

function parseType(input: string): TypeRef | Error {
  let index = 0;
  const skip = (): void => {
    while (input[index] === " " || input[index] === "\t") index++;
  };
  const parse = (): TypeRef | Error => {
    skip();
    let type: TypeRef;
    if (input[index] === "(") {
      index++;
      const inner = parse();
      if (inner instanceof Error) return inner;
      skip();
      if (input[index] !== ")") return new Error("missing closing parenthesis");
      index++;
      type = inner;
    } else {
      const start = index;
      while (/[a-z0-9]/u.test(input[index] ?? "")) index++;
      const name = input.slice(start, index);
      if (
        ![
          "string",
          "bool",
          "int",
          "u8",
          "u16",
          "u32",
          "u64",
          "i8",
          "i16",
          "i32",
          "i64",
          "float",
          "f32",
          "f64",
          "object",
          "unknown",
        ].includes(name)
      )
        return new Error(`unknown primitive type ${JSON.stringify(name)}`);
      type = primitive(name as PrimitiveTypeName);
    }
    for (;;) {
      skip();
      if (input.slice(index, index + 2) === "[]") {
        index += 2;
        type = { kind: "array", element: type };
      } else if (input[index] === "?") {
        if (type.kind === "nullable") return new Error("nested nullable type is invalid");
        index++;
        type = nullable(type);
      } else break;
    }
    return type;
  };
  const result = parse();
  if (result instanceof Error) return result;
  skip();
  return index === input.length ? result : new Error(`unexpected type syntax at byte ${index}`);
}

function primitive(name: PrimitiveTypeName): Extract<TypeRef, { kind: "primitive" }> {
  return { kind: "primitive", name };
}
function nullable(inner: TypeRef): TypeRef {
  return inner.kind === "nullable" ? inner : { kind: "nullable", inner };
}
function typeString(type: TypeRef): string {
  return type.kind === "primitive"
    ? type.name
    : type.kind === "array"
      ? `${typeString(type.element)}[]`
      : `${typeString(type.inner)}?`;
}
function typeEqual(left: TypeRef, right: TypeRef): boolean {
  return typeString(left) === typeString(right);
}
function isAssignable(from: TypeRef, to: TypeRef): boolean {
  if (typeEqual(from, to) || (to.kind === "primitive" && to.name === "unknown")) return true;
  if (to.kind === "nullable")
    return from.kind === "nullable" ? isAssignable(from.inner, to.inner) : isAssignable(from, to.inner);
  return from.kind === "array" && to.kind === "array" && isAssignable(from.element, to.element);
}
function isString(type: TypeRef): boolean {
  return type.kind === "primitive" && type.name === "string";
}
function isIntegerType(type: TypeRef): boolean {
  return type.kind === "primitive" && ["int", "u8", "u16", "u32", "u64", "i8", "i16", "i32", "i64"].includes(type.name);
}
function isNumericType(type: TypeRef): boolean {
  return (
    type.kind === "primitive" &&
    ["int", "u8", "u16", "u32", "u64", "i8", "i16", "i32", "i64", "float", "f32", "f64"].includes(type.name)
  );
}
function isScalarType(type: TypeRef): boolean {
  return type.kind === "nullable"
    ? isScalarType(type.inner)
    : type.kind === "primitive" && type.name !== "object" && type.name !== "unknown";
}
function valueType(value: Value): TypeRef {
  return value.kind === "null"
    ? nullable(primitive("unknown"))
    : primitive(value.kind === "bool" ? "bool" : value.kind);
}
function valueAssignable(value: Value, target: TypeRef): boolean {
  return value.kind === "null"
    ? target.kind === "nullable" || (target.kind === "primitive" && target.name === "unknown")
    : isAssignable(valueType(value), target);
}
function inputDefaultCompatible(value: Value, type: string): boolean {
  return value.kind === type || (type === "float" && value.kind === "int");
}
function scalar(value: Value): JsonScalar {
  return value.value;
}

function nodeProperty(node: Node, name: string): Property | undefined {
  return [...node.properties].reverse().find((property) => property.name === name);
}
function stringArgument(node: Node, index: number): string | undefined {
  const value = node.arguments[index];
  return value?.kind === "string" ? String(value.value) : undefined;
}
function numberArgument(node: Node, index: number): number | undefined {
  const value = node.arguments[index];
  return value?.kind === "int" || value?.kind === "float" ? Number(value.value) : undefined;
}
function stringProperty(node: Node, name: string): string | undefined {
  const value = nodeProperty(node, name)?.value;
  return value?.kind === "string" ? String(value.value) : undefined;
}
function booleanProperty(node: Node, name: string, fallback: boolean): boolean {
  const value = nodeProperty(node, name)?.value;
  return value?.kind === "bool" ? Boolean(value.value) : fallback;
}
function integerProperty(node: Node, name: string): number | undefined {
  const value = nodeProperty(node, name)?.value;
  return value?.kind === "int" ? Number(value.value) : undefined;
}

function validateIdentifier(diagnostics: Diagnostic[], name: string, span: Span, root: boolean, path: string): void {
  if (!(root ? ROOT_NAME : USER_NAME).test(name))
    diagnostics.push(diagnostic("E_IDENTIFIER_INVALID", `invalid identifier ${JSON.stringify(name)}`, span, path));
}

function withOptional<T extends object, K extends string, V>(
  base: T,
  key: K,
  value: V | undefined,
): T & { readonly [P in K]?: V } {
  return value === undefined ? base : ({ ...base, [key]: value } as T & { readonly [P in K]?: V });
}

function diagnostic(code: string, message: string, span: Span, path: string): Diagnostic {
  return path === "" ? { code, severity: "error", message, span } : { code, severity: "error", message, span, path };
}
function sortedDiagnostics(diagnostics: readonly Diagnostic[]): readonly Diagnostic[] {
  return [...diagnostics].sort(
    (left, right) =>
      compareCodePoints(left.span.file, right.span.file) ||
      left.span.start.offset - right.span.start.offset ||
      compareCodePoints(left.code, right.code),
  );
}
function compareCodePoints(left: string, right: string): number {
  const leftPoints = Array.from(left, (character) => character.codePointAt(0)!);
  const rightPoints = Array.from(right, (character) => character.codePointAt(0)!);
  const length = Math.min(leftPoints.length, rightPoints.length);
  for (let index = 0; index < length; index++)
    if (leftPoints[index] !== rightPoints[index]) return leftPoints[index]! - rightPoints[index]!;
  return leftPoints.length - rightPoints.length;
}

function isDateIdentifier(value: string): boolean {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/u.exec(value);
  if (match === null) return false;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(0);
  date.setUTCHours(0, 0, 0, 0);
  date.setUTCFullYear(year, month - 1, day);
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}
