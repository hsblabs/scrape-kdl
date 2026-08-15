---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: TypeScript と Bun
description: Scraping KDL の npm パッケージ。三つのエントリポイント、コアパッケージがファイルシステムへアクセスしない理由、Playwright アダプタの位置づけ。
hsblabs:
  sidebar:
    order: 40
---

`@hsblabs/scrape-kdl` パッケージは、コンパイラ、診断、IR、HTTP ランタイム、オフラインスナップショットのランタイム、ブラウザアダプタの型を提供する。
対応は Node.js 22 以降と Bun 1.3 以降である。
ESM のみに対応する。

```bash
npm install @hsblabs/scrape-kdl@1.0.4
```

## 三つのエントリポイント

| エントリポイント | 内容 |
| --- | --- |
| `@hsblabs/scrape-kdl` | `compile`、`validate`、`Program` インターフェース、実行オプション、ブラウザアダプタの型。 |
| `@hsblabs/scrape-kdl/node` | `compileFile` と `validateFile`。コアエントリポイントの各要素も再エクスポートする。 |
| `@hsblabs/scrape-kdl/authoring` | 制約付きのオーサリングモデルと組み込みトランスフォームのカタログ。 |

この分割は意図的なものである。
コアパッケージは、ファイルシステムへの自動アクセスもネットワークローダも持たない。
相対パスを字句的に解決し、バイト列は `SourceLoader` に要求する。

```ts
export interface SourceLoader {
  load(path: string, context: SourceLoadContext): Promise<string | Uint8Array>;
}
```

したがって、コンパイラがどのファイルを読めるかを決めるのは利用者である。
`/node` エントリポイントはファイルシステム用のローダを供給する。
ローカルファイルのコンパイルで問題ないなら `/node` を使う。
ソースを限定する必要があるなら、コアエントリポイントに自前のローダを渡す。

ローダは権限の境界である。
パスを意図した集合に限定し、`AbortSignal` に従い、ソースの内容や資格情報をエラーに含めないこと。

## 最初のコンパイル

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

`compile` 関数は `CompileResult` を返す。
診断にエラーが含まれる場合、`program` フィールドは存在しない。
使う前に必ず `program` を検査する。
[TypeScript でのコンパイルと抽出](./compile-and-extract.md) を参照。

## プログラムのメタデータ

コンパイル済みのプログラムは、実行する前に自分が必要とするものを伝える。

```ts
program.metadata.capabilities; // readonly string[]
program.metadata.languageVersion; // "2026-07-15"
program.metadata.irVersion; // "2026-07-15"
program.metadata.files; // each source file, with its SHA-256
program.descriptor.source.fetchMode; // "http" or "browser"
program.descriptor.source.sessionPolicy; // "none", "optional", or "required"
```

`metadata.capabilities` は、ホスト側でプログラムを許可するか拒否するかの判断に使う。
`metadata.files` は、実際にコンパイルしたソースを記録するために使う。

## ブラウザモード

コアパッケージはブラウザを含まない。
`BrowserAdapter` インターフェースを宣言し、`options.browser` に渡されたアダプタでブラウザモードのプログラムを実行する。

公式アダプタは別のパッケージである。

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
```

[Playwright アダプタ](./playwright.md) と [ブラウザモード](../guides/browser-mode.md) を参照。

## オーサリングモデル

`/authoring` エントリポイントは、データ構造から KDL ドキュメントを作る。
エディタ、ジェネレータ、利用者の選択から抽出器を作るツールで使う。

```ts
import { builtinCatalog, write } from "@hsblabs/scrape-kdl/authoring";

const catalog = builtinCatalog("2026-07-15");
```

カタログはバージョン管理されている。
言語バージョンを正確に指定すること。
`latest` のような指定は使わない。
`write` 関数が KDL テキストを作る。
そのテキストを通常のコンパイラでコンパイルし、診断を確認する。

## Bun

Bun 1.3 以降がコアパッケージに対応している。

```bash
bun add @hsblabs/scrape-kdl@1.0.4
```

Playwright アダプタのテストは Node.js 22 以降を使う。

## 次に読むもの

- [TypeScript でのコンパイルと抽出](./compile-and-extract.md)：実行 API の全体。
- [Playwright アダプタ](./playwright.md)：公式アダプタによるブラウザモード。
