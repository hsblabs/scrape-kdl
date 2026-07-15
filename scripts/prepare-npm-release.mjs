import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cp, mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const version = process.argv[2];
const destination = process.argv[3];
assert.match(version ?? "", /^(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/u, "usage: prepare-npm-release.mjs VERSION DESTINATION");
assert.ok(destination, "destination is required");
await mkdir(destination, { recursive: true });
const temporary = await mkdtemp(join(tmpdir(), "scrape-kdl-npm-release-"));
try {
  const core = await stage("packages/scrape-kdl", "core");
  const adapter = await stage("packages/scrape-kdl-playwright", "playwright");
  const coreManifest = JSON.parse(await readFile(join(core, "package.json"), "utf8"));
  coreManifest.version = version;
  await writeFile(join(core, "package.json"), `${JSON.stringify(coreManifest, null, 2)}\n`);
  const adapterManifest = JSON.parse(await readFile(join(adapter, "package.json"), "utf8"));
  adapterManifest.version = version;
  adapterManifest.devDependencies["@hsblabs/scrape-kdl"] = version;
  adapterManifest.peerDependencies["@hsblabs/scrape-kdl"] = version;
  await writeFile(join(adapter, "package.json"), `${JSON.stringify(adapterManifest, null, 2)}\n`);

  const corePack = pack(core);
  const adapterPack = pack(adapter);
  assert.equal(corePack.name, "@hsblabs/scrape-kdl");
  assert.equal(adapterPack.name, "@hsblabs/scrape-kdl-playwright");
  assert.equal(corePack.version, version);
  assert.equal(adapterPack.version, version);
  const coreTarball = join(destination, corePack.filename);
  const adapterTarball = join(destination, adapterPack.filename);

  const consumer = join(temporary, "consumer");
  await mkdir(consumer);
  await writeFile(join(consumer, "package.json"), '{"name":"rc-clean-consumer","private":true,"type":"module"}\n');
  await writeFile(join(consumer, "index.mjs"), `
import assert from "node:assert/strict";
import { compile, supportedLanguageVersions } from "@hsblabs/scrape-kdl";
import { PlaywrightAdapter } from "@hsblabs/scrape-kdl-playwright";
const result = await compile({ path: "smoke.kdl", data: 'extractor "smoke" version="2026-07-15" language-version="2026-07-15" { source "html" { fetch mode="http" url="https://example.invalid/" } field "title" type="string" required=#true { select "h1"; value "text" } }' });
assert.ok(result.program);
assert.deepEqual(supportedLanguageVersions(), ["2026-07-15"]);
assert.equal(typeof PlaywrightAdapter, "function");
`);
  run("npm", ["install", "--ignore-scripts", "--no-audit", "--no-fund", coreTarball, adapterTarball], consumer);
  run(process.execPath, ["index.mjs"], consumer);
  console.log(`npm RC package check: ${corePack.filename} and ${adapterPack.filename} passed clean install`);
} finally {
  await rm(temporary, { recursive: true, force: true });
}

async function stage(source, name) {
  const target = join(temporary, name);
  await cp(source, target, { recursive: true, filter: (path) => !path.includes("node_modules") });
  return target;
}

function pack(directory) {
  const encoded = run("npm", ["pack", "--json", directory, "--pack-destination", destination]);
  const result = JSON.parse(encoded);
  assert.equal(result.length, 1);
  for (const file of result[0].files) assert.doesNotMatch(file.path, /(?:^|\/)(?:src|test|tsconfig\.json)(?:\/|$)|\.map$/u);
  return result[0];
}

function run(command, args, cwd = process.cwd()) {
  return execFileSync(command, args, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
}
