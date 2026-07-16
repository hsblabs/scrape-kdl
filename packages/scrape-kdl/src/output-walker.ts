import { ExecutionError } from "./execution-error.js";
import type { CollectionIR, FieldIR, JsonValue, OutputObjectIR, ResolvedTransformCallIR } from "./ir.js";
import {
  executionMessage,
  handleMissingOutput,
  MissingValue,
  type OutputState,
  recoverOutputField,
  validateCollectionMaximum,
  validateCollectionMinimum,
} from "./output-semantics.js";

export interface OutputOperations<Scope> {
  readonly state: OutputState;
  check(path: string): void;
  readField(scope: Scope, field: FieldIR, path: string): JsonValue | Promise<JsonValue>;
  queryRows(scope: Scope, collection: CollectionIR, path: string): readonly Scope[] | Promise<readonly Scope[]>;
  applyTransforms(value: JsonValue, calls: readonly ResolvedTransformCallIR[], path: string): Promise<JsonValue>;
  validateField(value: JsonValue, field: FieldIR, path: string): void;
}

export async function executeOutputObject<Scope>(
  operations: OutputOperations<Scope>,
  scope: Scope,
  object: OutputObjectIR,
  path: string,
): Promise<Record<string, JsonValue>> {
  const output: Record<string, JsonValue> = {};
  for (const member of object.members) {
    operations.check(path);
    output[member.name] =
      member.kind === "field"
        ? await executeField(operations, scope, member, `${path}.${member.name}`)
        : await executeCollection(operations, scope, member, `${path}.${member.name}`);
  }
  return output;
}

async function executeField<Scope>(
  operations: OutputOperations<Scope>,
  scope: Scope,
  field: FieldIR,
  path: string,
): Promise<JsonValue> {
  let value: JsonValue;
  try {
    value = await operations.readField(scope, field, path);
  } catch (error) {
    if (error instanceof MissingValue) return handleMissingOutput(field, path, error.message);
    return recoverOutputField(operations.state, field, path, error);
  }
  try {
    value = await operations.applyTransforms(value, field.transforms, path);
    operations.validateField(value, field, path);
    return value;
  } catch (error) {
    return recoverOutputField(operations.state, field, path, error);
  }
}

async function executeCollection<Scope>(
  operations: OutputOperations<Scope>,
  scope: Scope,
  collection: CollectionIR,
  path: string,
): Promise<readonly JsonValue[]> {
  const rows = await operations.queryRows(scope, collection, path);
  const output: JsonValue[] = [];
  for (let index = 0; index < rows.length; index++) {
    const rowPath = `${path}[${index}]`;
    operations.check(rowPath);
    try {
      output.push(await executeOutputObject(operations, rows[index]!, collection.row, rowPath));
    } catch (error) {
      if (
        (error instanceof ExecutionError && error.code === "E_EXECUTION_CANCELED") ||
        collection.onRowError !== "skip"
      )
        throw error;
      operations.state.partial = true;
      operations.state.warnings.push({ code: "W_ROW_SKIPPED", message: executionMessage(error), path, row: index });
    }
    validateCollectionMaximum(collection.maxItems, output.length, path);
  }
  validateCollectionMinimum(collection.required, collection.minItems, output.length, path);
  return output;
}
