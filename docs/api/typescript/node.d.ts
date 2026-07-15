import type { CompileOptions, CompileResult, DiagnosticIR } from "./index.js";

export * from "./index.js";

export declare function compileFile(path: string, options?: Omit<CompileOptions, "loader">): Promise<CompileResult>;
export declare function validateFile(path: string, options?: Omit<CompileOptions, "loader">): Promise<readonly DiagnosticIR[]>;
