---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: TypeScript でのコンパイルと抽出
description: TypeScript の実行 API。注入したローダでのコンパイル、実行オプション、抽出結果、外部トランスフォーム、URL ポリシー、キャンセル。
hsblabs:
  sidebar:
    order: 41
---

このページはコンパイルと実行の API を示す。
ここに出る型はすべて `@hsblabs/scrape-kdl` パッケージのものである。

## コンパイル

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

`Source` は `path` と `data` を持つ。
`path` は、診断、ソース位置、import の解決、IR 内のファイル識別に使う論理的な識別子である。
OS 上のファイル名である必要はない。

コンパイルの結果は `program` と `diagnostics` を持つ。
診断にエラーが含まれる場合、`program` フィールドは存在しない。
使う前に必ず `program` を検査する。

`/node` エントリポイントからは `compileFile(path)` と `validateFile(path)` も使える。
`validate` 関数は診断だけを返し、プログラムは返さない。

## import

import を含むソースにはローダが必要である。
ローダがなければ、コンパイルは IR を返す前に失敗する。

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

コンパイラは各 import を、それを import しているソースからの相対で字句的に解決してから、ローダを呼ぶ。
ローダが返すのはバイト列だけである。
解析、検証、循環の検出、ハッシュ、決定的な順序づけはコンパイラが行う。

ローダの失敗は、ドキュメントの診断ではなく運用上のエラーである。
Promise は中断の理由か `SourceLoadError` で reject する。
`SourceLoadError.cause` フィールドが元の失敗を保持する。

## 実行

```ts
const result = await program.extract(
  { id: "123" },
  { requestTimeoutMs: 15_000 },
);
```

第一引数は実行時の入力である。
第二引数は実行オプションである。

| オプション | 役割 |
| --- | --- |
| `browser` | ブラウザモードのプログラム向けの `BrowserAdapter`。 |
| `allowJavaScript` | `evaluate-js` を許可する。既定は無効。 |
| `fetch` | 自前の `fetch` 実装。 |
| `session` | 最初のリクエストに付けるヘッダとクッキー。 |
| `externalTransforms` | ホスト関数のレジストリ。 |
| `requestTimeoutMs` | HTTP リクエストのタイムアウト。 |
| `maxResponseBytes` | レスポンスボディの上限。 |
| `userAgent` | HTTP リクエストの User-Agent。 |
| `urlPolicy` | リクエストの前に各 URL を検査する関数。 |
| `signal` | キャンセル用の `AbortSignal`。 |

## 結果

```ts
interface ExtractionResult {
  readonly value: Readonly<Record<string, JsonValue>>;
  readonly warnings: readonly Warning[];
  readonly partial: boolean;
}
```

`Warning` は `code`、`message`、任意の `path`、任意の `row` を持つ。
`partial` フラグが `true` になるのは、ランタイムがエラーから回復したか、行を捨てた場合だけである。

失敗すると `ExecutionError` が返る。
`code`、任意の `path`、任意の `cause` を持つ。
検査すべきは `code` である。
[診断](../guides/diagnostics.md) を参照。

`value` フィールドは動的な JSON 値である。
抽出器は宣言された出力型を検証するが、フィールド名と TypeScript のモデルとの対応は、依然としてアプリケーション側の責任である。
その境界では、自前のスキーマで値を検証すること。
検査を伴わない型アサーションを使ってはならない。

## オフラインスナップショット

```ts
const html = await readFile("./page.html", "utf8");
const result = await program.extractSnapshot(html);
```

`extractSnapshot` メソッドは取得を行わない。
HTTP モードのプログラムもブラウザモードのプログラムも受け付けるが、そのプログラムが適格である場合に限る。
ワークフロー、または `evaluate-js` のフィールド値ソースを持つプログラムは `E_SNAPSHOT_UNSUPPORTED` で失敗する。
[オフラインスナップショット](../guides/offline-snapshots.md) を参照。

## 外部トランスフォーム

```ts
const result = await program.extract(inputs, {
  externalTransforms: {
    decrypt_payload: async (context, input) => decrypt(input as string),
  },
});
```

関数は、任意の `signal` を含むコンテキストと入力を受け取る。
返すのは JSON 値、またはその Promise である。

プログラムが必要とするシンボルがレジストリにない場合、検証は fetch より前に失敗する。
関数が結果を返した直後に、ランタイムはその結果を宣言された出力型に照らして検査する。

## URL ポリシー

```ts
const result = await program.extract(inputs, {
  urlPolicy: (context, url) => {
    if (url.hostname !== "example.com") {
      throw new Error(`refused host: ${url.hostname}`);
    }
  },
});
```

ポリシーは最初のリクエストの前と、各リダイレクトの前に実行される。
TypeScript ランタイムはリダイレクトを自前で処理するため、ポリシーはすべてのホップを見る。
ポリシーがエラーを返すと、抽出は `E_URL_POLICY` で止まる。

ランタイムは、オリジンをまたぐ際に認証ヘッダとホスト限定のクッキーヘッダを除去し、ストリームされたボディはデコードの前に上限を適用する。

TypeScript パッケージに、公開インターネット向けの用意済みポリシーはない。
自分でポリシーを書くこと。
Go では `PublicInternetURLPolicy` が相当する。

## キャンセル

```ts
const controller = new AbortController();
setTimeout(() => controller.abort(), 5_000);

const result = await program.extract(inputs, { signal: controller.signal });
```

ランタイムは、親のキャンセルをリクエストのタイムアウトとは別に伝播する。
シグナルは、HTML の解析の前、各出力メンバの前、コレクションの各行の前に検査される。
キャンセルは `E_EXECUTION_CANCELED` になる。
フィールドポリシーでも行ポリシーでも回復できない。

## 互換性

```ts
import { supportedLanguageVersions, supportedIRVersions } from "@hsblabs/scrape-kdl";
```

この二つの関数は、このビルドが受け付ける正確なバージョンを返す。
バージョンは正確に指定すること。
`latest` のような移動する別名はない。

## 次に読むもの

- [Playwright アダプタ](./playwright.md)：ブラウザモードのプログラムを実行する方法。
- [パターン](../guides/patterns.md)：複数ページにまたがるループ。
