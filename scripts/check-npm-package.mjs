import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { constants } from "node:fs";
import { access, cp, copyFile, mkdtemp, mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, dirname, join, relative } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const developmentVersion = "0.0.0-development";
const defaultPublishAccess = "restricted";

export async function checkNpmPackages({ releaseVersion, outputDirectory, publishAccess = defaultPublishAccess } = {}) {
  if (releaseVersion !== undefined) validateReleaseVersion(releaseVersion);
  validatePublishAccess(publishAccess);
  const temporary = await mkdtemp(join(tmpdir(), "scrape-kdl-npm-"));
  try {
    const packDirectory = join(temporary, "pack");
    await mkdir(packDirectory);
    const { packed, adapterPacked } =
      releaseVersion === undefined
        ? packWorkspacePackages(packDirectory)
        : await packReleasePackages(temporary, packDirectory, releaseVersion, publishAccess);
    assert.equal(packed.length, 1, "npm pack must produce exactly one core artifact");
    const metadata = packed[0];
    const paths = metadata.files.map((file) => file.path).sort();
    for (const required of [
      "LICENSE",
      "NOTICE",
      "README.md",
      "dist/authoring.js",
      "dist/authoring.d.ts",
      "dist/index.js",
      "dist/index.d.ts",
      "dist/node.js",
      "dist/node.d.ts",
      "package.json",
    ]) {
      assert.ok(paths.includes(required), `packed core is missing ${required}`);
    }
    for (const path of paths) {
      assert.match(
        path,
        /^(?:LICENSE|NOTICE|README\.md|package\.json|dist\/(?:[a-z0-9-]+\.)+(?:js|d\.ts))$/u,
        `unexpected packed path ${path}`,
      );
      assert.doesNotMatch(path, /(?:fixture|test|tsconfig|\.map$|node_modules)/u);
    }

    const tarball = join(packDirectory, metadata.filename);
    assert.equal(adapterPacked.length, 1, "npm pack must produce exactly one adapter artifact");
    const adapterMetadata = adapterPacked[0];
    const adapterPaths = adapterMetadata.files.map((file) => file.path).sort();
    for (const required of ["LICENSE", "NOTICE", "README.md", "dist/index.js", "dist/index.d.ts", "package.json"]) {
      assert.ok(adapterPaths.includes(required), `packed adapter is missing ${required}`);
    }
    for (const path of adapterPaths) {
      assert.match(
        path,
        /^(?:LICENSE|NOTICE|README\.md|package\.json|dist\/(?:[a-z0-9-]+\.)+(?:js|d\.ts))$/u,
        `unexpected packed adapter path ${path}`,
      );
      assert.doesNotMatch(path, /(?:fixture|test|tsconfig|\.map$|node_modules)/u);
    }
    const adapterTarball = join(packDirectory, adapterMetadata.filename);
    const extracted = join(temporary, "extracted");
    await mkdir(extracted);
    run("tar", ["-xzf", tarball, "-C", extracted]);
    const packageRoot = join(extracted, "package");
    const packageJSON = JSON.parse(await readFile(join(packageRoot, "package.json"), "utf8"));
    assert.equal(packageJSON.name, "@hsblabs/scrape-kdl");
    assert.equal(packageJSON.license, "Apache-2.0");
    assert.equal(packageJSON.type, "module");
    assert.equal(packageJSON.engines.node, ">=26");
    assert.equal(packageJSON.version, releaseVersion ?? developmentVersion);
    assert.equal(packageJSON.publishConfig?.access, publishAccess);
    if (releaseVersion !== undefined) assert.equal(packageJSON.scripts, undefined);
    assert.deepEqual(
      packageJSON.dependencies,
      { parse5: "8.0.1", re2js: "2.8.6" },
      "core package must pin only the approved HTML parser and portable regex runtime dependencies",
    );
    assert.equal(packageJSON.peerDependencies, undefined, "core package must not acquire browser-library peers");
    assert.equal(
      JSON.stringify(packageJSON).toLowerCase().includes("playwright"),
      false,
      "core package metadata must not mention Playwright",
    );
    assert.doesNotMatch(
      JSON.stringify(packageJSON),
      /(?:file|workspace):/u,
      "core package metadata must not contain local dependency protocols",
    );

    const adapterExtracted = join(temporary, "adapter-extracted");
    await mkdir(adapterExtracted);
    run("tar", ["-xzf", adapterTarball, "-C", adapterExtracted]);
    const adapterRoot = join(adapterExtracted, "package");
    const adapterJSON = JSON.parse(await readFile(join(adapterRoot, "package.json"), "utf8"));
    assert.equal(adapterJSON.name, "@hsblabs/scrape-kdl-playwright");
    assert.equal(adapterJSON.private, undefined, "adapter package must be publishable");
    assert.equal(adapterJSON.engines.node, ">=26");
    assert.equal(adapterJSON.version, releaseVersion ?? developmentVersion);
    assert.equal(adapterJSON.publishConfig?.access, publishAccess);
    if (releaseVersion !== undefined) assert.equal(adapterJSON.scripts, undefined);
    assert.deepEqual(adapterJSON.dependencies, { playwright: "1.61.1" });
    if (releaseVersion === undefined) {
      assert.deepEqual(adapterJSON.devDependencies, {
        "@hsblabs/scrape-kdl": developmentVersion,
      });
      assert.deepEqual(adapterJSON.peerDependencies, {
        "@hsblabs/scrape-kdl": `${developmentVersion} || ^1.0.0`,
      });
    } else {
      assert.deepEqual(adapterJSON.devDependencies, {
        "@hsblabs/scrape-kdl": releaseVersion,
      });
      assert.deepEqual(adapterJSON.peerDependencies, {
        "@hsblabs/scrape-kdl": releasePeerRange(releaseVersion),
      });
      assert.doesNotMatch(
        JSON.stringify(packageJSON),
        /0\.0\.0-development/u,
        "release core metadata must not contain the workspace development version",
      );
      assert.doesNotMatch(
        JSON.stringify(adapterJSON),
        /0\.0\.0-development/u,
        "release adapter metadata must not contain the workspace development version",
      );
    }
    assert.doesNotMatch(
      JSON.stringify(adapterJSON),
      /(?:file|workspace):/u,
      "adapter package metadata must not contain local dependency protocols",
    );

    const extractedFiles = [];
    await collect(packageRoot, extractedFiles);
    const forbidden = [
      /\/Users\//u,
      /[A-Za-z]:\\/u,
      /-----BEGIN [A-Z ]*PRIVATE KEY-----/u,
      /(?:npm_|github_|ghp_)[A-Za-z0-9]{12,}/u,
      /(?:node:child_process|Bun\.spawn|Deno\.Command)/u,
    ];
    for (const path of extractedFiles) {
      const data = await readFile(path, "utf8");
      for (const pattern of forbidden)
        assert.doesNotMatch(
          data,
          pattern,
          `${relative(packageRoot, path)} contains forbidden local or secret material`,
        );
    }
    const adapterFiles = [];
    await collect(adapterRoot, adapterFiles);
    for (const path of adapterFiles) {
      const data = await readFile(path, "utf8");
      for (const pattern of forbidden)
        assert.doesNotMatch(
          data,
          pattern,
          `${relative(adapterRoot, path)} contains forbidden local or secret material`,
        );
    }

    const consumer = join(temporary, "consumer");
    await mkdir(consumer);
    const fixture = await readFile(join(root, "fixtures/valid/basic-http.kdl"), "utf8");
    await writeFile(join(consumer, "extractor.kdl"), fixture);
    await writeFile(
      join(consumer, "package.json"),
      `${JSON.stringify(
        {
          name: "clean-consumer",
          private: true,
          type: "module",
          devDependencies: { "@types/node": "26.1.1" },
        },
        null,
        2,
      )}\n`,
    );
    await writeFile(join(consumer, "index.mjs"), runtimeConsumerSource);
    await writeFile(join(consumer, "consumer.ts"), typeConsumerSource);
    await writeFile(
      join(consumer, "tsconfig.json"),
      `${JSON.stringify(
        {
          compilerOptions: {
            target: "ESNext",
            module: "NodeNext",
            moduleResolution: "NodeNext",
            lib: ["ESNext", "DOM", "DOM.Iterable"],
            types: ["node"],
            strict: true,
            exactOptionalPropertyTypes: true,
            noEmit: true,
          },
          include: ["consumer.ts"],
        },
        null,
        2,
      )}\n`,
    );
    run("npm", ["install", "--ignore-scripts", "--no-audit", "--no-fund", tarball, adapterTarball], consumer);
    run(
      process.execPath,
      [join(root, "node_modules/typescript/bin/tsc"), "--project", join(consumer, "tsconfig.json")],
      consumer,
    );
    run(process.execPath, [join(consumer, "index.mjs")], consumer);

    if (outputDirectory !== undefined) {
      await mkdir(outputDirectory, { recursive: true });
      const destinations = [join(outputDirectory, basename(tarball)), join(outputDirectory, basename(adapterTarball))];
      for (const destination of destinations) await requireAbsent(destination);
      await copyFile(tarball, destinations[0], constants.COPYFILE_EXCL);
      await copyFile(adapterTarball, destinations[1], constants.COPYFILE_EXCL);
    }
    console.log(
      `npm package check: ${basename(tarball)} and ${basename(adapterTarball)} passed clean consumer smoke tests`,
    );
    return [basename(tarball), basename(adapterTarball)];
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
}

async function requireAbsent(path) {
  try {
    await access(path);
  } catch (error) {
    if (error?.code === "ENOENT") return;
    throw error;
  }
  throw new Error(`refusing to overwrite existing release artifact ${path}`);
}

function packWorkspacePackages(packDirectory) {
  return {
    packed: JSON.parse(
      run("npm", ["pack", "--json", "--workspace", "@hsblabs/scrape-kdl", "--pack-destination", packDirectory]),
    ),
    adapterPacked: JSON.parse(
      run("npm", [
        "pack",
        "--json",
        "--workspace",
        "@hsblabs/scrape-kdl-playwright",
        "--pack-destination",
        packDirectory,
      ]),
    ),
  };
}

async function packReleasePackages(temporary, packDirectory, releaseVersion, publishAccess) {
  const coreStage = join(temporary, "core");
  const adapterStage = join(temporary, "adapter");
  await stagePackage(join(root, "packages/scrape-kdl"), coreStage, (manifest) => {
    manifest.version = releaseVersion;
    manifest.publishConfig = { ...manifest.publishConfig, access: publishAccess };
  });
  await stagePackage(join(root, "packages/scrape-kdl-playwright"), adapterStage, (manifest) => {
    manifest.version = releaseVersion;
    manifest.publishConfig = { ...manifest.publishConfig, access: publishAccess };
    manifest.devDependencies = { "@hsblabs/scrape-kdl": releaseVersion };
    manifest.peerDependencies = {
      "@hsblabs/scrape-kdl": releasePeerRange(releaseVersion),
    };
  });
  return {
    packed: JSON.parse(run("npm", ["pack", "--json", "--pack-destination", packDirectory], coreStage)),
    adapterPacked: JSON.parse(run("npm", ["pack", "--json", "--pack-destination", packDirectory], adapterStage)),
  };
}

async function stagePackage(source, destination, mutateManifest) {
  await mkdir(destination);
  await cp(join(source, "dist"), join(destination, "dist"), {
    recursive: true,
  });
  for (const name of ["README.md", "LICENSE", "NOTICE"]) await copyFile(join(source, name), join(destination, name));
  const manifest = JSON.parse(await readFile(join(source, "package.json"), "utf8"));
  mutateManifest(manifest);
  delete manifest.scripts;
  await writeFile(join(destination, "package.json"), `${JSON.stringify(manifest, null, 2)}\n`);
}

function validateReleaseVersion(version) {
  assert.match(
    version,
    /^(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/u,
    `invalid npm release version ${JSON.stringify(version)}`,
  );
}

function validatePublishAccess(access) {
  assert.match(access, /^(?:public|restricted)$/u, `invalid npm publish access ${JSON.stringify(access)}`);
}

function releasePeerRange(version) {
  return version.includes("-") ? version : `^${version}`;
}

function run(command, arguments_, cwd = root) {
  return execFileSync(command, arguments_, {
    cwd,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
}

async function collect(directory, output) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) await collect(path, output);
    else if (entry.isFile()) output.push(path);
  }
}

const runtimeConsumerSource = `
import assert from "node:assert/strict";
import { ExecutionError, compile, supportedIRVersions, supportedLanguageVersions, validate } from "@hsblabs/scrape-kdl";
import { builtinCatalog, callBuiltin, float, supportedBuiltinCatalogVersions, write } from "@hsblabs/scrape-kdl/authoring";
import { compileFile, validateFile } from "@hsblabs/scrape-kdl/node";
import { PlaywrightAdapter } from "@hsblabs/scrape-kdl-playwright";

assert.equal(typeof ExecutionError, "function");
assert.equal(typeof PlaywrightAdapter, "function");
assert.equal(typeof compile, "function");
assert.equal(typeof validate, "function");
assert.deepEqual(supportedLanguageVersions(), ["2026-07-15"]);
assert.deepEqual(supportedIRVersions(), ["2026-07-15"]);
assert.deepEqual(supportedBuiltinCatalogVersions(), ["2026-07-15"]);
assert.deepEqual(float(1), { kind: "float", value: 1 });
const source = await import("node:fs/promises").then(({ readFile }) => readFile(new URL("./extractor.kdl", import.meta.url)));
const memory = await compile({ path: "extractor.kdl", data: source });
assert.equal(memory.program.metadata.name, "basic-http");
assert.deepEqual(await memory.program.extract({ id: "package-smoke" }, {
  fetch: async () => new Response(
    "<!doctype html><html><body><h1> Package Smoke </h1><ul class=items><li><span class=value>1</span></li></ul></body></html>",
    { status: 200, headers: { "content-type": "text/html; charset=utf-8" } },
  ),
}), { value: { title: "Package Smoke", items: [{ value: 1 }] }, warnings: [], partial: false });
assert.deepEqual(await validate({ path: "extractor.kdl", data: source }), []);
const imported = await compile({
  path: "spec/extractor.kdl",
  data: "import \\"./common.kdl\\" as=\\"common\\"\\n" + new TextDecoder().decode(source),
}, {
  loader: {
    async load(path, context) {
      assert.equal(path, "spec/common.kdl");
      assert.equal(context.fromPath, "spec/extractor.kdl");
      return "module \\"common\\" version=\\"2026-07-15\\" language-version=\\"2026-07-15\\" {}";
    },
  },
});
assert.equal(imported.program.metadata.name, "basic-http");
assert.equal((await compileFile(new URL("./extractor.kdl", import.meta.url).pathname)).program.metadata.name, "basic-http");
assert.deepEqual(await validateFile(new URL("./extractor.kdl", import.meta.url).pathname), []);
const authoringCatalog = builtinCatalog("2026-07-15");
const normalize = authoringCatalog.builtins.find(({ name }) => name === "normalize-whitespace");
assert.ok(normalize);
const authored = write({
  languageVersion: "2026-07-15",
  extractor: {
    name: "package-authored", version: "2026-07-15",
    source: { fetchMode: "http", urlTemplate: "https://example.invalid/", sessionPolicy: "none" },
    inputs: [],
    members: [{
      kind: "field", name: "title", type: "string", required: true, selector: "h1", match: "one",
      value: { kind: "text" }, transforms: [callBuiltin(normalize)], onError: "fail",
    }],
  },
});
assert.equal((await compile({ path: "package-authored.kdl", data: authored })).program.metadata.name, "package-authored");
`;

const typeConsumerSource = `
import { ExecutionError, compile, supportedIRVersions, supportedLanguageVersions, validate } from "@hsblabs/scrape-kdl";
import { builtinCatalog, callBuiltin, float, supportedBuiltinCatalogVersions, write } from "@hsblabs/scrape-kdl/authoring";
import { compileFile, validateFile } from "@hsblabs/scrape-kdl/node";
import { PlaywrightAdapter } from "@hsblabs/scrape-kdl-playwright";
import type {
  BrowserAdapter, BrowserAdapterLease, BrowserAdapterQueryLimit, BrowserElement, BrowserEvaluateOptions, BrowserNavigateOptions,
  BrowserOperationOptions, CompileOptions, CompileResult, DiagnosticIR, ExecutionOptions, ExternalTransform,
  ExternalTransformContext, ExtractionResult, ExtractorIR, JsonValue, Program, ProgramMetadata, Session,
  SessionCookie, Source, SourceFile, SourceLoadContext, SourceLoader, URLPolicyContext, Warning,
} from "@hsblabs/scrape-kdl";
import type {
  ArgumentConstraint, AuthoringCollection, AuthoringDocument, AuthoringExtractor, AuthoringField, AuthoringFloat, AuthoringInput,
  AuthoringMember, AuthoringScalar, AuthoringSource, AuthoringValueSource, BuiltinCall, BuiltinCatalog,
  BuiltinDefinition, InputConstraint, NamedArgument, NullabilityEffect, OutputConstraint, PositionalArguments,
} from "@hsblabs/scrape-kdl/authoring";

type PublicTypes = [
  BrowserAdapter, BrowserAdapterLease, BrowserAdapterQueryLimit, BrowserElement, BrowserEvaluateOptions, BrowserNavigateOptions,
  BrowserOperationOptions, CompileOptions, CompileResult, DiagnosticIR, ExecutionOptions, ExternalTransform,
  ExternalTransformContext, ExtractionResult, ExtractorIR, JsonValue, Program, ProgramMetadata, Session,
  SessionCookie, Source, SourceFile, SourceLoadContext, SourceLoader, URLPolicyContext, Warning,
];
declare const publicTypes: PublicTypes;
void publicTypes;
type AuthoringTypes = [
  ArgumentConstraint, AuthoringCollection, AuthoringDocument, AuthoringExtractor, AuthoringField, AuthoringFloat, AuthoringInput,
  AuthoringMember, AuthoringScalar, AuthoringSource, AuthoringValueSource, BuiltinCall, BuiltinCatalog,
  BuiltinDefinition, InputConstraint, NamedArgument, NullabilityEffect, OutputConstraint, PositionalArguments,
];
declare const authoringTypes: AuthoringTypes;
void authoringTypes;
void ExecutionError;
void compile;
void validate;
void supportedLanguageVersions;
void supportedIRVersions;
void builtinCatalog;
void callBuiltin;
void float;
void supportedBuiltinCatalogVersions;
void write;
void compileFile;
void validateFile;
void PlaywrightAdapter;
`;

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) await checkNpmPackages();
