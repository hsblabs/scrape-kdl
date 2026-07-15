import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { parse } from "../dist/parser.js";

const root = fileURLToPath(new URL("../../..", import.meta.url));

test("raw multiline strings and nested comments match the documented KDL subset", () => {
  const source = `/* outer /* inner */ done */\nnode #"""\n  first\n  second\n  """# key=#"raw"#\n`;
  const { document, diagnostics } = parse("raw.kdl", source);
  assert.deepEqual(diagnostics, []);
  assert.equal(document.nodes.length, 1);
  assert.equal(document.nodes[0].arguments[0].value, "  first\n  second\n  ");
  assert.equal(document.nodes[0].properties[0].value.value, "raw");
});

test("slashdash suppresses nodes, arguments, properties, children, and typed components", () => {
  const source = `/- (ignored) dropped "node"\nkept /- (ignored) "drop" "first" /- old=1 new=2 /- { ignored } { real "ok" }\n`;
  const { document, diagnostics } = parse("slashdash.kdl", source);
  assert.deepEqual(diagnostics, []);
  assert.deepEqual(document.nodes.map((node) => node.name), ["kept"]);
  assert.deepEqual(document.nodes[0].arguments.map((value) => value.value), ["first"]);
  assert.deepEqual(document.nodes[0].properties.map((property) => [property.name, property.value.value]), [["new", 2]]);
  assert.deepEqual(document.nodes[0].children.map((node) => node.name), ["real"]);
});

test("integer bases, separators, floats, booleans, and null retain kinds and raw spellings", () => {
  const { document, diagnostics } = parse("numbers.kdl", "numbers 0x10 0o10 0b10 -0x2 1_000 1.25e+2 #true #false #null\n");
  assert.deepEqual(diagnostics, []);
  assert.deepEqual(document.nodes[0].arguments.map(({ kind, raw, value }) => ({ kind, raw, value })), [
    { kind: "int", raw: "0x10", value: 16 },
    { kind: "int", raw: "0o10", value: 8 },
    { kind: "int", raw: "0b10", value: 2 },
    { kind: "int", raw: "-0x2", value: -2 },
    { kind: "int", raw: "1_000", value: 1000 },
    { kind: "float", raw: "1.25e+2", value: 125 },
    { kind: "bool", raw: "#true", value: true },
    { kind: "bool", raw: "#false", value: false },
    { kind: "null", raw: "#null", value: null },
  ]);
});

test("signed 64-bit integers retain an exact internal value beyond Number safe range", () => {
  const { document, diagnostics } = parse("int64.kdl", "numbers 9223372036854775807 -9223372036854775808\n");
  assert.deepEqual(diagnostics, []);
  assert.deepEqual(document.nodes[0].arguments.map((value) => value.integerValue), [9223372036854775807n, -9223372036854775808n]);
});

test("quoted Unicode escapes and UTF-8 byte spans are deterministic", () => {
  const { document, diagnostics } = parse("unicode.kdl", "node \"é\\u{1f40e}\"\n");
  assert.deepEqual(diagnostics, []);
  const value = document.nodes[0].arguments[0];
  assert.equal(value.value, "é🐎");
  assert.deepEqual(value.span, {
    file: "unicode.kdl",
    start: { offset: 5, line: 1, column: 6 },
    end: { offset: 18, line: 1, column: 18 },
  });
});

test("unsupported representations and malformed tokens use stable parser diagnostics", () => {
  const annotated = parse("typed.kdl", "(type) node\n");
  assert.deepEqual(annotated.diagnostics, [{
    code: "E_TYPE_ANNOTATION_UNSUPPORTED",
    severity: "error",
    message: "KDL type annotations are not supported",
    span: { file: "typed.kdl", start: { offset: 0, line: 1, column: 1 }, end: { offset: 1, line: 1, column: 2 } },
  }]);

  const malformed = parse("bad.kdl", "node 0x @\n");
  assert.deepEqual(malformed.diagnostics.map((item) => [item.code, item.message, item.span.start.offset, item.span.end.offset]), [
    ["E_KDL_SYNTAX", "unexpected token \"0x\" in node", 5, 7],
    ["E_KDL_SYNTAX", "integer base prefix must be followed by a digit", 5, 7],
    ["E_KDL_SYNTAX", "unexpected token \"@\" in node", 8, 9],
    ["E_KDL_SYNTAX", "unexpected character '@'", 8, 9],
  ]);
});

test("the shared Go and TypeScript fuzz seeds and bounded mutations never throw or hang", async () => {
  const seeds = JSON.parse(await readFile(`${root}/fixtures/parser/fuzz-seeds.json`, "utf8"));
  for (const seed of seeds) assert.doesNotThrow(() => parse("fuzz.kdl", seed));
  let state = 0x5eed1234;
  const alphabet = "{}();=/#-_ abcdef0123456789\\\"\n\ré🐎";
  for (let sample = 0; sample < 2048; sample++) {
    state = xorshift(state);
    const length = state % 512;
    let source = "";
    for (let index = 0; index < length; index++) {
      state = xorshift(state);
      source += alphabet[state % alphabet.length];
    }
    assert.doesNotThrow(() => parse("fuzz.kdl", source));
  }
});

test("shared parser cases match Go acceptance and exact syntax diagnostics", async () => {
  const cases = JSON.parse(await readFile(`${root}/fixtures/parser/cases.json`, "utf8"));
  for (const testCase of cases) {
    const result = parse(`${testCase.name}.kdl`, testCase.source);
    assert.deepEqual(result.diagnostics, testCase.diagnostics, testCase.name);
  }
});

function xorshift(value) {
  value ^= value << 13;
  value ^= value >>> 17;
  value ^= value << 5;
  return value >>> 0;
}
