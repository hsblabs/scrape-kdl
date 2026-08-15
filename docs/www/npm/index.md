---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: TypeScript and Bun
description: The npm packages of Scraping KDL — the three entry points, the reason the core package has no access to the file system, and the position of the Playwright adapter.
hsblabs:
  sidebar:
    order: 40
---

The package `@hsblabs/scrape-kdl` gives you the compiler, the diagnostics, the IR, the HTTP runtime, the offline snapshot runtime, and the types of the browser adapter. It supports Node.js 22 or later and Bun 1.3 or later. It supports only ESM.

```bash
npm install @hsblabs/scrape-kdl@1.0.4
```

## The three entry points

| Entry point | Content |
| --- | --- |
| `@hsblabs/scrape-kdl` | `compile`, `validate`, the `Program` interface, the execution options, and the types of the browser adapter. |
| `@hsblabs/scrape-kdl/node` | `compileFile` and `validateFile`. It also exports each part of the core entry point again. |
| `@hsblabs/scrape-kdl/authoring` | The bounded authoring model and the catalog of the built-in transforms. |

The division is intentional. The core package has no automatic access to the file system and no automatic network loader. It resolves a relative path lexically and asks a `SourceLoader` for the bytes.

```ts
export interface SourceLoader {
  load(path: string, context: SourceLoadContext): Promise<string | Uint8Array>;
}
```

Thus you decide which files the compiler can read. The entry point `/node` supplies the loader for the file system. Use `/node` when the compilation of a local file is correct for you. Use the core entry point with your own loader when you must limit the sources.

Your loader is an authority boundary. Limit the paths to the intended set, obey the `AbortSignal`, and do not put the content of a source or a credential in an error.

## The first compilation

```ts
import { compileFile } from "@hsblabs/scrape-kdl/node";

const { program, diagnostics } = await compileFile("./extractor.kdl");

if (!program) {
  for (const diagnostic of diagnostics) {
    console.error(`${diagnostic.code}: ${diagnostic.message}`);
  }
  process.exit(1);
}

const result = await program.extract({ id: "123" });
console.log(result.value);
```

The function `compile` gives a `CompileResult`. The field `program` is absent when the diagnostics have an error. Examine `program` before you use it. Refer to [Compile and Extract in TypeScript](./compile-and-extract.md).

## The metadata of a program

A compiled program tells you what it needs, before you execute it:

```ts
program.metadata.capabilities; // readonly string[]
program.metadata.languageVersion; // "2026-07-15"
program.metadata.irVersion; // "2026-07-15"
program.metadata.files; // each source file, with its SHA-256
program.descriptor.source.fetchMode; // "http" or "browser"
program.descriptor.source.sessionPolicy; // "none", "optional", or "required"
```

Use `metadata.capabilities` to permit or to refuse a program in your host. Use `metadata.files` to make a record of the exact sources that you compiled.

## The browser mode

The core package does not contain a browser. It declares the interface `BrowserAdapter` and executes a browser-mode program with the adapter that you give in `options.browser`.

The official adapter is a separate package:

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
```

Refer to [Playwright Adapter](./playwright.md) and to [Browser Mode](../guides/browser-mode.md).

## The authoring model

The entry point `/authoring` makes a KDL document from a structure of data. Use it in an editor, a generator, or a tool that makes an extractor from a selection of the user.

```ts
import { builtinCatalog, write } from "@hsblabs/scrape-kdl/authoring";

const catalog = builtinCatalog("2026-07-15");
```

The catalog is versioned. Select the exact language version. Do not use a version `latest`. The function `write` makes the KDL text. Then compile that text with the ordinary compiler and examine the diagnostics.

## Bun

Bun 1.3 or later supports the core package:

```bash
bun add @hsblabs/scrape-kdl@1.0.4
```

The tests of the Playwright adapter use Node.js 22 or later.

## Next step

- [Compile and Extract in TypeScript](./compile-and-extract.md) — the full API of the execution.
- [Playwright Adapter](./playwright.md) — browser mode with an official adapter.
