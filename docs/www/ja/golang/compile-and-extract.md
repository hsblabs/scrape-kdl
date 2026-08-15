---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Go でのコンパイルと抽出
description: Go の実行 API。コンパイルオプション、注入するローダ、実行オプション、三つの抽出エントリポイント、URL ポリシー、結果の型付きデコード。
hsblabs:
  sidebar:
    order: 51
---

このページは `github.com/hsblabs/scrape-kdl` モジュールにおける、コンパイルと実行の API を示す。

## コンパイル

```go
source := scrapekdl.Source{
    Path: "extractor.kdl",
    Data: data,
}

program, diagnostics, err := scrapekdl.Compile(ctx, source, scrapekdl.CompileOptions{})
if err != nil {
    return err
}
if diagnostics.HasErrors() {
    return fmt.Errorf("compilation failed: %v", diagnostics)
}
```

`Path` フィールドは、診断、ソース位置、import の解決、IR 内のファイル識別に使う論理的な識別子である。
OS 上のファイル名である必要はない。

`Diagnostics` 型は決定的な順序を持つ。
警告も含まれる。
エラーの有無は `HasErrors()` で調べる。

## import

import を含むソースにはローダが必要である。

```go
options := scrapekdl.CompileOptions{
    Loader: func(ctx context.Context, path string) ([]byte, error) {
        if !allowed[path] {
            return nil, fmt.Errorf("refused: %s", path)
        }
        return os.ReadFile(path)
    },
}
```

コンパイラは各 import を、それを import しているソースからの相対で字句的に解決してから、ローダを呼ぶ。
ローダが返すのはバイト列だけである。
解析、検証、循環の検出、ハッシュ、決定的な順序づけはコンパイラが行う。

import を含むソースをローダなしでコンパイルすると、IR を返す前に失敗する。
ローダは権限の境界である。
パスを限定し、キャンセルに従い、ソースの内容や資格情報をエラーに含めないこと。

アプリケーションが持つファイルシステムには `CompileFS(ctx, fsys, path)` を使う。
入れ子の import を同じファイルシステム内で解決し、親への字句的な脱出を拒否する。
`fs.ReadFile` の前後でキャンセルを検査する。
`fs.FS` インターフェースは、進行中の読み取りを中断できない。

## 三つの抽出エントリポイント

```go
result, err := program.Extract(ctx, inputs, options)
result, err := program.ExtractHTML(ctx, html, options)
result, err := program.ExtractSnapshot(ctx, html, options)
```

| メソッド | 取得 | 受け付けるモード |
| --- | --- | --- |
| `Extract` | ソースのモードに従う。 | すべてのモード。ブラウザモードのプログラムには `Options.Browser` が必要。 |
| `ExtractHTML` | なし。 | HTTP モードのみ。 |
| `ExtractSnapshot` | なし。 | プログラムがスナップショットに適格なら、すべてのモード。 |

[オフラインスナップショット](../guides/offline-snapshots.md) を参照。

## 実行オプション

```go
type Options struct {
    Browser            BrowserAdapter
    AllowJavaScript    bool
    HTTPClient         *http.Client
    Session            *Session
    ExternalTransforms map[string]ExternalTransform
    CharsetDecoder     CharsetDecoder
    RequestTimeout     time.Duration
    MaxResponseBytes   int64
    UserAgent          string
    URLPolicy          URLPolicy
}
```

`Options` の値は一つの抽出を設定する。
可変な状態はその抽出の内側にとどまる。
そのため、一つの不変プログラムを、異なるオプションで同時に何度でも実行できる。

`AllowJavaScript` フィールドは既定で無効である。
`evaluate-js` ノードを持つプログラムは、遷移の前に `E_JAVASCRIPT_DISABLED` で失敗する。

## 結果

```go
type Result struct {
    Value    map[string]any `json:"value"`
    Warnings []Warning      `json:"warnings"`
    Partial  bool           `json:"partial"`
}
```

型付きの値には `Decode` を使う。

```go
var output struct {
    Title string `json:"title"`
    Items []struct {
        Value uint8 `json:"value"`
    } `json:"items"`
}
if err := result.Decode(&output); err != nil {
    return err
}
```

`Warning` は `Code`、`Message`、任意の `Path`、任意の `Row` を持つ。
`Partial` フラグが `true` になるのは、ランタイムがエラーから回復したか、行を捨てた場合だけである。

失敗すると `*ExecutionError` が返る。
`Code`、`Message`、`Path`、`Cause` を持つ。

```go
var execErr *scrapekdl.ExecutionError
if errors.As(err, &execErr) && execErr.Code == "E_REQUIRED_VALUE_MISSING" {
    // handle the missing value
}
```

検査すべきは `Code` である。
コードは安定しており、メッセージは安定していない。
[診断](../guides/diagnostics.md) を参照。

## URL ポリシー

```go
options := scrapekdl.Options{
    URLPolicy:  scrapekdl.PublicInternetURLPolicy(),
    HTTPClient: scrapekdl.NewPublicInternetHTTPClient(),
}
```

この二つは併用する。
ポリシーは最初の対象と各リダイレクトを検査する。
保護付きのクライアントは接続時にアドレスを再解決し、再度検査する。
そのため DNS リバインディングでは検査を回避できない。

保護付きのクライアントは直接接続を行い、環境のプロキシ設定を使わない。
プロキシを経由すると宛先の解決がプロキシ側で行われ、選ばれたアドレスをクライアントが検査できなくなるためである。

ライブラリは、設定するまでポリシーを適用しない。
CLI は既定で両方を適用する。
[HTTP 実行](../guides/http-execution.md) を参照。

## 外部トランスフォーム

```go
options := scrapekdl.Options{
    ExternalTransforms: map[string]scrapekdl.ExternalTransform{
        "decrypt_payload": func(ctx context.Context, input any) (any, error) {
            return decrypt(input)
        },
    },
}
```

プログラムが必要とするシンボルがレジストリにない場合、検証は fetch より前に失敗する。
関数が結果を返した直後に、ランタイムはその結果を宣言された出力型に照らして検査する。

## キャンセル

ランタイムはコンテキストを、HTTP リクエスト、アダプタの操作、出力の走査へ伝播する。
コンテキストは、メモリ上での HTML の解析の前、各出力メンバの前、コレクションの各行の前に検査される。

キャンセルは `E_EXECUTION_CANCELED` になり、原因として `context.Canceled` または `context.DeadlineExceeded` を保持する。
フィールドポリシーでも行ポリシーでも回復できない。

## 互換性

```go
scrapekdl.SupportedLanguageVersions()
scrapekdl.SupportedIRVersions()
```

この二つの関数は、このビルドが受け付ける正確なバージョンを返す。

## 次に読むもの

- [go-rod アダプタ](./rod.md)：ブラウザモードのプログラムを実行する方法。
- [パターン](../guides/patterns.md)：複数ページにまたがるループ。
