import {
  compile,
  type BrowserAdapter,
  type BrowserElement,
  type ExecutionOptions,
  type JsonValue,
  type SourceLoader,
} from "@hsblabs/scrape-kdl";
import { compileFile } from "@hsblabs/scrape-kdl/node";

const files = new Map<string, string>([
  ["spec/common.kdl", `module "common" version="2026-07-15" language-version="2026-07-15" {}`],
]);

const loader: SourceLoader = {
  async load(path) {
    const data = files.get(path);
    if (data === undefined) throw new Error(`missing source: ${path}`);
    return data;
  },
};

const browser: BrowserAdapter = {
  async navigate() {},
  async waitFor() {},
  async click() {},
  async fill() {},
  async press() {},
  async scroll() {},
  async waitForNetworkIdle() {},
  async evaluate(): Promise<JsonValue> { return null; },
  async queryAll(): Promise<readonly BrowserElement[]> { return []; },
  async text() { return ""; },
  async html() { return ""; },
  async attribute() { return undefined; },
};

const source = {
  path: "spec/main.kdl",
  data: `import "common.kdl" as="common"
extractor "consumer" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`,
};

const compiled = await compile(source, { loader });
if (compiled.program !== undefined) {
  const options: ExecutionOptions = {
    browser,
    externalTransforms: {
      async decorate(_context, input) { return input; },
    },
    urlPolicy(_context, url) {
      if (url.protocol !== "https:") throw new Error("HTTPS required");
    },
  };
  void compiled.program.metadata.capabilities;
  void compiled.program.ir;
  void compiled.program.extract({}, options);
}

void compileFile("extractor.kdl");
