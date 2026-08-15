---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Compile and Extract in TypeScript
description: The TypeScript execution API — compile with an injected loader, the execution options, the extraction result, the external transforms, the URL policy, and the cancellation.
hsblabs:
  sidebar:
    order: 41
---

This page shows you the API of the compilation and the execution. Each type here comes from the package `@hsblabs/scrape-kdl`.

## Compile

```ts
import { readFile } from "node:fs/promises";
import { compile } from "@hsblabs/scrape-kdl";

const compiled = await compile({
  path: "extractor.kdl",
  data: await readFile("extractor.kdl", "utf8"),
});

if (!compiled.program) {
  throw new Error(JSON.stringify(compiled.diagnostics));
}

const result = await compiled.program.extract({ id: "123" });
console.log(result.value);
```

A `Source` has a `path` and a `data`. The `path` is a logical identity for the diagnostics, the source locations, the resolution of the imports, and the file identities in the IR. It does not have to name a file of the operating system.

The result of a compilation has a `program` and a `diagnostics`. The field `program` is absent when the diagnostics have an error. Always examine `program` before you use it.

From the entry point `/node` you can also use `compileFile(path)` and `validateFile(path)`. The function `validate` gives only the diagnostics and no program.

## The imports

A source with an import needs a loader. Without a loader, the compilation fails before it gives an IR.

```ts
import { compile, type SourceLoader } from "@hsblabs/scrape-kdl";

const loader: SourceLoader = {
  async load(path, context) {
    if (!allowedPaths.has(path)) {
      throw new Error(`refused: ${path}`);
    }
    return await readFile(path, "utf8");
  },
};

const compiled = await compile(source, { loader });
```

The compiler resolves each import lexically, relative to the source that imports it, before it calls your loader. Your loader gives only the bytes. The compiler makes the parse, the validation, the detection of the cycles, the hash, and the deterministic order.

A failure of the loader is an operational error and not a diagnostic of the document. The promise rejects with the reason of the abort or with a `SourceLoadError`. The field `SourceLoadError.cause` keeps the original failure.

## Execute

```ts
const result = await program.extract(
  { id: "123" },
  { requestTimeoutMs: 15_000 },
);
```

The first argument has the runtime inputs. The second argument has the execution options:

| Option | Function |
| --- | --- |
| `browser` | The `BrowserAdapter` for a browser-mode program. |
| `allowJavaScript` | Permits `evaluate-js`. The default is off. |
| `fetch` | Your own implementation of `fetch`. |
| `session` | The headers and the cookies for the initial request. |
| `externalTransforms` | The registry of the host functions. |
| `requestTimeoutMs` | The timeout of the HTTP request. |
| `maxResponseBytes` | The limit of the body of the response. |
| `userAgent` | The User-Agent of the HTTP request. |
| `urlPolicy` | A function that examines each URL before the request. |
| `signal` | An `AbortSignal` for the cancellation. |

## The result

```ts
interface ExtractionResult {
  readonly value: Readonly<Record<string, JsonValue>>;
  readonly warnings: readonly Warning[];
  readonly partial: boolean;
}
```

A `Warning` has a `code`, a `message`, an optional `path`, and an optional `row`. The flag `partial` is `true` only after the runtime recovered an error or dropped a row.

A failure gives an `ExecutionError`. It has a `code`, an optional `path`, and an optional `cause`. Examine the `code`. Refer to [Diagnostics](../guides/diagnostics.md).

The field `value` is a dynamic JSON value. The extractor validates its declared output types, but your application still owns the correspondence between the names of the fields and your TypeScript model. Validate the value with your own schema at that boundary. Do not use an unchecked type assertion.

## The offline snapshot

```ts
const html = await readFile("./page.html", "utf8");
const result = await program.extractSnapshot(html);
```

The method `extractSnapshot` does no acquisition. It accepts an HTTP-mode program and also a browser-mode program, but the program must be eligible. A program with a workflow or with an `evaluate-js` field value source fails with `E_SNAPSHOT_UNSUPPORTED`. Refer to [Offline Snapshots](../guides/offline-snapshots.md).

## The external transforms

```ts
const result = await program.extract(inputs, {
  externalTransforms: {
    decrypt_payload: async (context, input) => decrypt(input as string),
  },
});
```

A function gets a context with an optional `signal`, and the input. It gives a JSON value or a promise of a JSON value.

If the registry does not have a symbol that the program needs, the validation fails before the fetch. After your function gives a result, the runtime immediately examines the result against the declared output type.

## The URL policy

```ts
const result = await program.extract(inputs, {
  urlPolicy: (context, url) => {
    if (url.hostname !== "example.com") {
      throw new Error(`refused host: ${url.hostname}`);
    }
  },
});
```

The policy executes before the initial request and before each redirect. The TypeScript runtime makes each redirect itself, thus the policy sees each hop. An error from the policy stops the extraction with `E_URL_POLICY`.

The runtime also removes an authorization header and a host-only cookie header between the origins, and it limits the streamed body before it decodes it.

The TypeScript package has no prepared policy for the public internet. Write your own policy. The Go equivalent is `PublicInternetURLPolicy`.

## The cancellation

```ts
const controller = new AbortController();
setTimeout(() => controller.abort(), 5_000);

const result = await program.extract(inputs, { signal: controller.signal });
```

The runtime propagates the cancellation of the parent separately from the timeout of the request. It examines the signal before the parse of the HTML and also before each output member and each collection row. A cancellation gives `E_EXECUTION_CANCELED`. A field policy or a row policy cannot recover it.

## The compatibility

```ts
import { supportedLanguageVersions, supportedIRVersions } from "@hsblabs/scrape-kdl";
```

The two functions give the exact versions that this build accepts. Select an exact version. There is no moving alias `latest`.

## Next step

- [Playwright Adapter](./playwright.md) — how to execute a browser-mode program.
- [Patterns](../guides/patterns.md) — a loop over more than one page.
