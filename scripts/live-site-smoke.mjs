import assert from "node:assert/strict";
import { compile } from "../packages/scrape-kdl/dist/index.js";

const raw = process.argv[2];
assert.ok(raw, "usage: node scripts/live-site-smoke.mjs URL");
const target = new URL(raw);
assert.ok(["http:", "https:"].includes(target.protocol), "live-site URL must use HTTP(S)");
assert.equal(target.username, "", "credentials in live-site URLs are forbidden");
assert.equal(target.password, "", "credentials in live-site URLs are forbidden");

const source = `extractor "live-site-smoke" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url=${JSON.stringify(target.href)} }
  field "title" type="string" required=#true { select "title" match="one"; value "text" }
}`;
const compiled = await compile({ path: "live-site-smoke.kdl", data: source });
assert.ok(compiled.program, JSON.stringify(compiled.diagnostics));
await compiled.program.extract({}, {
  requestTimeoutMs: 15_000,
  maxResponseBytes: 2 << 20,
  urlPolicy(_context, url) { assert.equal(url.origin, target.origin, "cross-origin redirect rejected"); },
  userAgent: "scrape-kdl-live-site-smoke/1",
});
console.log(`live-site smoke passed for ${target.origin}`);
