---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: 診断
description: Scraping KDL の診断の読み方。安定した診断コード、決定的な順序、重大度、警告、そして頻出するコンパイルエラーと実行時エラー。
hsblabs:
  sidebar:
    order: 27
---

診断は、何が誤っているか、どこにあるか、出力のどの部分に影響するかを伝える。
診断コードは公開された互換性の表面である。
一つのコードはリリースをまたいで同じ意味を保つ。
そのため、コードを検査するテストやアラートを書ける。

全一覧は [diagnostics.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/diagnostics.md) にある。

## 形式

CLI は診断ごとに 1 行を書く。

```text
extractor.kdl:9:5: error E_SELECTOR_UNSUPPORTED: selector byte 9: unsupported pseudo-class "has" [output.title.selection]
```

行は五つの部分からなる。

| 部分 | 意味 |
| --- | --- |
| `extractor.kdl:9:5` | ファイル、行、桁。番号は 1 から始まる。 |
| `error` | 重大度。 |
| `E_SELECTOR_UNSUPPORTED` | 安定したコード。 |
| コードの後ろのテキスト | 人が読むためのメッセージ。安定ではない。 |
| `[output.title.selection]` | エラーが影響する出力上のパス。 |

検査すべきはコードであり、メッセージではない。
コードとその条件が規範である。
メッセージは規範ではなく、変わり得る。

## 機械可読な形式

`--json` オプションは JSON ドキュメントを一つ出力する。

```bash
scrape-kdl validate ./extractor.kdl --json
```

```json
{
  "ok": false,
  "diagnostics": [
    {
      "code": "E_SELECTOR_UNSUPPORTED",
      "severity": "error",
      "message": "selector byte 13: unsupported pseudo-class \"has\"",
      "span": {
        "file": "extractor.kdl",
        "start": { "offset": 197, "line": 7, "column": 5 },
        "end": { "offset": 219, "line": 7, "column": 27 }
      },
      "path": "output.title.selection"
    }
  ]
}
```

`span` は Validated IR と同じ定義を使う。
行と桁は 1 から始まる。
`offset` は 0 起点の UTF-8 バイトオフセットである。
終端位置は含まない。
出力メンバに影響しない診断では `path` が現れない。

エンベロープはコマンドごとに異なる。

- `validate`：`{"ok": boolean, "diagnostics": [...]}`
- `compile`：`{"ok": true, "diagnostics": [...], "ir": {...}}` または `{"ok": false, "diagnostics": [...]}`
- `extract`：`{"ok": true, "result": {...}}` または `{"ok": false, "error": {...}}`

## 順序

コンパイラは静的診断を次の順序で並べる。

1. import の深さ優先の解決順序
2. 位置が同じ場合はファイルパスの辞書順
3. ソース上の開始オフセット
4. コードの辞書順

ランタイムは警告を実行の順序で並べる。

この順序は決定的である。
同じプログラムを同じ入力で 2 回実行すれば、同じ並びが得られる。
そのため、診断の出力全体をゴールデンファイルと比較できる。

## 重大度

重大度は二つある。

- **`error`**：処理が止まる。コンパイラは IR を作らず、ランタイムは結果を返さない。
- **`warning`**：抽出は続く。結果の `warnings` 配列に警告が入る。

警告はしばしば `partial` フラグを `true` にする。
これにより、結果が完全でないことがわかる。

## 頻出するコンパイルエラー

| コード | 原因 | 対処 |
| --- | --- | --- |
| `E_SELECTOR_UNSUPPORTED` | セレクタが可搬プロファイルの外にある。`:has()` など。 | [セレクタ](./selectors.md) を参照。 |
| `E_TRANSFORM_TYPE_MISMATCH` | あるトランスフォームの出力型が、次のトランスフォームの入力型と一致しない。 | `parse-int` などの変換を追加する。 |
| `E_TRANSFORM_UNKNOWN` | その名前が組み込みでも、ローカルのトランスフォームでも、修飾された import でもない。 | 綴りと import の別名を確認する。 |
| `E_BROWSER_CAPABILITY_REQUIRED` | ブラウザ専用のノードが HTTP モードのプログラムにある。 | モードを `browser` に変えるか、そのノードを除く。 |
| `E_LANGUAGE_VERSION_UNSUPPORTED` | `language-version` の値は形式として正しいが、対応していない。 | `2026-07-15` を使う。 |
| `E_DUPLICATE_PROPERTY` | 一つのノードが同じプロパティを 2 回持つ。 | 片方を除く。コンパイラが一方を選ぶことはない。 |
| `E_IMPORT_CYCLE` | import のグラフに循環がある。 | 共有するトランスフォームを、import を持たないモジュールに切り出す。 |
| `E_REMOTE_IMPORT_UNSUPPORTED` | import のパスが相対パスでない。 | そのモジュールを自分のリポジトリへ複製する。 |

## 頻出する実行時エラー

| コード | 原因 |
| --- | --- |
| `E_REQUIRED_VALUE_MISSING` | `required=#true` のフィールドが値を得られなかった。 |
| `E_SELECTOR_CARDINALITY` | `match="one"` のセレクタが 2 個以上の要素に一致した。 |
| `E_URL_POLICY` | URL ポリシーが最初の対象またはリダイレクトを拒否した。 |
| `E_HTTP_STATUS` | レスポンスのステータスが 200 から 299 の範囲外である。 |
| `E_HTTP_BODY_TOO_LARGE` | レスポンスがボディ上限より大きい。 |
| `E_JAVASCRIPT_DISABLED` | プログラムが JavaScript を含むが、opt-in がない。 |
| `E_SNAPSHOT_UNSUPPORTED` | ワークフローまたは JavaScript を含むプログラムに、スナップショット実行が要求された。 |
| `E_BROWSER_RUNTIME_MISSING` | ブラウザモードのプログラムにアダプタがない。 |
| `E_EXECUTION_CANCELED` | コンテキストまたは `AbortSignal` が実行を止めた。 |

`E_JAVASCRIPT_DISABLED`、`E_BROWSER_RUNTIME_MISSING`、`E_SNAPSHOT_UNSUPPORTED` は取得より前に起きる。
そのため、設定の誤りが通信を発生させることはない。

## 警告

| コード | 意味 |
| --- | --- |
| `W_ROW_SKIPPED` | `on-row-error="skip"` ポリシーがコレクションの行を捨てた。 |
| `W_ERROR_RECOVERED` | `on-error="warn"` ポリシーがエラーから回復した。 |
| `W_PARTIAL_EXTRACTION` | 部分的な結果を示す要約の警告。 |
| `W_JAVASCRIPT_PRESENT` | 静的検査が信頼コードのケイパビリティを検出した。 |

## 終了ステータス

| ステータス | 意味 |
| --- | --- |
| 0 | 成功。 |
| 1 | 検証、コンパイル、抽出、または入出力の失敗。 |
| 2 | コマンドまたはフラグの使い方の誤り。 |
| 130 | `SIGINT` がプロセスを止めた。 |
| 143 | `SIGTERM` がプロセスを止めた。 |

自動処理では、終了ステータスと `--json` エンベロープの `ok` フィールドの両方を検査する。

## 次に読むもの

- [パターン](./patterns.md)：頻出するエラーを避ける書き方。
- [CLI](../cli/index.md)：コマンドの契約の全体。
