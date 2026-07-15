import { RE2JS } from "re2js";

export interface PortableRegex {
  readonly source: string;
  readonly flags: string;
  readonly captureCount: number;
  exec(input: string): RegExpExecArray | null;
  test(input: string): boolean;
  replace(input: string, replacement: string, count: number): string;
}

export function compilePortableRegex(pattern: string, flags = ""): PortableRegex {
  const compiled = RE2JS.compile(pattern, regexFlags(flags));
  return {
    source: pattern,
    flags,
    captureCount: compiled.groupCount(),
    exec(input) {
      return compiled.exec(input) as RegExpExecArray | null;
    },
    test(input) {
      return compiled.test(input);
    },
    replace(input, replacement, count) {
      if (count === 0) return input;
      let output = "";
      let cursor = 0;
      let replacements = 0;
      for (const match of compiled.matchAll(input)) {
        if (count >= 0 && replacements >= count) break;
        const index = match.index;
        if (index === undefined) throw new Error("portable regex match did not report an index");
        output += input.slice(cursor, index);
        output += expandReplacement(replacement, match);
        cursor = index + match[0].length;
        replacements++;
      }
      return replacements === 0 ? input : output + input.slice(cursor);
    },
  };
}

function regexFlags(flags: string): number {
  let compiled = 0;
  const seen = new Set<string>();
  for (const flag of flags) {
    if (seen.has(flag)) throw new Error(`duplicate regex flag ${JSON.stringify(flag)}`);
    seen.add(flag);
    if (flag === "i") compiled |= RE2JS.CASE_INSENSITIVE;
    else if (flag === "m") compiled |= RE2JS.MULTILINE;
    else if (flag === "s") compiled |= RE2JS.DOTALL;
    else throw new Error(`unsupported regex flag ${JSON.stringify(flag)}`);
  }
  return compiled;
}

function expandReplacement(replacement: string, match: RegExpMatchArray): string {
  return replacement.replace(/\$(\$|\d{1,2})/gu, (_all, token: string) =>
    token === "$" ? "$" : String(match[Number(token)] ?? ""),
  );
}
