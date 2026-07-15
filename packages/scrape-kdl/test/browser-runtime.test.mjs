import assert from "node:assert/strict";
import test from "node:test";
import { compile, ExecutionError } from "../dist/index.js";

async function compileBrowser(source) {
  const result = await compile({ path: "browser.kdl", data: source });
  assert.ok(result.program, JSON.stringify(result.diagnostics, null, 2));
  return result.program;
}

const browserSource = `extractor "browser" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="browser" url="https://example.test/{id}"
    session policy="optional"
    workflow {
      wait-for "#ready" state="attached" timeout-ms=100
      click "#load" timeout-ms=200
      fill "#input" "value" timeout-ms=300
      press "#input" "Enter" timeout-ms=400
      scroll 1.5 -2
      wait-for-network-idle idle-ms=25 timeout-ms=500
      evaluate-js "() => null" timeout-ms=600
    }
  }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1" match="one"; value "text"; apply "trim" }
  field "markup" type="string" required=#true { select "h1" match="one"; value "html" }
  field "count" type="int" required=#true { evaluate-js "() => document.querySelectorAll('li').length" scope="document" returns="int" }
  collection "items" min-items=1 on-row-error="fail" {
    select "li"
    field "id" type="string" required=#true { select ".id" match="one"; value "attr" name="data-id" }
    field "text" type="string" required=#true { select ".label" match="one"; value "text" }
    field "dataset" type="object" required=#true { evaluate-js "(element) => ({ id: element.dataset.id })" scope="current" returns="object" }
  }
}`;

function element(id, text = "", html = "", attributes = {}) { return { id, text, html, attributes }; }

class FakeBrowser {
  constructor() {
    this.calls = [];
    this.heading = element("heading", "  Browser Title  ", "Browser <b>Title</b>");
    this.rows = [element("row-1"), element("row-2")];
  }
  async navigate(url, options) { this.calls.push(["navigate", url, options]); }
  async waitFor(selector, state, options) { this.calls.push(["waitFor", selector, state, options]); }
  async click(selector, options) { this.calls.push(["click", selector, options]); }
  async fill(selector, value, options) { this.calls.push(["fill", selector, value, options]); }
  async press(selector, key, options) { this.calls.push(["press", selector, key, options]); }
  async scroll(x, y, options) { this.calls.push(["scroll", x, y, options]); }
  async waitForNetworkIdle(idleMs, options) { this.calls.push(["networkIdle", idleMs, options]); }
  async evaluate(source, options) {
    this.calls.push(["evaluate", source, options]);
    if (source.includes("querySelectorAll")) return 2;
    if (source.includes("dataset")) return { id: options.scope.id.replace("row-", "") };
    return null;
  }
  async queryAll(scope, selector) {
    if (scope === undefined && selector === "h1") return [this.heading];
    if (scope === undefined && selector === "li") return this.rows;
    if (scope?.id.startsWith("row-") && selector === ".id") return [element(`${scope.id}-id`, "", "", { "data-id": scope.id.replace("row-", "") })];
    if (scope?.id.startsWith("row-") && selector === ".label") return [element(`${scope.id}-label`, scope.id === "row-1" ? "A" : "B")];
    return [];
  }
  async queryLimit(scope, selector, limit, options) {
    this.calls.push(["queryLimit", selector, limit, options]);
    return (await this.queryAll(scope, selector)).slice(0, limit);
  }
  async text(value) { return value.text; }
  async html(value) { return value.html; }
  async attribute(value, name) { return value.attributes[name]; }
}

test("browser runtime executes workflows, live reads, transforms, and JavaScript", async () => {
  const program = await compileBrowser(browserSource);
  const browser = new FakeBrowser();
  const session = { headers: { authorization: "Bearer test" }, cookies: [{ name: "sid", value: "test" }] };
  const result = await program.extract({ id: "browser fixture" }, { browser, allowJavaScript: true, session, userAgent: "browser-test" });
  assert.deepEqual(result, {
    value: {
      title: "Browser Title", markup: "Browser <b>Title</b>", count: 2,
      items: [{ id: "1", text: "A", dataset: { id: "1" } }, { id: "2", text: "B", dataset: { id: "2" } }],
    }, warnings: [], partial: false,
  });
  assert.equal(browser.calls[0][0], "navigate");
  assert.equal(browser.calls[0][1], "https://example.test/browser%20fixture");
  assert.equal(browser.calls[0][2].session, session);
  assert.deepEqual(browser.calls.slice(1, 8).map((call) => call[0]), ["waitFor", "click", "fill", "press", "scroll", "networkIdle", "evaluate"]);
  assert.deepEqual(
    browser.calls.filter((call) => call[0] === "queryLimit").map((call) => [call[1], call[2]]),
    [["h1", 2], ["h1", 2], [".id", 2], [".label", 2], [".id", 2], [".label", 2]],
  );
});

test("JavaScript, policy, input, and runtime failures occur before browser navigation", async () => {
  const program = await compileBrowser(browserSource);
  const browser = new FakeBrowser();
  await assert.rejects(program.extract({ id: "x" }, { browser }),
    (error) => error instanceof ExecutionError && error.code === "E_JAVASCRIPT_DISABLED");
  await assert.rejects(program.extract({}, { browser, allowJavaScript: true }),
    (error) => error instanceof ExecutionError && error.code === "E_INPUT_REQUIRED");
  await assert.rejects(program.extract({ id: "x" }, { browser, allowJavaScript: true, urlPolicy: () => { throw new Error("blocked"); } }),
    (error) => error instanceof ExecutionError && error.code === "E_URL_POLICY");
  await assert.rejects(program.extract({ id: "x" }, { allowJavaScript: true }),
    (error) => error instanceof ExecutionError && error.code === "E_BROWSER_RUNTIME_MISSING");
  assert.equal(browser.calls.length, 0);
});

test("the extraction-wide lease serializes calls and releases after timeout failures", async () => {
  const program = await compileBrowser(`extractor "lease" version="2026-07-15" language-version="2026-07-15" {
    source "html" { fetch mode="browser" url="https://example.test/{id}" }
    input "id" type="string" required=#true
    field "title" type="string" required=#true { select "h1"; value "text" }
  }`);
  const browser = new FakeBrowser();
  let held = false; const waiters = []; let releases = 0; let active = 0; let maximum = 0;
  browser.acquire = async () => {
    if (held) await new Promise((resolve) => waiters.push(resolve));
    held = true;
    let released = false;
    return () => { if (!released) { released = true; releases++; held = false; waiters.shift()?.(); } };
  };
  browser.navigate = async (url) => {
    active++; maximum = Math.max(maximum, active);
    await new Promise((resolve) => setTimeout(resolve, url.endsWith("/first") ? 15 : 1));
    active--;
  };
  const [first, second] = await Promise.all([
    program.extract({ id: "first" }, { browser }), program.extract({ id: "second" }, { browser }),
  ]);
  assert.equal(first.value.title, "  Browser Title  "); assert.equal(second.value.title, "  Browser Title  ");
  assert.equal(maximum, 1); assert.equal(releases, 2);

  browser.navigate = async () => { throw new DOMException("timed out", "TimeoutError"); };
  await assert.rejects(program.extract({ id: "timeout" }, { browser }),
    (error) => error instanceof ExecutionError && error.code === "E_TIMEOUT");
  assert.equal(releases, 3);
});
