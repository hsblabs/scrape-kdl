---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Reference
title: CLI
description: scrape-kdl コマンドラインの契約。四つのコマンド、オプション、標準ストリーム、JSON エンベロープ、終了ステータス、秘密情報の扱い。
hsblabs:
  sidebar:
    order: 30
---

Go のバイナリ `scrape-kdl` は四つのコマンドを持つ。
`validate`、`compile`、`extract`、`version` である。
バージョン 1 で提供されるコマンドライン配布物はこれだけである。
TypeScript パッケージに CLI はない。

go-rod アダプタは、ブラウザモード用に独自のバイナリ `scrape-kdl-rod` を持つ。
[go-rod アダプタ](../golang/rod.md) を参照。

## コマンド

```bash
scrape-kdl validate extractor.kdl
scrape-kdl compile extractor.kdl --out extractor.ir.json
scrape-kdl extract extractor.kdl --input id=42
scrape-kdl version
```

各コマンドとルートは `-h` と `--help` を受け付ける。
ヘルプは標準出力へ書かれ、終了ステータスは 0 になる。
`scrape-kdl help <command>` も同じである。

コマンドや引数を省略すると、短い使用法が標準エラーへ書かれ、終了ステータス 2 になる。
CLI が不足した入力を尋ねてくることはない。

## `validate`

```text
scrape-kdl validate <file.kdl|-> [--json]
```

ドキュメントを解析し、シンボルを解決し、型を検査し、ケイパビリティを算出する。
ネットワーク活動もブラウザ活動も発生しない。

## `compile`

```text
scrape-kdl compile <file.kdl|-> [--json] [-o file.json|-]
```

| オプション | 役割 |
| --- | --- |
| `-o`、`--out PATH` | IR そのものをパスへ書く。`-` で標準出力。 |
| `--json` | JSON ドキュメントを一つ標準出力へ書く。 |
| `--emit-ir` | 互換のための綴り。`compile` は常に IR を作る。 |

## `extract`

```text
scrape-kdl extract <file.kdl|-> [options]
```

| オプション | 役割 |
| --- | --- |
| `--input NAME=VALUE` | 実行時の入力。複数指定するときは繰り返す。 |
| `--html PATH` | パスから読んだデコード済み HTML を使う。`-` で標準入力。 |
| `--session-file PATH` | JSON ファイルからヘッダとクッキーを読む。`-` で標準入力。 |
| `--session` | 明示的な空のセッションを渡す。 |
| `--allow-private-hosts` | グローバル到達可能でない対象を許可し、プロキシの通常の挙動を戻す。 |
| `--timeout DURATION` | HTTP リクエストのタイムアウト。既定は 30 秒。 |
| `--max-body BYTES` | レスポンスボディの最大サイズ。既定は 33554432。 |
| `--user-agent VALUE` | HTTP リクエストの User-Agent。 |
| `--json` | JSON ドキュメントを一つ標準出力へ書く。 |
| `-o`、`--out PATH` | 結果そのものをパスへ書く。`-` で標準出力。 |

`--html` を使うと、ランタイムは取得を行わない。
URL 展開も、URL ポリシーも、セッションも、リクエストもない。
そのため宣言済みの input も不要である。
[オフラインスナップショット](../guides/offline-snapshots.md) を参照。

## ストリーム

主たる結果は標準出力へ書かれる。
診断、警告、書き込みの確認、エラーは標準エラーへ書かれる。

`compile` と `extract` は、既定で IR または結果そのものを整形された JSON として書く。
リダイレクトやパイプを伴う呼び出しでは、色のエスケープシーケンス、進捗のアニメーション、復帰文字による再描画、対話的な質問のいずれも起きない。

## 標準入力

ストリームが一つに定まる位置では、`-` が標準入力を指す。

```bash
cat extractor.kdl | scrape-kdl validate -
cat extractor.kdl | scrape-kdl compile - --out -
cat page.html | scrape-kdl extract extractor.kdl --html -
cat session.json | scrape-kdl extract extractor.kdl --session-file -
```

1 回の呼び出しで標準入力を渡せるのは、KDL ソース、`--html -`、`--session-file -` のいずれか一つである。
二つには渡せない。
曖昧な組み合わせは使用法の誤りになり、CLI が入力を待ち続けることはない。
`--out -` は標準出力を選ぶオプションであり、標準入力は使わない。

## JSON エンベロープ

`--json` オプションは、標準出力へちょうど一つの JSON ドキュメントを書く。
成功したとき、処理に失敗したとき、そして CLI がフラグを認識した後に使用法の誤りが起きたときに書かれる。
人が読むための診断は標準エラーに残る。

| コマンド | エンベロープ |
| --- | --- |
| `validate` | `{"ok": boolean, "diagnostics": [...]}` |
| `compile` | `{"ok": true, "diagnostics": [...], "ir": {...}}` または `{"ok": false, "diagnostics": [...]}` |
| `extract` | `{"ok": true, "result": {...}}` または `{"ok": false, "error": {...}}` |
| `version` | `{"version": "...", "commit": "...", "built": "..."}` |

`--json` と `--out FILE` は併用できない。
`--out -` を使うか、`--out` を使わない。

自動処理では `--json` を使い、`ok` フィールドと終了ステータスの両方を検査する。

## 終了ステータス

| ステータス | 意味 |
| --- | --- |
| 0 | 成功。 |
| 1 | 検証、コンパイル、抽出、入出力、その他の処理の失敗。 |
| 2 | コマンドまたはフラグの使い方の誤り。 |
| 130 | `SIGINT` がプロセスを止めた。 |
| 143 | `SIGTERM` がプロセスを止めた。 |

`SIGINT` と `SIGTERM` は、実行中の抽出のコンテキストをキャンセルする。
HTTP リクエストとランタイムの処理は、プロセスがシグナルのステータスで終わる前にキャンセルを検知する。
中断された抽出について、CLI が途中までの主たるドキュメントを書くことはない。

## ネットワークポリシー

`extract` は既定で、IANA の special-purpose レジストリがグローバル到達可能と示していないアドレスを拒否する。
ここにはループバック、プライベート、リンクローカル、CGN、ドキュメンテーション、ベンチマーク、マルチキャスト、未指定、予約の各範囲が含まれる。
CLI は宣言されたホストと、接続時に選ばれたアドレスの両方を検査し、リダイレクトのたびに検査し直す。
拒否は `E_URL_POLICY` になる。

保護付きの HTTP クライアントは直接接続を行い、環境のプロキシ設定を使わない。
プロキシを経由すると宛先の解決がプロキシ側で行われ、選ばれたアドレスをクライアントが検査できなくなるためである。

ローカル、イントラネット、明示的にプロキシを経由する抽出には `--allow-private-hosts` を使う。
`--html` によるオフライン実行はネットワーク活動を発生させないため、このオプションの影響を受けない。

## 秘密情報

CLI がヘッダとクッキーを受け取るのは、`--session-file FILE` または `--session-file -` からだけである。

```json
{
  "headers": {"Authorization": ["Bearer example"]},
  "cookies": [{"name": "session", "value": "example"}]
}
```

`--header` と `--cookie` フラグは、バージョン 0.5 の契約境界で削除された。
コマンド引数はシェルの履歴に残り得るし、プロセス一覧から見え得るからである。
繰り返し指定していたフラグは、`headers` 配列または `cookies` の要素として書く。
CLI は削除されたフラグを拒否し、その値を書き出さない。

## 互換性

ヘルプ、ストリーム、JSON、終了ステータス、シグナル、秘密情報の入力に関する契約は、バージョン 1 で凍結されている。
変更はセマンティックバージョニングに従い、ブラックボックステストと互換性に関する記述を必要とする。

## 次に読むもの

- [パターン](../guides/patterns.md)：`--json` と `jq` を使うループ。
- [診断](../guides/diagnostics.md)：失敗した出力の読み方。
