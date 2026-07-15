export type BuiltinExpectation = "string" | "bool" | "int" | "non-negative-int" | "number" | "scalar";

export interface BuiltinDefinition {
  readonly properties: Readonly<Record<string, BuiltinExpectation>>;
  readonly required: readonly string[];
  readonly minPositional: number;
  readonly maxPositional: number;
}

const definition = (
  properties: Readonly<Record<string, BuiltinExpectation>> = {},
  required: readonly string[] = [],
  minPositional = 0,
  maxPositional = 0,
): BuiltinDefinition => ({ properties, required, minPositional, maxPositional });

export const BUILTIN_CONTRACT: Readonly<Record<string, BuiltinDefinition>> = {
  trim: definition(),
  "normalize-whitespace": definition(),
  lowercase: definition(),
  uppercase: definition(),
  replace: definition({ old: "string", new: "string", count: "non-negative-int" }, ["old", "new"]),
  "regex-replace": definition(
    { pattern: "string", replacement: "string", flags: "string", count: "non-negative-int" },
    ["pattern", "replacement"],
  ),
  "regex-capture": definition({ pattern: "string", group: "non-negative-int", flags: "string" }, ["pattern"]),
  substring: definition({ start: "int", end: "int" }, ["start"]),
  split: definition({ separator: "string", limit: "non-negative-int" }, ["separator"]),
  join: definition({ separator: "string" }, ["separator"]),
  prepend: definition({ value: "string" }, ["value"]),
  append: definition({ value: "string" }, ["value"]),
  "parse-int": definition({ as: "string", radix: "int" }, ["as"]),
  "parse-float": definition({ as: "string" }, ["as"]),
  "parse-bool": definition({ "case-sensitive": "bool", true: "string", false: "string" }),
  "to-string": definition(),
  "empty-to-null": definition(),
  coalesce: definition({ value: "scalar" }, ["value"]),
  "url-resolve": definition({ base: "string" }, ["base"]),
  "url-query": definition({ name: "string", index: "non-negative-int" }, ["name"]),
  "url-path": definition(),
  "path-segment": definition({ index: "int" }, ["index"]),
  "assert-matches": definition({ pattern: "string", flags: "string" }, ["pattern"]),
  "assert-enum": definition({}, [], 1, -1),
  "assert-min": definition({ value: "number" }, ["value"]),
  "assert-max": definition({ value: "number" }, ["value"]),
};

export const BUILTIN_NAMES = new Set(Object.keys(BUILTIN_CONTRACT));
