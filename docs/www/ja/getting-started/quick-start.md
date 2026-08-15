---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: クイックスタート
description: 最初の Scraping KDL 抽出器を書き、保存済みの HTML ファイルに対して validate、compile、extract を実行する。この手順ではネットワークリクエストを送らない。
hsblabs:
  sidebar:
    order: 11
---

この手順は空のディレクトリから始まり、構造化された結果で終わる。
使うのは CLI だけである。
抽出器は保存済みの HTML ファイルに対して動作するため、**ネットワークリクエストは一切送らない**。
新しい抽出器は、実サービスに向ける前にこの形で試すのがよい。

`scrape-kdl` バイナリが必要になる。
[インストール](./installation.md) を参照。

## 1. HTML ファイルを保存する

次の内容を `page.html` として保存する。
見出しの中の空白は意図的に入れてある。
トランスフォームの働きを見せるためである。

```html
<!doctype html>
<html>
  <body>
    <h1>  Scraping   KDL Runtime  </h1>
    <ul class="items">
      <li><span class="value">1</span></li>
      <li><span class="value">2</span></li>
      <li><span class="value">3</span></li>
    </ul>
  </body>
</html>
```

## 2. 抽出器を書く

次の内容を `extractor.kdl` として保存する。

```kdl
extractor "basic-http" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/{id}"
  }

  input "id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }

  collection "items" min-items=1 {
    select "ul.items > li"
    field "value" type="u8" required=#true {
      select ".value" match="one"
      value "text"
      apply "trim"
      apply "parse-int" as="u8"
    }
  }
}
```

ドキュメントを上から順に読む。

- `version` はこのドキュメントの版を識別する。`language-version` は言語契約を選ぶ。値は `2026-07-15` でなければならない。どちらのプロパティも必須である。
- `source` はドキュメントの取得方法を宣言する。`mode="http"` は静的な HTTP ランタイムを選ぶ。ランタイムは宣言済みの input を `{id}` の位置に置く。
- `field "title"` ノードは `h1` を一つ選び、そのテキストを読み、余分な空白を取り除く。
- `collection "items"` ノードは、セレクタに一致する `li` ごとに行を作る。行数の下限も 1 に指定してある。
- `type="u8"` プロパティは実効的な制約である。ランタイムはテキストを 8 ビット符号なし整数として解釈する。値が大きすぎれば抽出エラーになる。ランタイムが値を切り詰めることはない。

## 3. 検証する

```bash
scrape-kdl validate ./extractor.kdl
```

```text
valid: ./extractor.kdl
```

検証は解析だけを行う。
ドキュメントを構文解析し、シンボルを解決し、型を検査し、ケイパビリティを算出する。
ソケットは開かない。
終了ステータスは、正しいドキュメントなら `0`、診断にエラーが含まれれば `1` である。

ここで意図的にエラーを起こしてみる。
title のセレクタを `h1:has(a)` に変えて、もう一度検証する。

```text
extractor.kdl:9:5: error E_SELECTOR_UNSUPPORTED: selector byte 9: unsupported pseudo-class "has" [output.title.selection]
```

疑似クラス `:has()` は可搬セレクタプロファイルの外にある。
そのためコンパイラはこれを拒否し、ソース位置と出力パスを返す。
このエラーはその場で得られる。
クロールの 20 ページ目で初めて分かるのではない。
先へ進む前にセレクタを `h1` に戻すこと。
詳細は [診断](../guides/diagnostics.md) を参照。

## 4. コンパイルする

```bash
scrape-kdl compile ./extractor.kdl --out ./extractor.ir.json
```

```text
wrote: ./extractor.ir.json
```

Validated IR は、コンパイラと各ランタイムのあいだの言語非依存な契約である。
`--out` を渡さない場合、CLI は IR を標準出力に書く。
まず次のフィールドを見るとよい。

```json
{
  "irVersion": "2026-07-15",
  "languageVersion": "2026-07-15",
  "capabilities": ["http.fetch"]
}
```

`capabilities` 配列は、このプログラムが必要とするケイパビリティの正確な集合を持つ。
このプログラムは HTTP で取得するだけである。
ブラウザワークフローや `evaluate-js` のフィールドを足せば、集合は大きくなる。
これにより、ホストはプログラムを実行する前に何を許可するかを判断できる。
詳細は [動作の仕組み](../guides/how-it-operates.md) を参照。

## 5. 保存済み HTML から抽出する

```bash
scrape-kdl extract ./extractor.kdl --html ./page.html
```

```json
{
  "value": {
    "items": [
      {
        "value": 1
      },
      {
        "value": 2
      },
      {
        "value": 3
      }
    ],
    "title": "Scraping KDL Runtime"
  },
  "warnings": [],
  "partial": false
}
```

見出しから余分な空白が消えている。
items の値は文字列ではなく数値になっている。
`partial: false` は、ランタイムがエラーから回復していないことを示す。

`--html` オプションは取得を行わない。
URL 展開も、URL ポリシーも、セッションも、HTTP リクエストもない。
そのため必須の `id` input を渡す必要もない。
詳細は [オフラインスナップショット](../guides/offline-snapshots.md) を参照。

## 6. 機械可読なエンベロープを得る

スクリプトから使う場合、`--json` オプションは成功フラグ付きのエンベロープに結果を入れる。

```bash
scrape-kdl extract ./extractor.kdl --html ./page.html --json
```

```json
{
  "ok": true,
  "result": {
    "value": {
      "items": [
        {
          "value": 1
        },
        {
          "value": 2
        },
        {
          "value": 3
        }
      ],
      "title": "Scraping KDL Runtime"
    },
    "warnings": [],
    "partial": false
  }
}
```

自動化された手順では、`ok` とプロセスの終了ステータスの両方を見ること。
詳細は [CLI](../cli/index.md) を参照。

## 実 URL に対して実行する

同じ抽出器は実 URL に対しても動作する。
`--html` の代わりに、宣言済みの input を渡す。

```bash
scrape-kdl extract ./extractor.kdl --input id=123
```

実サービスに対してこれを行う前に、[セキュリティと責任ある利用](../guides/security-and-responsible-use.md) を読むこと。
CLI は既定で、グローバルに到達可能でない宛先を拒否する。
セッションは `--session-file` からしか受け付けない。
これにより、認証情報がシェルの履歴に残らない。

## 次に読むもの

- [抽出器を書く](../guides/write-an-extractor.md)：フィールド、コレクション、input、エラーポリシー。
- [HTTP 実行](../guides/http-execution.md)：セッション、リダイレクト、各種上限、URL ポリシー。
- [TypeScript と Bun](../npm/index.md) または [Go](../golang/index.md)：同じプログラムをライブラリから実行する。
