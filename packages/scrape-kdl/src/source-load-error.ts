export class SourceLoadError extends Error {
  readonly path: string;
  readonly fromPath: string;

  constructor(path: string, fromPath: string, cause: unknown) {
    const detail = cause instanceof Error ? cause.message : String(cause);
    super(`read ${JSON.stringify(path)} imported from ${JSON.stringify(fromPath)}: ${detail}`, { cause });
    this.name = "SourceLoadError";
    this.path = path;
    this.fromPath = fromPath;
  }
}
