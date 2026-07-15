import { isDeepStrictEqual } from "node:util";

export function validateJSONSchema(schema, value) {
  const errors = [];
  validate(schema, value, "$", schema, errors);
  return errors;
}

function validate(schema, value, path, root, errors) {
  if (schema.$ref !== undefined) {
    validate(resolveReference(root, schema.$ref), value, path, root, errors);
    return;
  }
  if (schema.oneOf !== undefined) {
    const matches = schema.oneOf.filter((candidate) => {
      const candidateErrors = [];
      validate(candidate, value, path, root, candidateErrors);
      return candidateErrors.length === 0;
    }).length;
    if (matches !== 1) errors.push(`${path}: expected exactly one schema variant, matched ${matches}`);
    return;
  }
  if (schema.type !== undefined && !matchesType(schema.type, value)) {
    errors.push(`${path}: expected type ${JSON.stringify(schema.type)}`);
    return;
  }
  if (schema.const !== undefined && !isDeepStrictEqual(value, schema.const)) errors.push(`${path}: expected constant ${JSON.stringify(schema.const)}`);
  if (schema.enum !== undefined && !schema.enum.some((candidate) => isDeepStrictEqual(value, candidate))) errors.push(`${path}: value is outside the enum`);

  if (typeof value === "number") {
    if (!Number.isFinite(value)) errors.push(`${path}: number must be finite`);
    if (schema.minimum !== undefined && value < schema.minimum) errors.push(`${path}: number is below minimum ${schema.minimum}`);
    if (schema.maximum !== undefined && value > schema.maximum) errors.push(`${path}: number exceeds maximum ${schema.maximum}`);
  }
  if (typeof value === "string") {
    const length = Array.from(value).length;
    if (schema.minLength !== undefined && length < schema.minLength) errors.push(`${path}: string is shorter than ${schema.minLength}`);
    if (schema.maxLength !== undefined && length > schema.maxLength) errors.push(`${path}: string is longer than ${schema.maxLength}`);
    if (schema.pattern !== undefined && !new RegExp(schema.pattern, "u").test(value)) errors.push(`${path}: string does not match ${schema.pattern}`);
    if (schema.format === "date" && !isCalendarDate(value)) errors.push(`${path}: string is not a calendar date`);
  }
  if (Array.isArray(value)) {
    if (schema.minItems !== undefined && value.length < schema.minItems) errors.push(`${path}: array has fewer than ${schema.minItems} items`);
    if (schema.maxItems !== undefined && value.length > schema.maxItems) errors.push(`${path}: array has more than ${schema.maxItems} items`);
    if (schema.uniqueItems === true) {
      for (let index = 0; index < value.length; index++) {
        if (value.slice(0, index).some((candidate) => isDeepStrictEqual(candidate, value[index]))) errors.push(`${path}[${index}]: duplicate array item`);
      }
    }
    if (schema.items !== undefined) value.forEach((item, index) => validate(schema.items, item, `${path}[${index}]`, root, errors));
  }
  if (value !== null && typeof value === "object" && !Array.isArray(value)) {
    const properties = schema.properties ?? {};
    for (const name of schema.required ?? []) if (!(name in value)) errors.push(`${path}.${name}: required property is missing`);
    for (const [name, item] of Object.entries(value)) {
      if (properties[name] !== undefined) validate(properties[name], item, `${path}.${name}`, root, errors);
      else if (schema.additionalProperties === false) errors.push(`${path}.${name}: additional property is not allowed`);
    }
    for (const [name, dependencies] of Object.entries(schema.dependentRequired ?? {})) {
      if (name in value) for (const dependency of dependencies) if (!(dependency in value)) errors.push(`${path}.${dependency}: property is required by ${name}`);
    }
  }
}

function matchesType(type, value) {
  if (Array.isArray(type)) return type.some((candidate) => matchesType(candidate, value));
  if (type === "null") return value === null;
  if (type === "array") return Array.isArray(value);
  if (type === "object") return value !== null && typeof value === "object" && !Array.isArray(value);
  if (type === "integer") return typeof value === "number" && Number.isInteger(value);
  if (type === "number") return typeof value === "number";
  return typeof value === type;
}

function resolveReference(root, reference) {
  if (!reference.startsWith("#/")) throw new Error(`only local JSON Schema references are supported: ${reference}`);
  let current = root;
  for (const token of reference.slice(2).split("/")) current = current[token.replaceAll("~1", "/").replaceAll("~0", "~")];
  if (current === undefined) throw new Error(`unresolved JSON Schema reference: ${reference}`);
  return current;
}

function isCalendarDate(value) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/u.exec(value);
  if (match === null) return false;
  const year = Number(match[1]); const month = Number(match[2]); const day = Number(match[3]);
  const date = new Date(0); date.setUTCHours(0, 0, 0, 0); date.setUTCFullYear(year, month - 1, day);
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}
