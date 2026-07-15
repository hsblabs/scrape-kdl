import {
  compileSource,
  supportedIRVersions as compilerSupportedIRVersions,
  supportedLanguageVersions as compilerSupportedLanguageVersions,
} from "./compiler.js";
import { executeProgram, prepareRuntime, type RuntimePlan } from "./runtime.js";
import type { DiagnosticIR, ExtractorIR, JsonValue } from "./ir.js";

export type { DiagnosticIR, ExtractorIR, JsonValue } from "./ir.js";
export { ExecutionError } from "./execution-error.js";

export interface Source {
  readonly path: string;
  readonly data: string | Uint8Array;
}

export interface SourceLoadContext {
  readonly fromPath: string;
  readonly signal?: AbortSignal;
}

export interface SourceLoader {
  load(path: string, context: SourceLoadContext): Promise<string | Uint8Array>;
}

export interface CompileOptions {
  readonly loader?: SourceLoader;
  readonly signal?: AbortSignal;
}

export interface CompileResult {
  readonly program?: Program;
  readonly diagnostics: readonly DiagnosticIR[];
}

export interface SourceFile {
  readonly path: string;
  readonly moduleName?: string;
  readonly moduleVersion?: string;
  readonly sha256: string;
}

export interface ProgramMetadata {
  readonly name: string;
  readonly version: string;
  readonly languageVersion: string;
  readonly irVersion: string;
  readonly files: readonly SourceFile[];
  readonly capabilities: readonly string[];
}

export interface Program {
  readonly metadata: ProgramMetadata;
  readonly ir: ExtractorIR;
  extract(inputs?: Readonly<Record<string, JsonValue>>, options?: ExecutionOptions): Promise<ExtractionResult>;
}

export async function compile(source: Source, options: CompileOptions = {}): Promise<CompileResult> {
  options.signal?.throwIfAborted();
  const result = await compileSource(source, options);
  options.signal?.throwIfAborted();
  if (result.ir === undefined) return { diagnostics: result.diagnostics };
  return { program: new ProgramSnapshot(result.ir), diagnostics: result.diagnostics };
}

export async function validate(source: Source, options: CompileOptions = {}): Promise<readonly DiagnosticIR[]> {
  return (await compile(source, options)).diagnostics;
}

export function supportedLanguageVersions(): readonly string[] {
  return Object.freeze([...compilerSupportedLanguageVersions()]);
}

export function supportedIRVersions(): readonly string[] {
  return Object.freeze([...compilerSupportedIRVersions()]);
}

export interface ExternalTransformContext {
  readonly signal?: AbortSignal;
}

export type ExternalTransform = (context: ExternalTransformContext, input: JsonValue) => JsonValue | Promise<JsonValue>;

export interface SessionCookie {
  readonly name: string;
  readonly value: string;
  readonly domain?: string;
  readonly path?: string;
  readonly expires?: number;
  readonly httpOnly?: boolean;
  readonly secure?: boolean;
  readonly sameSite?: "Strict" | "Lax" | "None";
}

export interface Session {
  readonly headers?: Readonly<Record<string, string | readonly string[]>>;
  readonly cookies?: readonly SessionCookie[];
}

export type BrowserElement = object;

export interface BrowserNavigateOptions {
  readonly timeoutMs: number;
  readonly session?: Session;
  readonly userAgent?: string;
  readonly signal?: AbortSignal;
}

export interface BrowserEvaluateOptions {
  readonly timeoutMs: number;
  readonly scope?: BrowserElement;
  readonly signal?: AbortSignal;
}

export interface BrowserOperationOptions {
  readonly timeoutMs: number;
  readonly signal?: AbortSignal;
}

export interface BrowserAdapter {
  navigate(url: string, options: BrowserNavigateOptions): Promise<void>;
  waitFor(
    selector: string,
    state: "attached" | "visible" | "hidden" | "detached",
    options: BrowserOperationOptions,
  ): Promise<void>;
  click(selector: string, options: BrowserOperationOptions): Promise<void>;
  fill(selector: string, value: string, options: BrowserOperationOptions): Promise<void>;
  press(selector: string, key: string, options: BrowserOperationOptions): Promise<void>;
  scroll(x: number, y: number, options: Pick<BrowserOperationOptions, "signal">): Promise<void>;
  waitForNetworkIdle(idleMs: number, options: BrowserOperationOptions): Promise<void>;
  evaluate(source: string, options: BrowserEvaluateOptions): Promise<JsonValue>;
  queryAll(
    scope: BrowserElement | undefined,
    selector: string,
    options: Pick<BrowserOperationOptions, "signal">,
  ): Promise<readonly BrowserElement[]>;
  text(element: BrowserElement, options: Pick<BrowserOperationOptions, "signal">): Promise<string>;
  html(element: BrowserElement, options: Pick<BrowserOperationOptions, "signal">): Promise<string>;
  attribute(
    element: BrowserElement,
    name: string,
    options: Pick<BrowserOperationOptions, "signal">,
  ): Promise<string | undefined>;
}

export interface BrowserAdapterLease {
  acquire(signal?: AbortSignal): Promise<() => void>;
}

export interface BrowserAdapterQueryLimit {
  queryLimit(
    scope: BrowserElement | undefined,
    selector: string,
    limit: number,
    options: Pick<BrowserOperationOptions, "signal">,
  ): Promise<readonly BrowserElement[]>;
}

export interface URLPolicyContext {
  readonly signal?: AbortSignal;
}

export interface ExecutionOptions {
  readonly browser?: BrowserAdapter;
  readonly allowJavaScript?: boolean;
  readonly fetch?: typeof globalThis.fetch;
  readonly session?: Session;
  readonly externalTransforms?: Readonly<Record<string, ExternalTransform>>;
  readonly requestTimeoutMs?: number;
  readonly maxResponseBytes?: number;
  readonly userAgent?: string;
  readonly urlPolicy?: (context: URLPolicyContext, url: URL) => void | Promise<void>;
  readonly signal?: AbortSignal;
}

export interface Warning {
  readonly code: string;
  readonly message: string;
  readonly path?: string;
  readonly row?: number;
}

export interface ExtractionResult {
  readonly value: Readonly<Record<string, JsonValue>>;
  readonly warnings: readonly Warning[];
  readonly partial: boolean;
}

class ProgramSnapshot implements Program {
  readonly ir: ExtractorIR;
  readonly metadata: ProgramMetadata;
  readonly #plan: RuntimePlan;

  constructor(ir: ExtractorIR) {
    this.ir = deepFreeze(structuredClone(ir));
    this.#plan = prepareRuntime(this.ir, this.ir.source.fetch.mode);
    this.metadata = deepFreeze({
      name: ir.name,
      version: ir.version,
      languageVersion: ir.languageVersion,
      irVersion: ir.irVersion,
      files: structuredClone(ir.files),
      capabilities: [...ir.capabilities],
    });
  }

  async extract(
    inputs: Readonly<Record<string, JsonValue>> = {},
    options: ExecutionOptions = {},
  ): Promise<ExtractionResult> {
    return executeProgram(this.ir, inputs, options, this.#plan);
  }
}

function deepFreeze<T>(value: T): T {
  if (value === null || typeof value !== "object" || Object.isFrozen(value)) return value;
  for (const child of Object.values(value as Record<string, unknown>)) deepFreeze(child);
  return Object.freeze(value);
}
