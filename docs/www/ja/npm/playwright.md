---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Playwright アダプタ
description: TypeScript 向けの公式 Playwright ブラウザアダプタ。ブラウザの所有権、コンテキストごとの分離、タイムアウト後の後始末、対応ブラウザ。
hsblabs:
  sidebar:
    order: 42
---

`@hsblabs/scrape-kdl-playwright` パッケージは `BrowserAdapter` と `BrowserAdapterLease` の契約を実装する。
これは独立したパッケージである。
そのため、コアパッケージの依存グラフに Playwright が入ることはない。

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
npx playwright install chromium
```

## 使い方

```ts
import { chromium } from "playwright";
import { PlaywrightAdapter } from "@hsblabs/scrape-kdl-playwright";

const browser = await chromium.launch({ headless: true });
const adapter = new PlaywrightAdapter(browser);
try {
  const result = await compiled.program.extract(
    { id: "123" },
    { browser: adapter, allowJavaScript: true },
  );
  console.log(result.value);
} finally {
  await adapter.close();
  await browser.close();
}
```

`allowJavaScript: true` が必要なのは、プログラムが `evaluate-js` ノードを持つ場合だけである。
必要としないプログラムには設定しない。

## 所有権

`Browser` を所有するのは利用者である。
アダプタが所有するのは、自身が作った隔離コンテキストだけである。

`adapter.close()` メソッドはアダプタのコンテキストを閉じる。
利用者のブラウザを閉じることはない。
例のとおり、ブラウザは自分で閉じる。

そのため、一つのブラウザを複数のアダプタで使うことも、プロセスの生存期間を通じて一つのブラウザを保つこともできる。

## 分離

`navigate` の呼び出しごとに、アダプタは次の操作をこの順で行う。

1. 直前のコンテキストを閉じる。
2. 新しいコンテキストを作る。
3. 明示的なセッションのヘッダ、クッキー、User-Agent を設定する。
4. 新しいページへ遷移する。

そのため、ある抽出のクッキー、ストレージの値、ページへの変更、失敗した操作が、次の抽出へ持ち越されることはない。

## 操作の対応づけ

- 可搬セレクタは、ドキュメントまたは現在の要素のスコープで Playwright のロケータになる。
- テキストの読み取りは子孫の `textContent` を使う。HTML の読み取りは `innerHTML` を使う。属性は DOM の属性値を返す。
- ワークフローステップの `wait`、`click`、`fill`、`press`、`scroll`、および設定された network-idle の操作は、ソース順に実行される。
- `scope="document"` では JavaScript 関数に引数が渡らない。`scope="current"` では現在の DOM 要素が渡る。
- JavaScript の結果はアダプタの境界を越える。その後、コアランタイムが JSON 互換性と宣言された `returns` 型を検査する。

アダプタは `match="first"` と `match="one"` のためのクエリ数の制限にも対応する。
一致した要素の全体を作ることはしない。

## タイムアウトとキャンセル

アダプタは各操作を、公開されたタイムアウトと `AbortSignal` との競争に置く。

ブラウザ内で継続し得る操作では、タイムアウトまたはキャンセルがまず隔離コンテキストを閉じる。
ランタイムは、この後始末が完了するまでアダプタのリースを保持する。
そのため、リースの解放後に操作が続くことはない。
以後の抽出は新しいコンテキストを作り、正しく動作する。

## 対応ブラウザ

バージョン 1 でブロッキング対象となるのは Chromium である。
定期実行のブラウザワークフローは Firefox と WebKit の結果も報告するが、こちらは非ブロッキングの扱いである。

実運用の抽出には Chromium を使う。
他のブラウザの対応を格上げするには、互換性に関する別の意思決定が必要になる。

## 次に読むもの

- [ブラウザモード](../guides/browser-mode.md)：ワークフローステップと JavaScript の規則。
- [セキュリティと責任ある利用](../guides/security-and-responsible-use.md)：コンテキストの分離とネットワークの制御。
