import assert from "node:assert/strict";
import test from "node:test";
import { attribute, innerHTML, parseHTML, queryAll, textContent } from "../dist/dom.js";
import { compile, ExecutionError } from "../dist/index.js";
import { executeHTML } from "../dist/runtime.js";

async function compileSpec(source) {
  const result = await compile({ path: "extractor.kdl", data: source });
  assert.ok(result.program, JSON.stringify(result.diagnostics, null, 2));
  return result.program;
}

const sourcePrefix = `extractor "runtime" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.test/{id}" }
  input "id" type="string" required=#true
`;

test("parse5 DOM boundary implements the portable selector profile", () => {
  const document = parseHTML(`<!doctype html><main>
    <ul id="items" class="list"><li class="entry first" data-k="a"><a href="/a">A</a></li><li class="entry" data-k="b"><a href="/b">B</a></li><li class="entry"><span>C</span></li></ul>
    <div id="types"><span id="s1"></span><em id="e1"></em><span id="s2">text</span><span id="s3"></span></div>
  </main>`);
  const cases = new Map([
    ["#items > li.entry", 3], ["ul.list li:first-child", 1], ["li:nth-child(2)", 1],
    ["li:nth-last-child(1)", 1], ["li[data-k^='a']", 1], ["li.first + li", 1],
    ["li.first ~ li", 2], ["li:not(.first)", 2], ["li > a[href], li > span", 3],
    ["#types > span:nth-of-type(2n+1)", 2], ["#types > span:nth-last-of-type(2)", 1],
  ]);
  for (const [selector, count] of cases) assert.equal(queryAll(document, selector).length, count, selector);
  const items = queryAll(document, "#items")[0];
  assert.equal(textContent(items), "ABC");
  assert.equal(attribute(items, "class"), "list");
  assert.match(innerHTML(items), /^<li class="entry first"/u);
  assert.throws(() => queryAll(document, "a:hover"), /unsupported/u);
  assert.throws(() => queryAll(document, "[href='x' i]"), /unsupported/u);
});

test("HTTP runtime matches the shared fixture and runs every built-in family", async () => {
  const program = await compileSpec(`${sourcePrefix}
  field "trim" type="string" required=#true { select "#spaces"; value "text"; apply "trim" }
  field "normalize" type="string" required=#true { select "#spaces"; value "text"; apply "normalize-whitespace"; apply "lowercase"; apply "uppercase" }
  field "replace" type="string" required=#true { select "#replace"; value "text"; apply "replace" old="a" new="x" count=2 }
  field "replace_empty" type="string" required=#true { select "#unicode"; value "text"; apply "replace" old="" new="x" count=2 }
  field "regex_replace" type="string" required=#true { select "#regex"; value "text"; apply "regex-replace" pattern="([0-9]+)" replacement="[$1]" }
  field "regex_unicode" type="string" required=#true { select "#emoji"; value "text"; apply "regex-replace" pattern="." replacement="[$0]" count=1 }
  field "capture" type="string?" required=#true { select "#url"; value "text"; apply "regex-capture" pattern="/horse/([^/?#]+)" group=1 }
  field "substring" type="string" required=#true { select "#substring"; value "text"; apply "substring" start=1 end=-1 }
  field "joined" type="string" required=#true { select "#list"; value "text"; apply "split" separator=","; apply "join" separator="|"; apply "prepend" value="["; apply "append" value="]" }
  field "integer" type="u8" required=#true { select "#integer"; value "text"; apply "parse-int" as="u8"; apply "assert-min" value=10; apply "assert-max" value=20 }
  field "float" type="f64" required=#true { select "#float"; value "text"; apply "parse-float" as="f64" }
  field "boolean" type="bool" required=#true { select "#boolean"; value "text"; apply "parse-bool" true="yes" false="no" }
  field "coalesced" type="string" required=#true { select "#empty"; value "text"; apply "empty-to-null"; apply "coalesce" value="fallback" }
  field "resolved" type="string" required=#true { select "#reference"; value "text"; apply "url-resolve" base="https://example.test/base/" }
  field "query" type="string?" required=#true { select "#url"; value "text"; apply "url-query" name="x" index=1 }
  field "path" type="string" required=#true { select "#url"; value "text"; apply "url-path" }
  field "segment" type="string?" required=#true { select "#url"; value "text"; apply "path-segment" index=-1 }
  field "enum" type="string" required=#true { select "#enum"; value "text"; apply "assert-enum" "Alpha" "Beta"; apply "assert-matches" pattern="^[A-Z]"; apply "to-string" }
}`);
  const html = `<span id="spaces">  A   B  </span><span id="replace">aaaa</span><span id="regex">a12b34</span>
    <span id="url">https://example.test/horse/abc?x=one&amp;x=two</span><span id="unicode">é</span><span id="emoji">😀</span><span id="substring">A😀B</span><span id="list">a,b,c</span>
    <span id="integer">15</span><span id="float">1.25</span><span id="boolean">YES</span><span id="empty"></span>
    <span id="reference">../horse</span><span id="enum">Alpha</span>`;
  assert.deepEqual(await executeHTML(program.ir, html), {
    value: {
      trim: "A   B", normalize: "A B", replace: "xxaa", replace_empty: "xéx", regex_replace: "a[12]b[34]", regex_unicode: "[😀]", capture: "abc",
      substring: "😀", joined: "[a|b|c]", integer: 15, float: 1.25, boolean: true, coalesced: "fallback",
      resolved: "https://example.test/horse", query: "two", path: "/horse/abc", segment: "abc", enum: "Alpha",
    }, warnings: [], partial: false,
  });
});

test("missing values, field recovery, and row recovery match stable result semantics", async () => {
  const program = await compileSpec(`${sourcePrefix}
  field "optional" type="string?" required=#false { select ".missing"; value "text" }
  field "warned" type="string?" required=#false {
    select "#bad"; value "text"; apply "assert-matches" pattern="^ok$"; on-error "warn"
  }
  collection "items" min-items=1 on-row-error="skip" {
    select "li"
    field "number" type="u8" required=#true { select ".number"; value "text"; apply "parse-int" as="u8" }
  }
}`);
  const result = await executeHTML(program.ir, `<span id="bad">bad</span><ul><li><b class="number">1</b></li><li><b class="number">bad</b></li><li><b class="number">3</b></li></ul>`);
  assert.deepEqual(result.value, { optional: null, warned: null, items: [{ number: 1 }, { number: 3 }] });
  assert.equal(result.partial, true);
  assert.deepEqual(result.warnings.map(({ code, path, row }) => ({ code, path, row })), [
    { code: "W_ERROR_RECOVERED", path: "output.warned", row: undefined },
    { code: "W_ROW_SKIPPED", path: "output.items", row: 1 },
    { code: "W_PARTIAL_EXTRACTION", path: undefined, row: undefined },
  ]);
  assert.equal(result.warnings[0].message, "E_TRANSFORM at output.warned: value does not match required pattern");
});

test("pipeline, match, and external transforms share the declared runtime", async () => {
  const program = await compileSpec(`${sourcePrefix}
  transform "normalize" input="string" output="string" { pipeline { apply "trim"; apply "lowercase" } }
  transform "category" input="string" output="string" { match { case "male" "M"; case "female" "F"; default "U" } }
  transform "decorate" input="string" output="string" { external symbol="decorate" }
  field "category" type="string" required=#true { select "#category"; value "text"; apply "normalize"; apply "category" }
  field "decorated" type="string" required=#true { select "#name"; value "text"; apply "decorate" }
}`);
  await assert.rejects(executeHTML(program.ir, `<b id="category"> MALE </b><b id="name">Ada</b>`),
    (error) => error instanceof ExecutionError && error.code === "E_EXTERNAL_TRANSFORM_MISSING");
  const result = await executeHTML(program.ir, `<b id="category"> MALE </b><b id="name">Ada</b>`, {
    externalTransforms: { async decorate(context, input) { assert.equal(context.signal, undefined); return `&lt;${input}>`; } },
  });
  assert.deepEqual(result.value, { category: "M", decorated: "&lt;Ada>" });
});

test("preflight, inputs, and initial URL policy complete before fetch", async () => {
  const program = await compileSpec(`${sourcePrefix}
  field "title" type="string" required=#true { select "h1"; value "text" }
}`);
  let calls = 0;
  const fetch = async () => { calls++; return new Response("<h1>x</h1>"); };
  await assert.rejects(program.extract({}, { fetch }), (error) => error instanceof ExecutionError && error.code === "E_INPUT_REQUIRED");
  await assert.rejects(program.extract({ id: "x" }, { fetch, urlPolicy: () => { throw new Error("blocked"); } }), (error) => error instanceof ExecutionError && error.code === "E_URL_POLICY");
  assert.equal(calls, 0);
});

test("redirect policy runs before each request without duplicating or leaking secrets", async () => {
  const program = await compileSpec(`extractor "runtime" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.test/{id}"; session policy="optional" }
  input "id" type="string" required=#true
  field "title" type="string" required=#true { select "h1"; value "text" }
}`);
  const requests = [];
  const policies = [];
  const fetch = async (input, init) => {
    requests.push({ url: String(input), authorization: init.headers.get("authorization"), cookie: init.headers.get("cookie") });
    if (requests.length === 1) return new Response(null, { status: 302, headers: { location: "/same-origin" } });
    if (requests.length === 2) return new Response(null, { status: 302, headers: { location: "https://redirect.test/final" } });
    return new Response("<h1>done</h1>", { status: 200 });
  };
  const result = await program.extract({ id: "x" }, {
    fetch,
    session: { headers: { authorization: "Bearer secret" }, cookies: [{ name: "sid", value: "secret" }] },
    urlPolicy(_context, url) { policies.push(url.href); },
  });
  assert.equal(result.value.title, "done");
  assert.deepEqual(policies, ["https://example.test/x", "https://example.test/same-origin", "https://redirect.test/final"]);
  assert.deepEqual(requests, [
    { url: "https://example.test/x", authorization: "Bearer secret", cookie: "sid=secret" },
    { url: "https://example.test/same-origin", authorization: "Bearer secret", cookie: "sid=secret" },
    { url: "https://redirect.test/final", authorization: null, cookie: null },
  ]);
  await assert.rejects(program.extract({ id: "x" }, {
    fetch: async () => new Response(null, { status: 302, headers: { location: "file:///secret" } }),
  }), (error) => error instanceof ExecutionError && error.code === "E_HTTP_FETCH");
});

test("built-in failures retain the stable transform error code", async () => {
  const program = await compileSpec(`${sourcePrefix}
  field "number" type="u8" required=#true { select "#number"; value "text"; apply "parse-int" as="u8" }
}`);
  await assert.rejects(executeHTML(program.ir, `<span id="number">not-an-integer</span>`),
    (error) => error instanceof ExecutionError && error.code === "E_TRANSFORM" && error.path === "output.number"
      && error.message === "parse u8: strconv.ParseUint: parsing \"not-an-integer\": invalid syntax");
});

test("response bounds, timeout, cancellation, and charset decoding have stable codes", async () => {
  const program = await compileSpec(`${sourcePrefix}
  field "title" type="string" required=#true { select "h1"; value "text" }
}`);
  await assert.rejects(program.extract({ id: "x" }, {
    maxResponseBytes: 5, fetch: async () => new Response("<h1>too large</h1>"),
  }), (error) => error instanceof ExecutionError && error.code === "E_HTTP_BODY_TOO_LARGE");

  await assert.rejects(program.extract({ id: "x" }, {
    requestTimeoutMs: 5,
    fetch: async (_input, init) => new Promise((_resolve, reject) => init.signal.addEventListener("abort", () => reject(init.signal.reason), { once: true })),
  }), (error) => error instanceof ExecutionError && error.code === "E_TIMEOUT");

  const controller = new AbortController(); controller.abort(new Error("stop"));
  await assert.rejects(program.extract({ id: "x" }, { signal: controller.signal, fetch: async () => assert.fail("fetch after cancellation") }),
    (error) => error instanceof ExecutionError && error.code === "E_EXECUTION_CANCELED");

  const bytes = new Uint8Array([60, 104, 49, 62, 0x80, 60, 47, 104, 49, 62]);
  const decoded = await program.extract({ id: "x" }, { fetch: async () => new Response(bytes, { headers: { "content-type": "text/html; charset=windows-1252" } }) });
  assert.equal(decoded.value.title, "€");
});
