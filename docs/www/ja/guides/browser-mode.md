---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: ブラウザモード
description: Scraping KDL のブラウザモード。アダプタ契約、ワークフローステップ、JavaScript の opt-in、抽出のリース、各アダプタに共通する部分。
hsblabs:
  sidebar:
    order: 25
---

ブラウザモードはケイパビリティであり、ページ取得の別手段ではない。
`mode="browser"` のプログラムには、利用者が渡すアダプタが必要になる。
コアモジュールは Playwright、Puppeteer、go-rod、chromedp のいずれにも依存しない。
アダプタを渡さなければ、抽出は遷移の前に `E_BROWSER_RUNTIME_MISSING` で失敗する。

このページは、各アダプタに共通する契約を扱う。
インストールとライフサイクルは [Playwright アダプタ](../npm/playwright.md) または [go-rod アダプタ](../golang/rod.md) を参照。

## 必要になる場面

ブラウザモードを使うのは、静的 DOM では得られない状態に限る。

- 読み込み後にスクリプトがコンテンツを書き込む
- クリック、入力、スクロールのあとで初めてコンテンツが現れる
- データがマークアップではなくページのメモリ上にある

それ以外は HTTP モードを使う。
そのほうが速く、Chromium を必要とせず、攻撃面も小さい。

## 実行順序

```text
capability and output preflight
  -> input resolution and URL expansion
  -> session policy
  -> optional lease acquisition
  -> navigation
  -> workflow steps, in source order
  -> extraction from the live DOM
  -> validation of the JavaScript results and the transforms
  -> lease release, after a success or a failure
```

ランタイムは、リースの取得と遷移よりも先に、外部トランスフォームの有無、ワークフローステップの種類とセレクタ、出力側の可搬セレクタ、出力メンバの種類、値ソースの種類を検査する。
そのため、誤りを含むプログラムがブラウザを起動することはない。

## ワークフロー

ワークフローは遷移のあと、抽出の前に実行される。
ステップはソース順に実行する。

```kdl
source "html" {
  fetch mode="browser" url="https://example.com/race/{race_id}"
  workflow {
    wait-for ".content" state="visible" timeout-ms=5000
    click ".load-more" timeout-ms=3000
    wait-for-network-idle idle-ms=500 timeout-ms=5000
  }
}
```

| ステップ | 役割 |
| --- | --- |
| `wait-for selector [state] [timeout-ms]` | 状態を待つ。`attached`、`visible`、`hidden`、`detached` のいずれか。既定は `visible`。 |
| `click selector [timeout-ms]` | 要素をクリックする。 |
| `fill selector value [timeout-ms]` | 要素に値を入れる。 |
| `press selector key [timeout-ms]` | 要素にキーを送る。 |
| `scroll x y` | ウィンドウをスクロールする。二つの数値は CSS ピクセル。 |
| `wait-for-network-idle [idle-ms] [timeout-ms]` | 追跡中の HTTP リクエストが `idle-ms` のあいだ 1 件もない状態まで待つ。既定は 500 ミリ秒。 |
| `evaluate-js script [timeout-ms]` | 関数を実行する。結果は捨てられる。 |

ワークフローステップはブラウザモードでのみ使える。
HTTP モードでは、コンパイラが `workflow` ノードを `E_BROWSER_CAPABILITY_REQUIRED` として拒否する。

`timeout-ms` と `idle-ms` の値は 1 から 9,223,372,036,854 ミリ秒の範囲でなければならない。
タイムアウトの超過は抽出エラーになる。
WebSocket 接続と EventSource 接続は、`wait-for-network-idle` の追跡対象リクエストに含まれない。

## JavaScript

JavaScript は既定で無効である。
明示的な opt-in が必要になる。
Go では `AllowJavaScript: true`、TypeScript では `allowJavaScript: true` である。
opt-in なしでは、JavaScript を含むプログラムは遷移の前に `E_JAVASCRIPT_DISABLED` で失敗する。

```kdl
field "race" type="object?" {
  evaluate-js #"""
    () => window.__INITIAL_STATE__?.race ?? null
    """# scope="document" returns="object?" timeout-ms=3000
}
```

規則は厳格である。

- スクリプトは呼び出し可能な関数を返さなければならない。async 関数も使える。
- `scope` プロパティは `document` か `current` である。`document` では関数に引数が渡らない。`current` では現在の要素が第一引数として渡る。
- `returns` プロパティは生の結果の型を宣言する。
- 結果は JSON 互換でなければならない。`null`、真偽値、文字列、有限の数値、配列、文字列キーのプレーンオブジェクトである。
- `undefined`、`NaN`、無限大、bigint、symbol、関数、DOM ノード、ランタイムのハンドル、循環オブジェクト、`Map`、`Set`、`Date` は禁じられている。これらは `E_JAVASCRIPT_RESULT_TYPE` で失敗する。

`evaluate-js` は、仕様側の信頼されたコードとして扱うこと。
ページの全権限で実行される。
また、可搬な挙動からの意図的な離脱でもあり、オフラインスナップショットでは再現できない。

## アダプタ契約

アダプタは小さなインターフェースを実装する。
Go のインターフェースは次の操作を持つ。

```go
type BrowserAdapter interface {
    Navigate(context.Context, string, BrowserNavigateOptions) error
    WaitFor(context.Context, string, string, time.Duration) error
    Click(context.Context, string, time.Duration) error
    Fill(context.Context, string, string, time.Duration) error
    Press(context.Context, string, string, time.Duration) error
    Scroll(context.Context, float64, float64) error
    WaitForNetworkIdle(context.Context, time.Duration, time.Duration) error
    Evaluate(context.Context, string, BrowserEvaluateOptions) (any, error)
    QueryAll(context.Context, BrowserElement, string) ([]BrowserElement, error)
    Text(context.Context, BrowserElement) (string, error)
    HTML(context.Context, BrowserElement) (string, error)
    Attribute(context.Context, BrowserElement, string) (string, bool, error)
}
```

TypeScript の契約は同じ操作を、Promise、ミリ秒単位のタイムアウトフィールド、任意の `AbortSignal` とともに持つ。

`BrowserElement` は不透明なハンドルである。
Playwright アダプタは Locator や ElementHandle を保持できる。
go-rod アダプタは `*rod.Element` を保持できる。
コアがハンドルの中身を見ることはない。

プログラムが必要とする操作をアダプタが持たない場合、検証は遷移の前に `E_BROWSER_CAPABILITY_MISSING` で失敗する。

## リース

可変なページを一つ制御するアダプタは、リースを実装しなければならない。

```go
type BrowserAdapterLease interface {
    Acquire(context.Context) (release func(), err error)
}
```

ランタイムは、遷移、ワークフロー、各出力の読み取りのあいだリースを保持する。
そのため、並行する二つの抽出の操作が一つのページ上で交錯することがない。

リースは並列性を与えるものではない。
並列実行には、複数のページか複数のアダプタを使う。

## クエリのスコープ

`QueryAll` に nil の要素を渡すと、ドキュメント全体がスコープになる。
`evaluate-js` では、スコープ `document` が nil スコープ、スコープ `current` がフィールドまたはコレクション行で選択された要素になる。

アダプタは個数を絞ったクエリも実装できる。
Go では `BrowserAdapterQueryLimit`、TypeScript では `queryLimit` である。
ランタイムはこれを `match="first"` と `match="one"` で使う。
これらはハンドルが 1 個か 2 個あれば足りる場面である。
この機能を持たないアダプタでは `QueryAll` を使う。

## セッションと URL ポリシー

`session policy="none"` の場合、ランタイムは `Navigate` に明示的なセッションを渡さない。
ブラウザコンテキストが既に持っているクッキー、ストレージ、認証を消去することはしない。
状態を持たない実行が必要なら、隔離されたコンテキストを渡すこと。

`Options.URLPolicy` フックは、リースの取得と遷移の前に実行される。
拒否された場合は `E_URL_POLICY` になり、ブラウザは使われない。

このポリシーが制御するのは最初の遷移先だけである。
ブラウザ内のリダイレクト、サブリソース、Service Worker、ページ自身が開始するリクエストは、このフックの外にある。
これらはブラウザコンテキストか、ホスト側のネットワークポリシーで制御する。

## 次に読むもの

- [Playwright アダプタ](../npm/playwright.md)：TypeScript と Node.js 向けの公式アダプタ。
- [go-rod アダプタ](../golang/rod.md)：Go 向けの公式アダプタ。
- [オフラインスナップショット](./offline-snapshots.md)：ブラウザなしでブラウザモードのプログラムを試す方法。
