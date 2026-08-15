---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: go-rod アダプタ
description: Go 向けの公式 go-rod ブラウザアダプタ。独立したモジュール、ページの所有権、抽出を直列化するリース、scrape-kdl-rod コマンドライン。
hsblabs:
  sidebar:
    order: 52
---

go-rod の統合は、独立した入れ子の Go モジュールである。

```text
adapters/rod/
```

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

メインモジュールはブラウザライブラリに依存しない。
アダプタは利用者が明示的に選ぶ。
そのため、Chromium のインストール、起動オプション、サンドボックス、ネットワークポリシー、プロセスのライフサイクルを利用者が制御できる。

## ライフサイクル

| 関数 | 挙動 |
| --- | --- |
| `rodadapter.New(page)` | 利用者が所有するページを使う。 |
| `rodadapter.NewBrowser(browser)` | ページを一つ作り、それを所有する。 |
| `Adapter.Close` | アダプタが所有するページを閉じる。それ以外は閉じない。 |

アダプタが、利用者の所有する `*rod.Browser` を閉じることはない。

一つのアダプタは、一つの可変なブラウザページを表す。
アダプタは `BrowserAdapterLease` を実装する。
そのためリースは、遷移、ワークフロー、抽出という一連の流れを、並行する呼び出しのあいだで直列化する。

並列に抽出するには、作業の系列ごとに別のアダプタと別のページを使う。

## JavaScript

コアは `AllowJavaScript` が `true` になるまで JavaScript を拒否する。

アダプタは、スコープ `document` のスクリプトを `Page.Evaluate` で実行する。
スコープ `current` のスクリプトは `Element.Evaluate` で実行し、KDL 側の関数に現在の要素を渡す。

## コマンドライン

このモジュールは独自のバイナリを持つ。
抽出器を一つコンパイルし、ブラウザモードで実行する。

```bash
go install github.com/hsblabs/scrape-kdl/adapters/rod/cmd/scrape-kdl-rod@v1.0.4
```

```bash
scrape-kdl-rod --spec extractor.kdl --input race_id=202401010101 --json
scrape-kdl-rod --spec extractor.kdl --session-file session.json -o result.json
```

| オプション | 役割 |
| --- | --- |
| `--spec FILE` | KDL ソース。この CLI はソースを常にこのオプションで受け取る。 |
| `--input NAME=VALUE` | 実行時の入力。複数指定するときは繰り返す。 |
| `--session-file FILE\|-` | コア CLI と同じ JSON のセッションスキーマ。 |
| `--timeout` | 各操作のタイムアウト。 |
| `--user-agent` | User-Agent。 |
| `--headless` | ブラウザのモード。 |
| `--allow-js` | JavaScript の明示的な opt-in。 |
| `--allow-private-hosts` | 最初の対象に対する既定の制限を無効にする。 |
| `--json` | 成功でも失敗でも JSON ドキュメントを一つ出力する。 |
| `-o`、`--out FILE\|-` | 抽出の結果そのもの。 |

この CLI は `--header` と `--cookie` フラグを拒否し、その値を書き出さない。
標準入力は `--session-file -` のために予約されている。

## 出力と終了ステータス

`--json` を使わない場合、成功時には整形された結果そのものが標準出力または指定のファイルへ書かれる。
人が読むための警告と失敗は標準エラーへ書かれる。

`--json` を使う場合、標準出力には次のいずれか一つのドキュメントが出る。

- 抽出に成功したとき：`{"ok": true, "result": {...}}`
- コンパイル、実行、入出力、使い方のいずれかに失敗したとき：`{"ok": false, "error": {...}}`
- `--version --json` のとき：`{"version": "...", "commit": "...", "built": "..."}`

`--json` と `--out FILE` は併用できない。
`--out -` を使うか、`--out` を使わない。

終了ステータスは、成功が 0、処理の失敗が 1、使い方の誤りが 2、`SIGINT` が 130、`SIGTERM` が 143 である。
二つのシグナルは、終了の前に実行中のコンテキストをキャンセルする。

## セッションと URL ポリシー

セッションのヘッダとクッキー、User-Agent、実行時の入力、タイムアウト、JavaScript の opt-in は、ライブラリと同じ公開契約 `scrapekdl.Options` を使う。

この CLI は既定で、最初の遷移先に `PublicInternetURLPolicy` を適用する。
スキーム、資格情報、そして IANA の special-purpose レジストリがグローバル到達可能と示していないアドレスを拒否する。
`--allow-private-hosts` オプションはこれを無効にする。

この最初の検査は、ブラウザに対するネットワークサンドボックスではない。
Chromium 内部のリダイレクト、サブリソース、Service Worker、ページ自身が開始するリクエストは、このフックの外にある。
実運用のホストは、必要な egress ポリシーを、ブラウザコンテキスト、プロセス、コンテナ、ネットワークのいずれかの境界で適用する必要がある。

## 検証

契約テストはローカルのスタブを使い、go-rod をダウンロードしない。

```bash
make test-rod-contract
```

実際の依存を伴うビルドとそのテストは次のコマンドを使う。

```bash
make test-rod
```

Chromium を使う E2E スイートは次のコマンドを使う。

```bash
make test-rod-e2e
```

E2E スイートには Chromium と互換のランタイムが必要である。
コアのテストと契約テストには不要である。

## 次に読むもの

- [ブラウザモード](../guides/browser-mode.md)：ワークフローステップと JavaScript の規則。
- [Go でのコンパイルと抽出](./compile-and-extract.md)：実行のオプション。
