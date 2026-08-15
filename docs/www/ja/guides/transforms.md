---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: トランスフォーム
description: Scraping KDL の値パイプライン。組み込みレジストリ、宣言されたトランスフォーム、match テーブル、ホスト側の外部関数、RE2 正規表現プロファイル。
hsblabs:
  sidebar:
    order: 23
---

値ソースが返すのは文字列である。
トランスフォームは、その文字列を宣言された型の値にする。
`apply` ノードはソース順に実行する。
コンパイラは実行前に、並び全体の型を検査する。

規範レジストリは [builtins-v0.1.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.md) にある。
コンパイル済みの実例は [トランスフォームクックブック](https://github.com/hsblabs/scrape-kdl/blob/main/docs/cookbook.md) にある。

## パイプライン

```kdl
field "price" type="u32" required=#true {
  select ".price" match="one"
  value "text"
  apply "trim"
  apply "replace" old="," new=""
  apply "parse-int" as="u32"
}
```

各呼び出しは、直前の呼び出しの出力を受け取る。
ある呼び出しの出力型は、次の呼び出しの入力型と一致しなければならない。
不整合な並びは実行時ではなくコンパイル時に `E_TRANSFORM_TYPE_MISMATCH` になる。

最終出力はフィールドの `type` に代入可能でなければならない。
暗黙の変換はない。
`string` は `parse-int` を経ずに `u32` にはならない。

## 組み込みレジストリ

組み込みトランスフォームは四つの群からなる。

| 群 | 役割 | 例 |
| --- | --- | --- |
| 文字列 | テキストの形を変える。 | `trim`、`normalize-whitespace`、`replace`、`regex-capture`、`split` |
| 変換 | テキストから型付きの値を作る。 | `parse-int`、`parse-float`、`parse-bool`、`empty-to-null`、`coalesce` |
| URL | RFC 3986 に従って URL を読む、または解決する。 | `url-resolve`、`url-query`、`url-path`、`path-segment` |
| 検証 | 入力をそのまま返すか、エラーにする。 | `assert-matches`、`assert-enum`、`assert-min`、`assert-max` |

全一覧と各シグネチャはレジストリを参照。
組み込みには次の規則が共通して適用される。

- 組み込み名は予約されている。上書きはできない。
- 未知、重複、型の合わない呼び出しプロパティはエラーになる。
- 文字列の添字は Unicode スカラー値の添字である。UTF-8 バイトや UTF-16 コードユニットのオフセットではない。
- 数値の出力は有限であり、かつ対象型の範囲に収まっていなければならない。

`parse-int` は余分な空白を除去しない。
先に `trim` を適用すること。
入力全体を消費することも要求される。
そのため `12 kg` というテキストは抽出エラーになり、値 `12` にはならない。
この挙動は意図的である。
黙って一部だけ解釈すると、ページの変化が隠れてしまう。

## 正規表現

正規表現は RE2 構文を使う。
RE2 には後読み、後方参照、名前付きキャプチャグループ、条件式がない。
これらがないことが、実行時間の予測可能性の代償である。

`flags` プロパティで `i`、`m`、`s` を指定できる。
各フラグは 1 回だけ書ける。

置換文字列では、`$0` が一致全体、`$1` から `$99` が番号付きキャプチャである。
ドル記号そのものは `$$` と書く。

JavaScript の正規表現構文はこの言語の構文ではない。
`RegExp` で動くパターンが `E_REGEX_INVALID` になることはある。

## 宣言されたトランスフォーム

2 回以上使う並びには名前を与える。

```kdl
transform "extract_horse_id" input="string" output="string?" {
  pipeline {
    apply "regex-capture" pattern=#"/horse/([^/?#]+)"# group=1
  }
}
```

宣言されたトランスフォームは、`pipeline`、`match`、`external` のうちちょうど一つの本体を持つ。
v0.1 では、宣言されたトランスフォームは呼び出し引数も呼び出しプロパティも取らない。
異なるパラメータが必要なら、二つ目のトランスフォームを作る。

## match テーブル

スカラー値の対応表には `match` を使う。

```kdl
transform "normalize_sex" input="string" output="string" {
  match {
    case "牡" "male"
    case "牝" "female"
    case "セ" "gelding"
    default "unknown"
  }
}
```

ランタイムはソース順に、完全一致で case を比較する。
`default` はちょうど一つ必要である。
同じ入力値を持つ case が二つあるとエラーになる。
入力型と出力型は、スカラーか null 許容スカラーでなければならない。

## 外部トランスフォーム

外部トランスフォームはホスト側の関数である。
復号や内部サービスの呼び出しのように、ロジックを言語内に置けない場合に使う。

```kdl
transform "decrypt_payload" input="string" output="object" {
  external symbol="decrypt_payload"
}
```

ホストはレジストリからシンボルを供給する。
コンパイラは IR に `transform.external:decrypt_payload` ケイパビリティを追加する。
レジストリにシンボルがなければ、検証は fetch の前に失敗する。
そのため、抽出の途中で関数の不在に気付くことはない。

関数が結果を返した直後に、ランタイムはその結果を宣言された出力型に照らして検査する。
不一致は、後続のどのトランスフォームよりも先に `E_EXTERNAL_TRANSFORM_RESULT_TYPE` で失敗する。

外部トランスフォームは可搬な挙動からの意図的な離脱である。
同じプログラムを動かすには、どのランタイムでも同じレジストリが必要になる。

## 名前の解決順序

コンパイラは `apply` の引数を次の順序で解決する。

1. 組み込み名との完全一致
2. ローカルのトランスフォーム名との完全一致
3. `alias.name` 形式の修飾された import 名

import したトランスフォームは別名付きで書かなければならない。
修飾なしで参照するとエラーになる。
組み込み名は上書きできない。
したがって `apply "trim"` の意味が変わることはない。

## 次に読むもの

- [HTTP 実行](./http-execution.md)：ランタイムがドキュメントを取得する方法。
- [診断](./diagnostics.md)：コンパイルエラーの読み方。
- [パターン](./patterns.md)：よく使う組み合わせ。
