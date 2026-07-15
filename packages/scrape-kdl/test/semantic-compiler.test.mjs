import assert from "node:assert/strict";
import test from "node:test";
import { compileContractSlice } from "../dist/compiler.js";

async function compileText(data) {
  return compileContractSlice({ path: "extractor.kdl", data });
}

test("semantic compiler lowers browser workflows, transforms, defaults, and capabilities", async () => {
  const result = await compileText(`extractor "workflow" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="browser" url="https://example.invalid/{lang}"
    workflow {
      wait-for "#ready" state="attached" timeout-ms=100
      click "button" timeout-ms=200
      fill "input" "value" timeout-ms=300
      press "input" "Enter" timeout-ms=400
      scroll 1.5 -2
      wait-for-network-idle idle-ms=250 timeout-ms=500
      evaluate-js "() => null" timeout-ms=600
    }
  }
  input "lang" type="string" required=#false default="ja"
  transform "clean" input="string" output="string" {
    pipeline { apply "trim"; apply "lowercase" }
  }
  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "clean"
  }
}`);
  assert.equal(result.diagnostics.some((item) => item.severity === "error"), false, JSON.stringify(result.diagnostics));
  assert.ok(result.ir);
  assert.equal(result.ir.source.workflow.length, 7);
  assert.deepEqual(result.ir.source.workflow[4], {
    kind: "scroll", x: 1.5, y: -2,
    span: result.ir.source.workflow[4].span,
  });
  assert.deepEqual(result.ir.capabilities, [
    "browser.evaluate-js", "browser.input", "browser.navigate", "browser.network-idle",
    "browser.query", "browser.read-text", "browser.scroll", "browser.wait",
  ]);
  assert.equal(result.ir.transforms[0].kind, "pipeline");
  assert.equal(result.ir.inputs[0].default, "ja");
});

test("semantic compiler rejects invalid workflow, selector, template, and defaults without IR", async () => {
  const result = await compileText(`extractor "invalid" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="browser" url="/relative/{optional}"
    workflow {
      wait-for "a:hover" state="moving" timeout-ms=0
      scroll "right" 0
      wait-for-network-idle idle-ms=0 timeout-ms=0
      unsupported-step
    }
  }
  input "optional" type="string" required=#false
  field "title" type="string" required=#false default=1 {
    select "h1" match="one"
    value "text"
    on-error "default"
  }
}`);
  assert.equal(result.ir, undefined);
  const codes = new Set(result.diagnostics.map((item) => item.code));
  for (const code of [
    "E_DEFAULT_INVALID", "E_SELECTOR_UNSUPPORTED", "E_TEMPLATE_INVALID", "E_TEMPLATE_OPTIONAL_INPUT",
    "E_TYPE_MISMATCH", "E_UNKNOWN_NODE",
  ]) assert.ok(codes.has(code), `${code} missing from ${JSON.stringify(result.diagnostics)}`);
});

test("semantic compiler tokenizes escaped URL-template braces", async () => {
  const result = await compileText(`extractor "escaped-template" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/{{literal}}/{id}" }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`);
  assert.ok(result.ir, JSON.stringify(result.diagnostics));
  assert.deepEqual(result.ir.source.fetch.urlTemplate.segments, [
    { kind: "literal", value: "https://example.invalid/{literal}/" },
    { kind: "input", name: "id" },
  ]);
});

test("semantic compiler keeps generated IR JSON-compatible and schema-bounded", async () => {
  const result = await compileText(`extractor "schema" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`);
  assert.ok(result.ir, JSON.stringify(result.diagnostics));
  assert.doesNotThrow(() => JSON.stringify(result.ir));
  const visit = (value) => {
    if (typeof value === "number") assert.ok(Number.isFinite(value) && Number.isSafeInteger(value) || Number.isFinite(value));
    else if (Array.isArray(value)) value.forEach(visit);
    else if (value !== null && typeof value === "object") Object.values(value).forEach(visit);
  };
  visit(result.ir);
  assert.equal(result.ir.irVersion, "2026-07-15");
  assert.match(result.ir.files[0].sha256, /^[a-f0-9]{64}$/u);
});
