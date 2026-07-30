import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { chromium, firefox, webkit } from "playwright";
import { compile, ExecutionError } from "@hsblabs/scrape-kdl";
import { PlaywrightAdapter } from "../dist/index.js";

const root = new URL("../../../", import.meta.url);
const browserName = process.env.PLAYWRIGHT_BROWSER ?? "chromium";
const browserType = { chromium, firefox, webkit }[browserName];
assert.ok(browserType, `unsupported PLAYWRIGHT_BROWSER ${JSON.stringify(browserName)}`);
const chromiumExecutablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
const launchOptions = {
  headless: true,
  ...(browserName === "chromium" && chromiumExecutablePath ? { executablePath: chromiumExecutablePath } : {}),
};
const [fixtureSource, fixtureHTML, expected] = await Promise.all([
  readFile(new URL("fixtures/valid/rod-browser-e2e.kdl", root), "utf8"),
  readFile(new URL("fixtures/html/rod-browser-e2e.html", root), "utf8"),
  readFile(new URL("fixtures/expected-output/rod-browser-e2e.json", root), "utf8").then(JSON.parse),
]);

test(`Chromium matches every portable HTML and pinned WPT observation`, { timeout: 60_000, skip: browserName !== "chromium" }, async () => {
  const manifest = JSON.parse(await readFile(new URL("fixtures/html-compat/manifest.json", root), "utf8"));
  assert.deepEqual(manifest.approvedDivergences, []);
  assert.ok(manifest.upstream.selectedTests.length >= 3);
  const browser = await browserType.launch(launchOptions);
  const page = await browser.newPage();
  try {
    for (const fixture of manifest.cases) {
      assert.equal(fixture.parserMode, "document");
      await page.setContent(await readFile(new URL(fixture.input, root), "utf8"), { waitUntil: "domcontentloaded" });
      for (const observation of fixture.observations) {
        const actual = await page.locator(observation.selector).evaluateAll((nodes, expectedObservation) => ({
          text: expectedObservation.text === undefined ? undefined : nodes.map((node) => node.textContent),
          innerHTML: expectedObservation.innerHTML === undefined ? undefined : nodes.map((node) => node.innerHTML),
          attributes: expectedObservation.attributes === undefined ? undefined : nodes.map((node, index) =>
            Object.fromEntries(Object.keys(expectedObservation.attributes[index]).map((name) => [name, node.getAttribute(name)]))),
        }), observation);
        if (observation.text !== undefined) assert.deepEqual(actual.text, observation.text, `${fixture.id}/${observation.selector}/text`);
        if (observation.innerHTML !== undefined) assert.deepEqual(actual.innerHTML, observation.innerHTML, `${fixture.id}/${observation.selector}/innerHTML`);
        if (observation.attributes !== undefined) assert.deepEqual(actual.attributes, observation.attributes, `${fixture.id}/${observation.selector}/attributes`);
      }
    }
  } finally {
    await page.close();
    await browser.close();
  }
});

async function fixtureServer() {
  const observations = [];
  const server = createServer((request, response) => {
    observations.push({ url: request.url, authorization: request.headers.authorization, cookie: request.headers.cookie, userAgent: request.headers["user-agent"] });
    if (request.url === "/failure") { request.socket.destroy(); return; }
    const send = () => {
      response.writeHead(200, { "content-type": "text/html; charset=utf-8" });
      const body = request.url === "/first" ? fixtureHTML.replace(">pending<", ">/first<") : request.url === "/second" ? fixtureHTML.replace(">pending<", ">/second<") : fixtureHTML;
      response.end(body);
    };
    if (request.url === "/slow") setTimeout(send, 250);
    else send();
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  assert.ok(address && typeof address === "object");
  return {
    url: `http://127.0.0.1:${address.port}`,
    observations,
    close: () => new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve())),
  };
}

test(`packed Playwright boundary executes the shared browser fixture in ${browserName}`, { timeout: 60_000 }, async () => {
  const fixture = await fixtureServer();
  const browser = await browserType.launch(launchOptions);
  const adapter = new PlaywrightAdapter(browser);
  try {
    const source = fixtureSource.replace("http://127.0.0.1:18080", fixture.url);
    const compiled = await compile({ path: "fixtures/valid/rod-browser-e2e.kdl", data: source });
    assert.ok(compiled.program, JSON.stringify(compiled.diagnostics, null, 2));
    const result = await compiled.program.extract({}, {
      browser: adapter,
      allowJavaScript: true,
      userAgent: "scrape-kdl-playwright-e2e",
      session: { headers: { authorization: "Bearer fixture" }, cookies: [{ name: "sid", value: "fixture" }] },
    });
    assert.deepEqual(result, expected);
    assert.equal(fixture.observations[0].authorization, "Bearer fixture");
    assert.equal(fixture.observations[0].cookie, "sid=fixture");
    assert.equal(fixture.observations[0].userAgent, "scrape-kdl-playwright-e2e");
  } finally {
    await adapter.close();
    await browser.close();
    await fixture.close();
  }
});

test("lease waiters honor cancellation and releases are idempotent", async () => {
  const adapter = new PlaywrightAdapter({});
  const first = await adapter.acquire();
  const controller = new AbortController();
  const waiting = adapter.acquire(controller.signal);
  controller.abort(new Error("stop waiting"));
  await assert.rejects(waiting, /stop waiting/u);
  first(); first();
  const next = await adapter.acquire();
  next();
  await adapter.close();
});

test("the official adapter lease serializes concurrent extractions", { timeout: 60_000 }, async () => {
  const fixture = await fixtureServer();
  const browser = await browserType.launch(launchOptions);
  const adapter = new PlaywrightAdapter(browser);
  try {
    const source = `extractor "concurrency" version="2026-07-15" language-version="2026-07-15" {
      source "html" { fetch mode="browser" url="${fixture.url}/{id}" }
      input "id" type="string" required=#true
      field "value" type="string" required=#true { select "#value"; value "text" }
    }`;
    const compiled = await compile({ path: "concurrency.kdl", data: source });
    assert.ok(compiled.program, JSON.stringify(compiled.diagnostics));
    const [first, second] = await Promise.all([
      compiled.program.extract({ id: "first" }, { browser: adapter }),
      compiled.program.extract({ id: "second" }, { browser: adapter }),
    ]);
    assert.equal(first.value.value, "/first");
    assert.equal(second.value.value, "/second");
  } finally {
    await adapter.close();
    await browser.close();
    await fixture.close();
  }
});

test("timeout, cancellation, and navigation failure clean up before recovery", { timeout: 60_000 }, async () => {
  const fixture = await fixtureServer();
  const browser = await browserType.launch(launchOptions);
  const adapter = new PlaywrightAdapter(browser);
  try {
    const timeoutSource = `extractor "timeout" version="2026-07-15" language-version="2026-07-15" {
      source "html" { fetch mode="browser" url="${fixture.url}" }
      field "never" type="string" required=#true {
        evaluate-js "async () => new Promise(() => {})" scope="document" returns="string" timeout-ms=20
      }
    }`;
    const timeoutProgram = await compile({ path: "timeout.kdl", data: timeoutSource });
    assert.ok(timeoutProgram.program, JSON.stringify(timeoutProgram.diagnostics));
    await assert.rejects(timeoutProgram.program.extract({}, { browser: adapter, allowJavaScript: true }),
      (error) => error instanceof ExecutionError && error.code === "E_TIMEOUT");
    assert.equal(browser.contexts().length, 0);

    const cancellationSource = `extractor "cancellation" version="2026-07-15" language-version="2026-07-15" {
      source "html" { fetch mode="browser" url="${fixture.url}/slow" }
      field "value" type="string" required=#true { select "#value"; value "text" }
    }`;
    const cancellationProgram = await compile({ path: "cancellation.kdl", data: cancellationSource });
    assert.ok(cancellationProgram.program, JSON.stringify(cancellationProgram.diagnostics));
    const controller = new AbortController();
    const canceled = cancellationProgram.program.extract({}, { browser: adapter, signal: controller.signal });
    setTimeout(() => controller.abort(new Error("stop")), 20);
    await assert.rejects(canceled, (error) => error instanceof ExecutionError && error.code === "E_EXECUTION_CANCELED");
    assert.equal(browser.contexts().length, 0);

    const failureSource = `extractor "failure" version="2026-07-15" language-version="2026-07-15" {
      source "html" { fetch mode="browser" url="${fixture.url}/failure" }
      field "value" type="string" required=#true { select "#value"; value "text" }
    }`;
    const failureProgram = await compile({ path: "failure.kdl", data: failureSource });
    assert.ok(failureProgram.program, JSON.stringify(failureProgram.diagnostics));
    await assert.rejects(failureProgram.program.extract({}, { browser: adapter }),
      (error) => error instanceof ExecutionError && error.code === "E_BROWSER_NAVIGATE");
    assert.equal(browser.contexts().length, 0);

    const recoverySource = `extractor "recovery" version="2026-07-15" language-version="2026-07-15" {
      source "html" { fetch mode="browser" url="${fixture.url}" }
      field "value" type="string" required=#true { select "#value"; value "text" }
    }`;
    const recoveryProgram = await compile({ path: "recovery.kdl", data: recoverySource });
    assert.ok(recoveryProgram.program, JSON.stringify(recoveryProgram.diagnostics));
    assert.equal((await recoveryProgram.program.extract({}, { browser: adapter })).value.value, "pending");
  } finally {
    await adapter.close();
    await browser.close();
    await fixture.close();
  }
});
