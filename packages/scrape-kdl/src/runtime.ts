import { attribute, innerHTML, parseHTML, queryAll, textContent, type DocumentNode, type ElementNode } from "./dom.js";
import { ExecutionError } from "./execution-error.js";
import type {
  ExtractorIR,
  FieldIR,
  JsonScalar,
  JsonValue,
  MatchTransformIR,
  OutputObjectIR,
  ResolvedTransformCallIR,
  TransformIR,
  TypeRef,
} from "./ir.js";
import { parseSelector } from "./selector.js";
import { compilePortableRegex, type PortableRegex } from "./portable-regex.js";
import { MissingValue } from "./output-semantics.js";
import { executeOutputObject, type OutputOperations } from "./output-walker.js";
import type { ExecutionOptions, ExtractionResult, Session, Warning } from "./public-api.js";

const DEFAULT_TIMEOUT_MS = 30_000;
const DEFAULT_MAX_RESPONSE_BYTES = 32 << 20;
const DEFAULT_USER_AGENT = "scrape-kdl/1.0";
const MAX_REDIRECTS = 20;

export interface RuntimeState {
  readonly ir: ExtractorIR;
  readonly plan: RuntimePlan;
  readonly options: ExecutionOptions;
  readonly transforms: ReadonlyMap<string, TransformIR>;
  readonly callStack: Set<string>;
  readonly signal?: AbortSignal;
  readonly warnings: Warning[];
  partial: boolean;
}

export interface RuntimePlan {
  readonly ir: ExtractorIR;
  readonly mode: "http" | "browser";
  readonly transforms: ReadonlyMap<string, TransformIR>;
  readonly selectors: ReadonlyMap<string, ReturnType<typeof parseSelector>>;
  readonly regexes: WeakMap<ResolvedTransformCallIR, PortableRegex>;
  readonly snapshotError?: ExecutionError;
}

export async function executeProgram(
  ir: ExtractorIR,
  inputs: Readonly<Record<string, JsonValue>> = {},
  options: ExecutionOptions = {},
  prepared?: RuntimePlan,
): Promise<ExtractionResult> {
  if (ir.source.fetch.mode === "browser") {
    const { executeBrowserProgram } = await import("./browser-runtime.js");
    return executeBrowserProgram(ir, inputs, options, prepared);
  }
  checkAbort(options.signal, "output");
  const state = preflightRuntime(ir, options, "http", prepared);
  const resolvedInputs = resolveInputs(ir, inputs);
  if (ir.source.sessionPolicy === "required" && options.session === undefined)
    throw new ExecutionError("E_SESSION_REQUIRED", "source requires a runtime session");
  const target = expandTemplate(ir, resolvedInputs);
  await enforceURLPolicy(options, target);
  const session = ir.source.sessionPolicy === "none" ? undefined : options.session;
  const fetchOptions = { ...options } as { -readonly [K in keyof ExecutionOptions]: ExecutionOptions[K] };
  if (session === undefined) delete fetchOptions.session;
  else fetchOptions.session = session;
  const html = await fetchHTML(target, fetchOptions);
  return executeDocument(state, parseHTML(html));
}

export async function executeHTML(
  ir: ExtractorIR,
  html: string,
  options: ExecutionOptions = {},
  prepared?: RuntimePlan,
): Promise<ExtractionResult> {
  checkAbort(options.signal, "output");
  return executeDocument(preflightRuntime(ir, options, "http", prepared), parseHTML(html));
}

export async function executeSnapshot(
  ir: ExtractorIR,
  html: string,
  options: ExecutionOptions = {},
  prepared?: RuntimePlan,
): Promise<ExtractionResult> {
  checkAbort(options.signal, "output");
  const mode = ir.source.fetch.mode;
  const plan = prepared ?? prepareRuntime(ir, mode);
  if (plan.snapshotError !== undefined) throw plan.snapshotError;
  return executeDocument(preflightRuntime(ir, options, mode, plan), parseHTML(html));
}

export function prepareRuntime(ir: ExtractorIR, mode: "http" | "browser"): RuntimePlan {
  if (ir.source.fetch.mode !== mode) {
    const code = mode === "http" ? "E_BROWSER_RUNTIME_MISSING" : "E_BROWSER_MODE_REQUIRED";
    throw new ExecutionError(
      code,
      `${mode === "http" ? "HTTP" : "browser"} runtime cannot execute fetch mode ${JSON.stringify(ir.source.fetch.mode)}`,
    );
  }
  const transforms = new Map<string, TransformIR>();
  for (const transform of ir.transforms) {
    if (transforms.has(transform.symbolId))
      throw new ExecutionError(
        "E_IR_INVALID",
        `duplicate declared transform symbol ${JSON.stringify(transform.symbolId)}`,
        { path: "transforms" },
      );
    transforms.set(transform.symbolId, transform);
  }
  const selectors = new Map<string, ReturnType<typeof parseSelector>>();
  preflightOutput(ir.output, mode, selectors);
  const snapshotError = snapshotCompatibilityError(ir);
  const plan = { ir, mode, transforms, selectors, regexes: new WeakMap<ResolvedTransformCallIR, PortableRegex>() };
  return snapshotError === undefined ? plan : { ...plan, snapshotError };
}

function snapshotCompatibilityError(ir: ExtractorIR): ExecutionError | undefined {
  if (ir.source.workflow.length > 0)
    return new ExecutionError(
      "E_SNAPSHOT_UNSUPPORTED",
      "offline snapshot execution cannot reproduce browser workflow",
      {
        path: "source.workflow",
      },
    );
  const inspect = (object: OutputObjectIR): ExecutionError | undefined => {
    for (const member of object.members) {
      if (member.kind === "field" && member.valueSource.kind === "javascript")
        return new ExecutionError(
          "E_SNAPSHOT_UNSUPPORTED",
          "offline snapshot execution cannot evaluate JavaScript value sources",
          { path: member.id },
        );
      if (member.kind === "collection") {
        const nested = inspect(member.row);
        if (nested !== undefined) return nested;
      }
    }
    return undefined;
  };
  return inspect(ir.output);
}

export function preflightRuntime(
  ir: ExtractorIR,
  options: ExecutionOptions,
  mode: "http" | "browser",
  prepared?: RuntimePlan,
): RuntimeState {
  const plan = prepared ?? prepareRuntime(ir, mode);
  if (plan.ir !== ir || plan.mode !== mode)
    throw new ExecutionError("E_IR_INVALID", "prepared runtime plan does not match the extractor and execution mode");
  for (const transform of plan.transforms.values()) {
    if (transform.kind === "external" && options.externalTransforms?.[transform.symbol] === undefined)
      throw new ExecutionError(
        "E_EXTERNAL_TRANSFORM_MISSING",
        `external transform ${JSON.stringify(transform.symbol)} is not registered`,
        { path: transform.name },
      );
  }
  const state: RuntimeState = {
    ir,
    plan,
    options,
    transforms: plan.transforms,
    callStack: new Set(),
    warnings: [],
    partial: false,
  };
  return options.signal === undefined ? state : { ...state, signal: options.signal };
}

function preflightOutput(
  object: OutputObjectIR,
  mode: "http" | "browser",
  selectors: Map<string, ReturnType<typeof parseSelector>>,
): void {
  for (const member of object.members) {
    if (member.kind === "field") {
      if (member.selection !== undefined) {
        try {
          selectors.set(member.selection.selector, parseSelector(member.selection.selector));
        } catch (error) {
          throw new ExecutionError("E_SELECTOR_INVALID", errorMessage(error), { path: member.id, cause: error });
        }
      }
      if (mode === "http" && member.valueSource.kind === "javascript")
        throw new ExecutionError("E_BROWSER_RUNTIME_MISSING", "HTTP runtime cannot execute JavaScript value sources", {
          path: member.id,
        });
    } else {
      try {
        selectors.set(member.selector, parseSelector(member.selector));
      } catch (error) {
        throw new ExecutionError("E_SELECTOR_INVALID", errorMessage(error), { path: member.id, cause: error });
      }
      preflightOutput(member.row, mode, selectors);
    }
  }
}

export function resolveInputs(ir: ExtractorIR, provided: Readonly<Record<string, JsonValue>>): Map<string, JsonScalar> {
  const known = new Map(ir.inputs.map((input) => [input.name, input]));
  const unknown = Object.keys(provided)
    .filter((name) => !known.has(name))
    .sort(compareCodePoints)[0];
  if (unknown !== undefined)
    throw new ExecutionError("E_INPUT_UNKNOWN", `unknown input ${JSON.stringify(unknown)}`, {
      path: `input.${unknown}`,
    });
  const output = new Map<string, JsonScalar>();
  for (const input of ir.inputs) {
    const value = Object.hasOwn(provided, input.name) ? provided[input.name] : input.default;
    if (value === undefined) {
      if (input.required)
        throw new ExecutionError("E_INPUT_REQUIRED", `required input ${JSON.stringify(input.name)} is missing`, {
          path: `input.${input.name}`,
        });
      continue;
    }
    if (!inputMatches(value, input.type))
      throw new ExecutionError(
        "E_INPUT_TYPE",
        `input ${JSON.stringify(input.name)} is not assignable to ${input.type}`,
        { path: `input.${input.name}` },
      );
    output.set(input.name, value as JsonScalar);
  }
  return output;
}

function inputMatches(value: JsonValue, type: string): boolean {
  if (type === "string") return typeof value === "string";
  if (type === "bool") return typeof value === "boolean";
  if (type === "int") return typeof value === "number" && Number.isSafeInteger(value);
  return type === "float" && typeof value === "number" && Number.isFinite(value);
}

export function expandTemplate(ir: ExtractorIR, inputs: ReadonlyMap<string, JsonScalar>): URL {
  let output = "";
  for (const segment of ir.source.fetch.urlTemplate.segments) {
    if (segment.kind === "literal") output += segment.value;
    else {
      const value = inputs.get(segment.name);
      if (value === undefined)
        throw new ExecutionError("E_INPUT_REQUIRED", `URL template input ${JSON.stringify(segment.name)} is missing`, {
          path: `input.${segment.name}`,
        });
      output += percentEncode(scalarString(value));
    }
  }
  try {
    const url = new URL(output);
    if ((url.protocol !== "http:" && url.protocol !== "https:") || url.host === "")
      throw new Error("unsupported target");
    return url;
  } catch (error) {
    throw new ExecutionError(
      "E_URL_INVALID",
      `expanded URL is not an absolute HTTP(S) URL: ${JSON.stringify(output)}`,
      { cause: error },
    );
  }
}

function scalarString(value: JsonScalar): string {
  if (value === null) throw new ExecutionError("E_INPUT_TYPE", "null cannot be used in a URL template");
  if (typeof value === "number" && !Number.isFinite(value))
    throw new ExecutionError("E_INPUT_TYPE", "float must be finite");
  return String(value);
}

function percentEncode(value: string): string {
  return [...new TextEncoder().encode(value)]
    .map((byte) =>
      (byte >= 0x61 && byte <= 0x7a) ||
      (byte >= 0x41 && byte <= 0x5a) ||
      (byte >= 0x30 && byte <= 0x39) ||
      [0x2d, 0x2e, 0x5f, 0x7e].includes(byte)
        ? String.fromCharCode(byte)
        : `%${byte.toString(16).toUpperCase().padStart(2, "0")}`,
    )
    .join("");
}

export async function enforceURLPolicy(options: ExecutionOptions, url: URL): Promise<void> {
  if (options.urlPolicy === undefined) return;
  try {
    await options.urlPolicy(options.signal === undefined ? {} : { signal: options.signal }, url);
  } catch (error) {
    throw new ExecutionError(
      "E_URL_POLICY",
      `URL ${JSON.stringify(url.href)} rejected by policy: ${errorMessage(error)}`,
      { cause: error },
    );
  }
}

async function fetchHTML(initialURL: URL, options: ExecutionOptions): Promise<string> {
  const fetchImplementation = options.fetch ?? globalThis.fetch;
  const timeoutMs =
    options.requestTimeoutMs !== undefined && options.requestTimeoutMs > 0
      ? options.requestTimeoutMs
      : DEFAULT_TIMEOUT_MS;
  const maxBytes =
    options.maxResponseBytes !== undefined && options.maxResponseBytes > 0
      ? options.maxResponseBytes
      : DEFAULT_MAX_RESPONSE_BYTES;
  const controller = new AbortController();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort(new DOMException("request timed out", "TimeoutError"));
  }, timeoutMs);
  const onAbort = (): void => controller.abort(options.signal?.reason);
  options.signal?.addEventListener("abort", onAbort, { once: true });
  let url = initialURL;
  let headers = sessionHeaders(options.session, url, options.userAgent ?? DEFAULT_USER_AGENT);
  try {
    for (let redirect = 0; redirect <= MAX_REDIRECTS; redirect++) {
      checkAbort(options.signal, "source.fetch");
      let response: Response;
      try {
        response = await fetchImplementation(url, {
          method: "GET",
          headers,
          redirect: "manual",
          signal: controller.signal,
        });
      } catch (error) {
        if (timedOut) throw new ExecutionError("E_TIMEOUT", "request timed out", { cause: error });
        if (options.signal?.aborted === true)
          throw new ExecutionError("E_EXECUTION_CANCELED", errorMessage(options.signal.reason), { cause: error });
        throw new ExecutionError("E_HTTP_FETCH", errorMessage(error), { cause: error });
      }
      if (isRedirect(response.status)) {
        const location = response.headers.get("location");
        await response.body?.cancel();
        if (location === null)
          throw new ExecutionError("E_HTTP_STATUS", `redirect status ${response.status} has no Location header`);
        if (redirect === MAX_REDIRECTS)
          throw new ExecutionError("E_HTTP_FETCH", `stopped after ${MAX_REDIRECTS} redirects`);
        let next: URL;
        try {
          next = new URL(location, url);
          if ((next.protocol !== "http:" && next.protocol !== "https:") || next.host === "")
            throw new Error("unsupported redirect target");
        } catch (error) {
          throw new ExecutionError("E_HTTP_FETCH", `invalid redirect target ${JSON.stringify(location)}`, {
            cause: error,
          });
        }
        await enforceURLPolicy(options, next);
        const sameOrigin = next.origin === url.origin;
        headers = new Headers(headers);
        if (!sameOrigin) {
          for (const name of ["authorization", "proxy-authorization", "cookie", "cookie2"]) headers.delete(name);
          applySessionCookies(headers, options.session, next, false);
        }
        url = next;
        continue;
      }
      if (!response.ok) {
        await response.body?.cancel();
        throw new ExecutionError(
          "E_HTTP_STATUS",
          `unexpected HTTP status ${response.status} ${response.statusText}`.trim(),
        );
      }
      const bytes = await readBoundedBody(response, maxBytes, controller.signal, () => timedOut, options.signal);
      return decodeHTML(bytes, response.headers.get("content-type") ?? "");
    }
    throw new ExecutionError("E_HTTP_FETCH", `stopped after ${MAX_REDIRECTS} redirects`);
  } finally {
    clearTimeout(timer);
    options.signal?.removeEventListener("abort", onAbort);
  }
}

function sessionHeaders(session: Session | undefined, url: URL, userAgent: string): Headers {
  const headers = new Headers();
  for (const name of Object.keys(session?.headers ?? {}).sort(compareCodePoints)) {
    const values = session!.headers![name]!;
    for (const value of typeof values === "string" ? [values] : values) headers.append(name, value);
  }
  if (!headers.has("user-agent")) headers.set("user-agent", userAgent);
  applySessionCookies(headers, session, url, true);
  return headers;
}

function applySessionCookies(headers: Headers, session: Session | undefined, url: URL, retainHeader: boolean): void {
  const values: string[] = [];
  if (retainHeader && headers.has("cookie")) values.push(headers.get("cookie")!);
  else headers.delete("cookie");
  for (const cookie of session?.cookies ?? [])
    if (cookieApplies(cookie, url, retainHeader)) values.push(`${cookie.name}=${cookie.value}`);
  if (values.length > 0) headers.set("cookie", values.join("; "));
}

function cookieApplies(cookie: NonNullable<Session["cookies"]>[number], url: URL, allowHostOnly: boolean): boolean {
  if (cookie.secure === true && url.protocol !== "https:") return false;
  if (cookie.expires !== undefined && cookie.expires <= Date.now() / 1000) return false;
  const host = url.hostname.toLowerCase();
  if (cookie.domain === undefined && !allowHostOnly) return false;
  if (cookie.domain !== undefined) {
    const domain = cookie.domain.toLowerCase().replace(/^\./u, "");
    if (host !== domain && !host.endsWith(`.${domain}`)) return false;
  }
  return cookie.path === undefined || url.pathname.startsWith(cookie.path);
}

async function readBoundedBody(
  response: Response,
  maximum: number,
  signal: AbortSignal,
  timedOut: () => boolean,
  parent: AbortSignal | undefined,
): Promise<Uint8Array> {
  if (response.body === null) return new Uint8Array();
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      let result: ReadableStreamReadResult<Uint8Array>;
      try {
        result = await reader.read();
      } catch (error) {
        if (timedOut() || signal.reason?.name === "TimeoutError")
          throw new ExecutionError("E_TIMEOUT", "request timed out", { cause: error });
        if (parent?.aborted === true)
          throw new ExecutionError("E_EXECUTION_CANCELED", errorMessage(parent.reason), { cause: error });
        throw new ExecutionError("E_HTTP_READ", errorMessage(error), { cause: error });
      }
      if (result.done) break;
      size += result.value.byteLength;
      if (size > maximum) {
        await reader.cancel();
        throw new ExecutionError("E_HTTP_BODY_TOO_LARGE", `response exceeds ${maximum} bytes`);
      }
      chunks.push(result.value);
    }
  } finally {
    reader.releaseLock();
  }
  const output = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    output.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return output;
}

function isRedirect(status: number): boolean {
  return status === 301 || status === 302 || status === 303 || status === 307 || status === 308;
}

function decodeHTML(bytes: Uint8Array, contentType: string): string {
  let charset = charsetFromContentType(contentType) ?? sniffMetaCharset(bytes) ?? "utf-8";
  if (startsWith(bytes, [0xef, 0xbb, 0xbf])) {
    bytes = bytes.slice(3);
    charset = "utf-8";
  } else if (startsWith(bytes, [0xff, 0xfe])) {
    bytes = bytes.slice(2);
    charset = "utf-16le";
  } else if (startsWith(bytes, [0xfe, 0xff])) {
    bytes = bytes.slice(2);
    charset = "utf-16be";
  }
  charset = normalizeCharset(charset);
  if (charset === "us-ascii" && bytes.some((byte) => byte > 0x7f))
    throw new ExecutionError("E_HTML_DECODE", "response is not valid US-ASCII");
  if (charset === "iso-8859-1") return Array.from(bytes, (byte) => String.fromCodePoint(byte)).join("");
  if (charset === "windows-1252") return decodeWindows1252(bytes);
  try {
    return new TextDecoder(charset, { fatal: true }).decode(bytes);
  } catch (error) {
    if (error instanceof RangeError)
      throw new ExecutionError("E_HTML_CHARSET_UNSUPPORTED", `unsupported HTML charset ${JSON.stringify(charset)}`, {
        cause: error,
      });
    throw new ExecutionError("E_HTML_DECODE", `response is not valid ${charset.toUpperCase()}`, { cause: error });
  }
}

function charsetFromContentType(contentType: string): string | undefined {
  const match = /(?:^|;)\s*charset\s*=\s*(?:"([^"]+)"|'([^']+)'|([^;\s]+))/iu.exec(contentType);
  return match?.slice(1).find((value) => value !== undefined);
}

function sniffMetaCharset(bytes: Uint8Array): string | undefined {
  const source = Array.from(bytes.slice(0, 4096), (byte) => String.fromCharCode(byte)).join("");
  const match =
    /<meta\s+[^>]*(?:charset\s*=\s*["']?\s*([^\s"'/>;]+)|content\s*=\s*["'][^"']*charset\s*=\s*([^\s"'/>;]+))/iu.exec(
      source,
    );
  return match?.slice(1).find((value) => value !== undefined);
}

function normalizeCharset(value: string): string {
  const normalized = value.trim().toLowerCase();
  if (normalized === "utf8") return "utf-8";
  if (normalized === "ascii") return "us-ascii";
  if (["latin1", "latin-1", "iso8859-1"].includes(normalized)) return "iso-8859-1";
  if (normalized === "cp1252") return "windows-1252";
  if (["utf16", "utf_16", "utf16le", "utf_16le"].includes(normalized)) return "utf-16le";
  if (["utf16be", "utf_16be"].includes(normalized)) return "utf-16be";
  return normalized;
}

const WINDOWS_1252 = [
  0x20ac, 0x0081, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021, 0x02c6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008d,
  0x017d, 0x008f, 0x0090, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x02dc, 0x2122, 0x0161, 0x203a,
  0x0153, 0x009d, 0x017e, 0x0178,
];
function decodeWindows1252(bytes: Uint8Array): string {
  return Array.from(bytes, (byte) =>
    String.fromCodePoint(byte >= 0x80 && byte <= 0x9f ? WINDOWS_1252[byte - 0x80]! : byte),
  ).join("");
}
function startsWith(bytes: Uint8Array, prefix: readonly number[]): boolean {
  return prefix.every((byte, index) => bytes[index] === byte);
}

async function executeDocument(state: RuntimeState, document: DocumentNode): Promise<ExtractionResult> {
  const value = await executeOutputObject(domOutputOperations(state), document, state.ir.output, "output");
  const warnings = [...state.warnings];
  if (state.partial)
    warnings.push({ code: "W_PARTIAL_EXTRACTION", message: "extraction completed with one or more recovered errors" });
  return { value, warnings, partial: state.partial };
}

function domOutputOperations(state: RuntimeState): OutputOperations<DocumentNode | ElementNode> {
  return {
    state,
    check: (path) => checkCanceled(state, path),
    readField: (scope, field, path) => readFieldValue(state, scope, field, path),
    queryRows: (scope, collection) => queryAll(scope, state.plan.selectors.get(collection.selector)!),
    applyTransforms: (value, calls, path) => applyCalls(state, value, calls, path),
    validateField(value, field, path) {
      if (!matchesRuntimeType(value, field.successfulType))
        throw new ExecutionError("E_OUTPUT_TYPE", `value is not assignable to ${typeString(field.successfulType)}`, {
          path,
        });
    },
  };
}

function readFieldValue(
  state: RuntimeState,
  scope: DocumentNode | ElementNode,
  field: FieldIR,
  path: string,
): JsonValue {
  let selected: ElementNode | undefined;
  if (field.selection !== undefined) {
    const matches = queryAll(
      scope,
      state.plan.selectors.get(field.selection.selector)!,
      field.selection.match === "one" ? 2 : 1,
    );
    if (matches.length === 0)
      throw new MissingValue(`selector ${JSON.stringify(field.selection.selector)} matched no elements`);
    if (field.selection.match === "one" && matches.length !== 1)
      throw new ExecutionError(
        "E_SELECTOR_CARDINALITY",
        `selector ${JSON.stringify(field.selection.selector)} matched ${matches.length} elements; expected exactly one`,
        { path },
      );
    selected = matches[0];
  }
  if (field.valueSource.kind === "text") {
    if (selected === undefined)
      throw new ExecutionError("E_IR_INVALID", "text value source has no selected element", { path });
    return textContent(selected);
  }
  if (field.valueSource.kind === "html") {
    if (selected === undefined)
      throw new ExecutionError("E_IR_INVALID", "HTML value source has no selected element", { path });
    return innerHTML(selected);
  }
  if (field.valueSource.kind === "attribute") {
    if (selected === undefined)
      throw new ExecutionError("E_IR_INVALID", "attribute value source has no selected element", { path });
    const value = attribute(selected, field.valueSource.name);
    if (value === undefined) throw new MissingValue(`attribute ${JSON.stringify(field.valueSource.name)} is missing`);
    return value;
  }
  throw new ExecutionError("E_BROWSER_RUNTIME_MISSING", "HTTP runtime cannot evaluate JavaScript", { path });
}

export async function applyCalls(
  state: RuntimeState,
  input: JsonValue,
  calls: readonly ResolvedTransformCallIR[],
  path: string,
): Promise<JsonValue> {
  let value = input;
  for (const call of calls) {
    checkCanceled(state, path);
    try {
      value =
        call.target.kind === "builtin"
          ? applyBuiltin(state, call.target.name, value, call)
          : await applyDeclared(state, call.target.symbolId, value, path);
      if (!matchesRuntimeType(value, call.output))
        throw new Error(`transform result is not assignable to ${typeString(call.output)}`);
    } catch (error) {
      if (error instanceof ExecutionError) throw error;
      throw new ExecutionError("E_TRANSFORM", errorMessage(error), { path, cause: error });
    }
  }
  return value;
}

async function applyDeclared(
  state: RuntimeState,
  symbolId: string,
  input: JsonValue,
  path: string,
): Promise<JsonValue> {
  const transform = state.transforms.get(symbolId);
  if (transform === undefined) throw new Error(`unknown declared transform ${JSON.stringify(symbolId)}`);
  if (state.callStack.has(symbolId)) throw new Error(`recursive transform invocation ${JSON.stringify(symbolId)}`);
  state.callStack.add(symbolId);
  try {
    if (transform.kind === "pipeline") return applyCalls(state, input, transform.calls, path);
    if (transform.kind === "match") return applyMatch(transform, input);
    const implementation = state.options.externalTransforms?.[transform.symbol];
    if (implementation === undefined)
      throw new ExecutionError(
        "E_EXTERNAL_TRANSFORM_MISSING",
        `external transform ${JSON.stringify(transform.symbol)} is not registered`,
        { path },
      );
    const context = state.signal === undefined ? {} : { signal: state.signal };
    const result = await implementation(context, input);
    if (!isJSONCompatible(result) || !matchesRuntimeType(result, transform.output))
      throw new Error(`external transform result is not assignable to ${typeString(transform.output)}`);
    return result;
  } finally {
    state.callStack.delete(symbolId);
  }
}

function applyMatch(transform: MatchTransformIR, input: JsonValue): JsonValue {
  const matched = transform.cases.find((item) => scalarEqual(item.when, input));
  return matched?.then ?? transform.default;
}

// Built-in transforms are pure so HTTP and browser execution share semantics.
function applyBuiltin(state: RuntimeState, name: string, input: JsonValue, call: ResolvedTransformCallIR): JsonValue {
  const arguments_ = Object.fromEntries(call.namedArguments.map((item) => [item.name, item.value]));
  if (name === "trim") return requireString(input).trim();
  if (name === "normalize-whitespace") return requireString(input).trim().split(/\s+/u).filter(Boolean).join(" ");
  if (name === "lowercase") return requireString(input).toLowerCase();
  if (name === "uppercase") return requireString(input).toUpperCase();
  if (name === "replace")
    return replaceCount(
      requireString(input),
      requiredString(arguments_, "old"),
      requiredString(arguments_, "new"),
      optionalInteger(arguments_, "count", -1),
    );
  if (name === "regex-replace")
    return portableRegex(state, call, arguments_).replace(
      requireString(input),
      requiredString(arguments_, "replacement"),
      optionalInteger(arguments_, "count", -1),
    );
  if (name === "regex-capture") {
    const group = optionalInteger(arguments_, "group", 0);
    if (group < 0) throw new Error('argument "group" must be non-negative');
    const match = portableRegex(state, call, arguments_).exec(requireString(input));
    return match?.[group] ?? null;
  }
  if (name === "substring") {
    const runes = Array.from(requireString(input));
    const start = normalizeIndex(requiredInteger(arguments_, "start"), runes.length);
    const end = normalizeIndex(optionalInteger(arguments_, "end", runes.length), runes.length);
    return runes.slice(start, Math.max(start, end)).join("");
  }
  if (name === "split") {
    const value = requireString(input);
    const separator = requiredString(arguments_, "separator");
    const limit = optionalInteger(arguments_, "limit", -1);
    if (limit < -1) throw new Error('argument "limit" must be non-negative');
    const parts = separator === "" ? Array.from(value) : value.split(separator);
    return limit < 0 ? parts : parts.slice(0, limit);
  }
  if (name === "join") {
    if (!Array.isArray(input) || input.some((value) => typeof value !== "string")) throw new Error("expected string[]");
    return input.join(requiredString(arguments_, "separator"));
  }
  if (name === "prepend") return requiredString(arguments_, "value") + requireString(input);
  if (name === "append") return requireString(input) + requiredString(arguments_, "value");
  if (name === "parse-int")
    return parseInteger(
      requireString(input),
      requiredString(arguments_, "as"),
      optionalInteger(arguments_, "radix", 10),
    );
  if (name === "parse-float") return parseFloatValue(requireString(input), requiredString(arguments_, "as"));
  if (name === "parse-bool") return parseBoolean(requireString(input), arguments_);
  if (name === "to-string") return scalarString(input as JsonScalar);
  if (name === "empty-to-null") {
    const value = requireString(input);
    return value === "" ? null : value;
  }
  if (name === "coalesce") return input ?? requiredArgument(arguments_, "value");
  if (name === "url-resolve") return new URL(requireString(input), requiredString(arguments_, "base")).href;
  if (name === "url-query")
    return (
      new URL(requireString(input)).searchParams.getAll(requiredString(arguments_, "name"))[
        optionalInteger(arguments_, "index", 0)
      ] ?? null
    );
  if (name === "url-path") return decodeURIComponent(new URL(requireString(input)).pathname);
  if (name === "path-segment") {
    const value = requireString(input);
    let path: string;
    try {
      path = new URL(value).pathname;
    } catch {
      path = value;
    }
    const parts = path.split("/").filter(Boolean).map(decodeURIComponent);
    let index = requiredInteger(arguments_, "index");
    if (index < 0) index += parts.length;
    return parts[index] ?? null;
  }
  if (name === "assert-matches") {
    if (!portableRegex(state, call, arguments_).test(requireString(input)))
      throw new Error("value does not match required pattern");
    return input;
  }
  if (name === "assert-enum") {
    if (!call.positionalArguments.some((value) => scalarEqual(value, input)))
      throw new Error("value is outside the allowed enum");
    return input;
  }
  if (name === "assert-min" || name === "assert-max") {
    if (typeof input !== "number") throw new Error("expected number");
    const bound = requiredArgument(arguments_, "value");
    if (typeof bound !== "number") throw new Error("numeric bound is required");
    if (name === "assert-min" && input < bound) throw new Error("value is less than minimum");
    if (name === "assert-max" && input > bound) throw new Error("value is greater than maximum");
    return input;
  }
  throw new Error(`unknown built-in ${JSON.stringify(name)}`);
}

function requireString(value: JsonValue): string {
  if (typeof value !== "string") throw new Error(`expected string, got ${jsonType(value)}`);
  return value;
}
function requiredArgument(arguments_: Readonly<Record<string, JsonScalar>>, name: string): JsonScalar {
  const value = arguments_[name];
  if (value === undefined) throw new Error(`argument ${JSON.stringify(name)} is required`);
  return value;
}
function requiredString(arguments_: Readonly<Record<string, JsonScalar>>, name: string): string {
  const value = requiredArgument(arguments_, name);
  if (typeof value !== "string") throw new Error(`argument ${JSON.stringify(name)} must be a string`);
  return value;
}
function requiredInteger(arguments_: Readonly<Record<string, JsonScalar>>, name: string): number {
  const value = requiredArgument(arguments_, name);
  if (typeof value !== "number" || !Number.isInteger(value))
    throw new Error(`argument ${JSON.stringify(name)} must be an integer`);
  return value;
}
function optionalInteger(arguments_: Readonly<Record<string, JsonScalar>>, name: string, fallback: number): number {
  return arguments_[name] === undefined ? fallback : requiredInteger(arguments_, name);
}

function replaceCount(value: string, oldValue: string, newValue: string, count: number): string {
  if (count === 0) return value;
  if (oldValue === "") {
    const characters = Array.from(value);
    let output = "";
    let replacements = 0;
    for (let index = 0; index <= characters.length; index++) {
      if (count < 0 || replacements < count) {
        output += newValue;
        replacements++;
      }
      if (index < characters.length) output += characters[index];
    }
    return output;
  }
  if (count < 0) return value.split(oldValue).join(newValue);
  let output = value;
  let start = 0;
  for (let index = 0; index < count; index++) {
    const found = output.indexOf(oldValue, start);
    if (found < 0) break;
    output = output.slice(0, found) + newValue + output.slice(found + oldValue.length);
    start = found + newValue.length;
  }
  return output;
}

function portableRegex(
  state: RuntimeState,
  call: ResolvedTransformCallIR,
  arguments_: Readonly<Record<string, JsonScalar>>,
): PortableRegex {
  const cached = state.plan.regexes.get(call);
  if (cached !== undefined) return cached;
  const pattern = requiredString(arguments_, "pattern");
  const flags = typeof arguments_.flags === "string" ? arguments_.flags : "";
  if (pattern.includes("(?P<") || pattern.includes("(?<"))
    throw new Error("named capture groups are outside the portable RE2 profile");
  const compiled = compilePortableRegex(pattern, flags);
  state.plan.regexes.set(call, compiled);
  return compiled;
}

function normalizeIndex(index: number, length: number): number {
  return Math.max(0, Math.min(length, index < 0 ? length + index : index));
}

function parseInteger(value: string, type: string, radix: number): number {
  if (value === "" || value.includes("_") || radix < 2 || radix > 36)
    throw new Error(`invalid integer ${JSON.stringify(value)}`);
  const negative = value.startsWith("-");
  if (negative && type.startsWith("u")) throw new Error("unsigned integer rejects negative input");
  const signless = value.replace(/^[+-]/u, "");
  const operation = type.startsWith("u") ? "ParseUint" : "ParseInt";
  const parsedInput = type.startsWith("u") ? value.replace(/^\+/u, "") : value;
  if (
    signless === "" ||
    [...signless.toLowerCase()].some((character) => digitValue(character) < 0 || digitValue(character) >= radix)
  ) {
    throw new Error(`parse ${type}: strconv.${operation}: parsing ${JSON.stringify(parsedInput)}: invalid syntax`);
  }
  let parsed = 0n;
  for (const character of signless.toLowerCase()) parsed = parsed * BigInt(radix) + BigInt(digitValue(character));
  if (negative) parsed = -parsed;
  const ranges: Readonly<Record<string, readonly [bigint, bigint]>> = {
    i8: [-128n, 127n],
    u8: [0n, 255n],
    i16: [-32768n, 32767n],
    u16: [0n, 65535n],
    i32: [-2147483648n, 2147483647n],
    u32: [0n, 4294967295n],
    int: [-(1n << 63n), (1n << 63n) - 1n],
    i64: [-(1n << 63n), (1n << 63n) - 1n],
    u64: [0n, (1n << 64n) - 1n],
  };
  const range = ranges[type];
  if (range === undefined || parsed < range[0] || parsed > range[1])
    throw new Error(`parse ${type}: strconv.${operation}: parsing ${JSON.stringify(parsedInput)}: value out of range`);
  const result = Number(parsed);
  if (!Number.isSafeInteger(result))
    throw new Error(`parse ${type}: value is outside the TypeScript safe integer range`);
  return result;
}

function digitValue(character: string): number {
  const code = character.codePointAt(0)!;
  if (code >= 48 && code <= 57) return code - 48;
  if (code >= 97 && code <= 122) return code - 87;
  return -1;
}

function parseFloatValue(value: string, type: string): number {
  if (!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/u.test(value))
    throw new Error(`invalid finite decimal float ${JSON.stringify(value)}`);
  let output = Number(value);
  if (!Number.isFinite(output)) throw new Error(`parse ${type}: value out of range`);
  if (type === "f32") output = Math.fround(output);
  else if (type !== "float" && type !== "f64") throw new Error(`unsupported float target ${JSON.stringify(type)}`);
  if (!Number.isFinite(output)) throw new Error(`parse ${type}: value out of range`);
  return output;
}

function parseBoolean(value: string, arguments_: Readonly<Record<string, JsonScalar>>): boolean {
  const sensitive = arguments_["case-sensitive"] === true;
  let trueValue = typeof arguments_.true === "string" ? arguments_.true : "true";
  let falseValue = typeof arguments_.false === "string" ? arguments_.false : "false";
  let compared = value;
  if (!sensitive) {
    compared = compared.toLowerCase();
    trueValue = trueValue.toLowerCase();
    falseValue = falseValue.toLowerCase();
  }
  if (compared === trueValue) return true;
  if (compared === falseValue) return false;
  throw new Error(`${JSON.stringify(value)} is neither configured true nor false value`);
}

export function matchesRuntimeType(value: JsonValue, type: TypeRef): boolean {
  if (type.kind === "nullable") return value === null || matchesRuntimeType(value, type.inner);
  if (type.kind === "array")
    return Array.isArray(value) && value.every((item) => matchesRuntimeType(item, type.element));
  if (type.name === "unknown") return isJSONCompatible(value);
  if (type.name === "object") return value !== null && typeof value === "object" && !Array.isArray(value);
  if (type.name === "string") return typeof value === "string";
  if (type.name === "bool") return typeof value === "boolean";
  if (["float", "f64"].includes(type.name)) return typeof value === "number" && Number.isFinite(value);
  if (type.name === "f32") return typeof value === "number" && Number.isFinite(value) && Math.fround(value) === value;
  if (typeof value !== "number" || !Number.isSafeInteger(value)) return false;
  const ranges: Readonly<Record<string, readonly [number, number]>> = {
    i8: [-128, 127],
    u8: [0, 255],
    i16: [-32768, 32767],
    u16: [0, 65535],
    i32: [-2147483648, 2147483647],
    u32: [0, 4294967295],
  };
  const range = ranges[type.name];
  return range === undefined || (value >= range[0] && value <= range[1]);
}

export function isJSONCompatible(value: unknown): value is JsonValue {
  if (value === null || typeof value === "string" || typeof value === "boolean") return true;
  if (typeof value === "number") return Number.isFinite(value);
  if (Array.isArray(value)) return value.every(isJSONCompatible);
  if (typeof value === "object")
    return Object.getPrototypeOf(value) === Object.prototype && Object.values(value).every(isJSONCompatible);
  return false;
}

function scalarEqual(left: JsonScalar, right: JsonValue): boolean {
  return typeof right !== "object" && Object.is(left, right);
}
function typeString(type: TypeRef): string {
  return type.kind === "primitive"
    ? type.name
    : type.kind === "array"
      ? `${typeString(type.element)}[]`
      : `${typeString(type.inner)}?`;
}
function jsonType(value: JsonValue): string {
  return value === null ? "null" : Array.isArray(value) ? "array" : typeof value;
}
function checkCanceled(state: RuntimeState, path: string): void {
  checkAbort(state.signal, path);
}
export function checkAbort(signal: AbortSignal | undefined, path: string): void {
  if (signal?.aborted === true)
    throw new ExecutionError("E_EXECUTION_CANCELED", errorMessage(signal.reason), { path, cause: signal.reason });
}
export function executionMessage(error: unknown): string {
  return error instanceof ExecutionError && error.path !== undefined
    ? `${error.code} at ${error.path}: ${error.message}`
    : errorMessage(error);
}
export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error ?? "operation canceled");
}
function compareCodePoints(left: string, right: string): number {
  const a = Array.from(left);
  const b = Array.from(right);
  for (let index = 0; index < Math.min(a.length, b.length); index++) {
    const difference = a[index]!.codePointAt(0)! - b[index]!.codePointAt(0)!;
    if (difference !== 0) return difference;
  }
  return a.length - b.length;
}
