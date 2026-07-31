import {
  compile,
  SourceLoadError,
  type BrowserAdapter,
  type BrowserElement,
  type ExecutionOptions,
  type JsonValue,
  type SourceLoader,
} from "@hsblabs/scrape-kdl";
import { compileFile } from "@hsblabs/scrape-kdl/node";
import { builtinCatalog, callBuiltin, write, type AuthoringDocument } from "@hsblabs/scrape-kdl/authoring";

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
  async evaluate(): Promise<JsonValue> {
    return null;
  },
  async queryAll(): Promise<readonly BrowserElement[]> {
    return [];
  },
  async text() {
    return "";
  },
  async html() {
    return "";
  },
  async attribute() {
    return undefined;
  },
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
      async decorate(_context, input) {
        return input;
      },
    },
    urlPolicy(_context, url) {
      if (url.protocol !== "https:") throw new Error("HTTPS required");
    },
  };
  void compiled.program.metadata.capabilities;
  void compiled.program.descriptor.source.fetchMode;
  void compiled.program.descriptor.source.urlTemplate;
  void compiled.program.descriptor.source.sessionPolicy;
  void compiled.program.ir;
  void compiled.program.extract({}, options);
  void compiled.program.extractSnapshot("<h1>Snapshot</h1>", options);
}

try {
  await compile(source, { loader });
} catch (error) {
  if (error instanceof SourceLoadError) {
    void error.path;
    void error.fromPath;
    void error.cause;
  } else {
    throw error;
  }
}

void compileFile("extractor.kdl");

const catalog = builtinCatalog("2026-07-15");
const normalize = catalog.builtins.find(({ name }) => name === "normalize-whitespace");
if (normalize === undefined) throw new Error("authoring catalog is missing normalize-whitespace");
const authoredDocument: AuthoringDocument = {
  languageVersion: "2026-07-15",
  extractor: {
    name: "authored-consumer",
    version: "2026-07-15",
    source: { fetchMode: "http", urlTemplate: "https://example.invalid/", sessionPolicy: "none" },
    inputs: [],
    members: [
      {
        kind: "field",
        name: "title",
        type: "string",
        required: true,
        selector: "h1",
        match: "one",
        value: { kind: "text" },
        transforms: [callBuiltin(normalize)],
        onError: "fail",
      },
    ],
  },
};
void compile({ path: "authored-consumer.kdl", data: write(authoredDocument) });
