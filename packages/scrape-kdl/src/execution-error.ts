export class ExecutionError extends Error {
  readonly code: string;
  readonly path?: string;
  override readonly cause?: unknown;

  constructor(code: string, message: string, options: { readonly path?: string; readonly cause?: unknown } = {}) {
    super(message, options.cause === undefined ? undefined : { cause: options.cause });
    this.name = "ExecutionError";
    this.code = code;
    if (options.path !== undefined) this.path = options.path;
    if (options.cause !== undefined) this.cause = options.cause;
  }
}
