---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Go
description: Scraping KDL の Go モジュール。コアモジュール、独立した go-rod アダプタモジュール、コアがブラウザライブラリを import しない理由、最初のコンパイル。
hsblabs:
  sidebar:
    order: 50
---

コアモジュールは、コンパイラ、診断、IR、HTTP ランタイム、オフラインスナップショットのランタイム、ブラウザアダプタのインターフェースを提供する。
Go 1.26 以降が必要である。

```bash
go get github.com/hsblabs/scrape-kdl@v1.0.4
```

## 二つのモジュール

go-rod アダプタは**独立したモジュール**である。
コアモジュール内のパッケージではない。

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

コアモジュールは go-rod を import してはならない。
これは好みではなく、プロジェクトの不変条件である。
ブラウザライブラリはアダプタに属する。

そのため、HTTP だけを使うアプリケーションの依存グラフに、Chromium、CDP、go-rod が入ることはない。
アダプタのモジュールを足すのは、ブラウザモードが必要になったときだけである。
[go-rod アダプタ](./rod.md) を参照。

## 最初のコンパイル

```go
package main

import (
    "context"
    "time"

    scrapekdl "github.com/hsblabs/scrape-kdl"
)

func main() {
    ctx := context.Background()
    program, diagnostics, err := scrapekdl.CompileFile(ctx, "extractor.kdl")
    if err != nil {
        panic(err)
    }
    if diagnostics.HasErrors() {
        panic(diagnostics)
    }

    result, err := program.Extract(ctx, map[string]any{"id": "123"}, scrapekdl.Options{
        RequestTimeout: 15 * time.Second,
    })
    if err != nil {
        panic(err)
    }

    var output struct {
        Title string `json:"title"`
    }
    if err := result.Decode(&output); err != nil {
        panic(err)
    }
}
```

コンパイルは三つの値を返す。
プログラム、順序づけられた診断、そして運用上のエラーである。
プログラムを使う前に `diagnostics.HasErrors()` を検査する。

運用上のエラーは診断とは別のものである。
キャンセル、ファイルシステムの失敗、注入したローダの失敗は運用上のエラーである。
これらの関数は `errors.Is` と `errors.As` のために原因を保持する。
[Go でのコンパイルと抽出](./compile-and-extract.md) を参照。

## コンパイルの関数

| 関数 | ソース |
| --- | --- |
| `Compile(ctx, Source, CompileOptions)` | メモリ上のソース。 |
| `CompileFile(ctx, path)` | OS 上のファイル。 |
| `CompileFS(ctx, fsys, path)` | アプリケーションが持つ `fs.FS`。 |

`Validate`、`ValidateFile`、`ValidateFS` は、診断だけを返す対応物である。

`CompileFS` と `ValidateFS` は、ルートと各 import を `io/fs` の正当なパスに限定し、親への字句的な脱出を拒否する。
それでも真の権限を決めるのは、渡された `fs.FS` である。
`os.DirFS` はディレクトリ外へのシンボリックリンクをたどり得る。
封じ込めが必要なら `os.Root.FS` を使う。

## プログラムのメタデータ

```go
metadata := program.Metadata()
metadata.Capabilities  // the exact capabilities that the program needs
metadata.LanguageVersion
metadata.IRVersion
metadata.Files         // each source file, with its SHA-256

descriptor := program.Descriptor()
descriptor.Source.FetchMode      // "http" or "browser"
descriptor.Source.SessionPolicy  // "none", "optional", or "required"
```

プログラムは不変であり、再利用できる実行計画を持つ。
ケイパビリティは、ホスト側でプログラムを許可するか拒否するかの判断に使う。
ディスクリプタは、IR 全体をデコードせずに取得方法を判断するために使う。

`Program.IRJSON()` は、交換やツール向けに Validated IR の全体を返す。

## 並行性

アダプタが文書化された所有権の契約に従うなら、プログラムは並行した抽出に対して安全である。
可変な状態は一つの抽出の内側にとどまる。

例外は、一つのページを制御するブラウザアダプタである。
このアダプタは `BrowserAdapterLease` を実装しなければならず、リースが操作の交錯を防ぐ。
真の並列実行には、複数のページか複数のアダプタを使う。

## コマンドライン

コアモジュールは CLI も含む。

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@v1.0.4
```

[CLI](../cli/index.md) を参照。

## 次に読むもの

- [Go でのコンパイルと抽出](./compile-and-extract.md)：実行 API の全体。
- [go-rod アダプタ](./rod.md)：公式アダプタによるブラウザモード。
