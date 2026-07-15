import { readFile } from "node:fs/promises";
import { compile, validate, type CompileOptions, type CompileResult, type DiagnosticIR } from "./public-api.js";

export * from "./public-api.js";

export async function compileFile(path: string, options: Omit<CompileOptions, "loader"> = {}): Promise<CompileResult> {
  options.signal?.throwIfAborted();
  const data = await readFile(path);
  return compile(
    { path, data },
    {
      ...options,
      loader: {
        async load(importPath) {
          return readFile(importPath);
        },
      },
    },
  );
}

export async function validateFile(
  path: string,
  options: Omit<CompileOptions, "loader"> = {},
): Promise<readonly DiagnosticIR[]> {
  options.signal?.throwIfAborted();
  const data = await readFile(path);
  return validate(
    { path, data },
    {
      ...options,
      loader: {
        async load(importPath) {
          return readFile(importPath);
        },
      },
    },
  );
}
