import { ExecutionError } from "./execution-error.js";
import type { FieldIR, JsonValue } from "./ir.js";
import type { Warning } from "./public-api.js";

export class MissingValue extends Error {}

export interface OutputState {
  readonly warnings: Warning[];
  partial: boolean;
}

export function handleMissingOutput(field: FieldIR, path: string, message: string): JsonValue {
  if (field.required) throw new ExecutionError("E_REQUIRED_VALUE_MISSING", message, { path });
  return field.default ?? null;
}

export function recoverOutputField(state: OutputState, field: FieldIR, path: string, cause: unknown): JsonValue {
  if (field.onError === "fail") {
    if (cause instanceof ExecutionError) throw cause;
    throw new ExecutionError("E_FIELD_EXECUTION", errorMessage(cause), { path, cause });
  }
  state.partial = true;
  if (field.onError === "warn")
    state.warnings.push({ code: "W_ERROR_RECOVERED", message: executionMessage(cause), path });
  if (field.onError === "default") {
    if (field.default === undefined)
      throw new ExecutionError("E_IR_INVALID", "on-error default requires a field default", { path });
    return field.default;
  }
  return null;
}

export function validateCollectionMinimum(required: boolean, minItems: number, count: number, path: string): void {
  const minimum = required ? Math.max(1, minItems) : minItems;
  if (count < minimum)
    throw new ExecutionError(
      "E_COLLECTION_CARDINALITY",
      `collection has ${count} rows after recovery; minimum is ${minimum}`,
      { path },
    );
}

export function validateCollectionMaximum(maxItems: number | undefined, count: number, path: string): void {
  if (maxItems !== undefined && count > maxItems)
    throw new ExecutionError("E_COLLECTION_CARDINALITY", `collection has ${count} rows; maximum is ${maxItems}`, {
      path,
    });
}

export function executionMessage(error: unknown): string {
  return error instanceof ExecutionError && error.path !== undefined
    ? `${error.code} at ${error.path}: ${error.message}`
    : errorMessage(error);
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error ?? "operation canceled");
}
