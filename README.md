# xlsxdiff

2 つの Excel ファイル（.xlsx / .xlsm など）が、同じシート構成・同じセル内容を持つかを確認する CLI ツールです。
テストツール（runn の `exec` ステップ、CI のシェルスクリプトなど）から呼び出しやすいよう、結果を終了コードで返します。

## インストール

```sh
go install github.com/ramsesyok/xlsxdiff@latest
```

Go がない環境では、[Releases](https://github.com/ramsesyok/xlsxdiff/releases) から OS / アーキテクチャに合ったアーカイブ（Linux / macOS / Windows × amd64 / arm64）をダウンロードし、`xlsxdiff` を PATH の通った場所に置いてください。

## 使い方

```sh
# ファイル同士を比較
xlsxdiff --expect expected.xlsx --actual actual.xlsx

# ディレクトリ同士を比較（サブディレクトリを含む .xlsx をすべて比較）
xlsxdiff --expect-dir testdata/expected --actual-dir out
```

`--expect` / `--actual`（ファイル比較）か `--expect-dir` / `--actual-dir`（ディレクトリ比較）のどちらか一方を、2 つ揃えて指定します。

| オプション | 説明 |
|---|---|
| `--expect <file>` | 期待値の Excel ファイル |
| `--actual <file>` | 実際の Excel ファイル |
| `--expect-dir <dir>` | 期待値の Excel ファイルを置いたディレクトリ |
| `--actual-dir <dir>` | 実際の Excel ファイルを置いたディレクトリ |
| `--format json\|text` | 標準出力の形式（既定 `json`）。`text` は人向けの表示 |
| `--report <file>` | 結果の JSON をファイルにも書き出す（標準出力への出力はそのまま） |
| `--raw` | 表示書式（数値書式・日付書式）を適用せず、生の値で比較する |
| `--ignore-sheet-order` | シートの並び順の違いを無視する |
| `--ignore-config <file>` | 比較から除外する列・行・セルを書いた YAML ファイル |
| `-q, --quiet` | 標準出力に何も出力しない（`--report` のファイルは書き出す） |
| `--version` | バージョン表示 |

### 終了コード

| コード | 意味 |
|---|---|
| `0` | 一致 |
| `1` | 差分あり |
| `2` | エラー（必須引数なし、ファイルが開けない、`--expect-dir` に .xlsx が 1 つもない など） |

## 比較内容

- **シート構成**: シート名の過不足と、並び順（両方に存在するシートで判定）
- **セル内容**: 両方に存在するシートの全セルの値。既定では Excel 上の表示値（書式適用後の文字列）で比較し、`--raw` で生の値で比較します
- 値が空のセルと存在しないセルは同じとみなします
- 数式そのもの、書式（フォント・色・罫線）、結合セル、列幅などは比較対象外です
- **ディレクトリ比較**: 両ディレクトリ配下（サブディレクトリを含む）の `.xlsx` を相対パスで対応付けて比較し、ファイルの過不足も差分として報告します。拡張子の大文字・小文字は区別せず、Excel のロックファイル（`~$` で始まるファイル）と `.xlsx` 以外のファイルは無視します

### 比較から除外する（無視設定）

`--ignore-config` に YAML ファイルを指定すると、ファイル・シートごとに列・行・セルを比較から除外できます。

```yaml
- file: report.xlsx          # ディレクトリ比較では相対パス、ファイル比較では --expect のファイル名
  sheet: 集計                # 必須（完全一致）
  column: A                  # 列（"C:E" で範囲）
- file: "monthly/*.xlsx"     # ワイルドカード可（* は / をまたがない）
  sheet: 表紙
  row: [2, "10:12"]          # 行（リストで複数指定可）
  cell: ["B1", "D5:E6"]      # セル（範囲は B1:D3 のように）
- file: report.xlsx
  sheet: Sheet1
  cell: "C2:C"               # 最終行まで: C 列の 2 行目以降（見出しの C1 は比較する）
                             # 行・列は "2:"（2 行目以降）、"C:"（C 列以降）とも書ける
```

書き方の誤りはエラー（終了コード `2`）、どのファイル・シートにも当てはまらなかった指定は警告（標準エラー出力）になります。

判定基準の詳細（具体例つき、テスト管理者向け）は [how-compare.md](how-compare.md) を参照してください。

## 出力

結果は**差分の有無に関わらず**、JSON で標準出力に出力します。`--report <file>` を指定すると、同じ JSON をファイルにも書き出します。

### 差分ありの場合（終了コード `1`）

```json
{
  "equal": false,
  "count": 2,
  "diffs": [
    { "kind": "sheet_missing", "sheet": "Summary" },
    { "kind": "cell_value", "sheet": "Sheet1", "cell": "B3", "expected": "100", "actual": "120" }
  ]
}
```

### 一致した場合（終了コード `0`）

```json
{
  "equal": true,
  "count": 0,
  "diffs": []
}
```

### エラーの場合（終了コード `2`）

比較できなかったときは `error` にメッセージが入ります（標準エラー出力にも表示します）。引数の誤りなど、比較の前に起きたエラーも同じ形で出力します。

```json
{
  "equal": false,
  "count": 0,
  "error": "open actual \"out/report.xlsx\": open out/report.xlsx: no such file or directory",
  "diffs": []
}
```

| 項目 | 内容 |
|---|---|
| `equal` | すべて一致したら `true`。差分あり・エラーのときは `false` |
| `count` | 差分の件数（`diffs` の要素数） |
| `error` | エラーメッセージ。正常に比較できたときは項目自体がない |
| `diffs` | 差分の一覧。差分がなければ空配列 `[]` |

`diffs` の各要素の `kind` は次のいずれかです。ディレクトリ比較では各要素に `file`（相対パス）が付きます。

| `kind` | 意味 | 主な項目 |
|---|---|---|
| `file_missing` / `file_extra` | ファイルの不足 / 余分（ディレクトリ比較のみ） | `file` |
| `sheet_missing` / `sheet_extra` | シートの不足 / 余分 | `sheet` |
| `sheet_order` | シートの並び順の違い | `expected`, `actual`（シート名の配列） |
| `cell_value` | セルの値の違い | `sheet`, `cell`, `expected`, `actual` |

無視設定がどこにも当てはまらなかったときの警告は、JSON には含めず標準エラー出力に表示します。

### 人向けの表示（`--format text`）

```
sheet "Summary": missing in actual
Sheet1!B3: expected "100", actual "120"
2 difference(s) found
```

ディレクトリ比較では、各行の先頭に対象ファイルの相対パスが付きます。一致した場合は何も表示しません。
`--format text` でも、`--report` のファイルは JSON で書き出します。

## runn から使う例

標準出力の JSON を `fromJSON(current.stdout)` で読み取れます。

```yaml
steps:
  compareReport:
    exec:
      command: xlsxdiff --expect testdata/expected.xlsx --actual out/report.xlsx
    test: current.exit_code == 0 && fromJSON(current.stdout).count == 0
  compareAllReports:
    exec:
      command: xlsxdiff --expect-dir testdata/expected --actual-dir out --report out/xlsxdiff-result.json
    test: |
      current.exit_code == 0
      && fromJSON(current.stdout).equal
```

- 終了コード `1`（差分あり）と `2`（エラー）は、runn の `exec` ステップ自体の失敗にはならないので、`test` で判定します。
- `--report` で保存した JSON は、テスト後に差分の内容を確認したり、別のツールで集計したりするのに使えます。

## リリース

`v*` 形式のタグを push すると、GitHub Actions（`.github/workflows/release.yml`）が GoReleaser でビルドし、GitHub Releases に公開します。
Actions タブから `Release` ワークフローを手動実行（`tag` に `v0.1.0` などを指定）しても、タグ作成とリリースをまとめて行えます。

依存ライブラリを更新したら `go run scripts/licenses.go` で `THIRD_PARTY_NOTICES.md` を再生成してください（CI で差分をチェックしています）。

## ライセンス

自作部分は [MIT License](LICENSE)（Copyright (c) 2026 ramsesyok）です。
依存ライブラリの表示は [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) を参照してください。
