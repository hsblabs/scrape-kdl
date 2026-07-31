import type { DiagnosticIR, ExtractorIR, JsonValue } from "../../ir/typescript/index.js";

export type { DiagnosticIR, ExtractorIR, JsonValue };

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

export declare class SourceLoadError extends Error {
  readonly path: string;
  readonly fromPath: string;
  constructor(path: string, fromPath: string, cause: unknown);
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

export type FetchMode = "http" | "browser";
export type SessionPolicy = "none" | "optional" | "required";

export interface SourceDescriptor {
  readonly fetchMode: FetchMode;
  readonly urlTemplate: string;
  readonly sessionPolicy: SessionPolicy;
}

export interface ProgramDescriptor {
  readonly source: SourceDescriptor;
}

export interface Program {
  readonly metadata: ProgramMetadata;
  readonly descriptor: ProgramDescriptor;
  readonly ir: ExtractorIR;
  extract(inputs?: Readonly<Record<string, JsonValue>>, options?: ExecutionOptions): Promise<ExtractionResult>;
  extractSnapshot(html: string, options?: ExecutionOptions): Promise<ExtractionResult>;
}

export declare function compile(source: Source, options?: CompileOptions): Promise<CompileResult>;
export declare function validate(source: Source, options?: CompileOptions): Promise<readonly DiagnosticIR[]>;
export declare function supportedLanguageVersions(): readonly string[];
export declare function supportedIRVersions(): readonly string[];

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
  waitFor(selector: string, state: "attached" | "visible" | "hidden" | "detached", options: BrowserOperationOptions): Promise<void>;
  click(selector: string, options: BrowserOperationOptions): Promise<void>;
  fill(selector: string, value: string, options: BrowserOperationOptions): Promise<void>;
  press(selector: string, key: string, options: BrowserOperationOptions): Promise<void>;
  scroll(x: number, y: number, options: Pick<BrowserOperationOptions, "signal">): Promise<void>;
  waitForNetworkIdle(idleMs: number, options: BrowserOperationOptions): Promise<void>;
  evaluate(source: string, options: BrowserEvaluateOptions): Promise<JsonValue>;
  queryAll(scope: BrowserElement | undefined, selector: string, options: Pick<BrowserOperationOptions, "signal">): Promise<readonly BrowserElement[]>;
  text(element: BrowserElement, options: Pick<BrowserOperationOptions, "signal">): Promise<string>;
  html(element: BrowserElement, options: Pick<BrowserOperationOptions, "signal">): Promise<string>;
  attribute(element: BrowserElement, name: string, options: Pick<BrowserOperationOptions, "signal">): Promise<string | undefined>;
}

export interface BrowserAdapterLease {
  acquire(signal?: AbortSignal): Promise<() => void>;
}

export interface BrowserAdapterQueryLimit {
  queryLimit(scope: BrowserElement | undefined, selector: string, limit: number, options: Pick<BrowserOperationOptions, "signal">): Promise<readonly BrowserElement[]>;
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

export declare class ExecutionError extends Error {
  readonly code: string;
  readonly path?: string;
  readonly cause?: unknown;
}
