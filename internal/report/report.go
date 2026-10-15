// Package report は比較結果を出力する。
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ramsesyok/xlsxdiff/internal/compare"
)

// Text は人向けの形式で差分を出力する。差分が無ければ何も出力しない。
func Text(w io.Writer, diffs []compare.Diff) error {
	for _, d := range diffs {
		// ディレクトリ比較ではどのファイルの差分かを先頭に付ける。
		prefix := ""
		if d.File != "" {
			prefix = d.File + ": "
		}
		var err error
		switch d.Kind {
		case compare.FileMissing:
			_, err = fmt.Fprintf(w, "file %q: missing in actual\n", d.File)
		case compare.FileExtra:
			_, err = fmt.Fprintf(w, "file %q: unexpected in actual\n", d.File)
		case compare.SheetMissing:
			_, err = fmt.Fprintf(w, "%ssheet %q: missing in actual\n", prefix, d.Sheet)
		case compare.SheetExtra:
			_, err = fmt.Fprintf(w, "%ssheet %q: unexpected in actual\n", prefix, d.Sheet)
		case compare.SheetOrder:
			_, err = fmt.Fprintf(w, "%ssheet order: expected %q, actual %q\n", prefix, d.Expected, d.Actual)
		case compare.CellValue:
			_, err = fmt.Fprintf(w, "%s%s!%s: expected %q, actual %q\n", prefix, d.Sheet, d.Cell, d.Expected, d.Actual)
		}
		if err != nil {
			return err
		}
	}
	if len(diffs) > 0 {
		_, err := fmt.Fprintf(w, "%d difference(s) found\n", len(diffs))
		return err
	}
	return nil
}

// Result は JSON 出力の内容。
type Result struct {
	// Equal は比較対象がすべて一致したかどうか。エラーのときは false。
	Equal bool `json:"equal"`
	// Count は差分の件数。
	Count int `json:"count"`
	// Error は比較できなかったときのエラーメッセージ。正常に比較できたときは出力しない。
	Error string `json:"error,omitempty"`
	// Diffs は差分の一覧。差分が無ければ空配列。
	Diffs []compare.Diff `json:"diffs"`
}

// NewResult は比較結果から Result を作る。
func NewResult(diffs []compare.Diff) Result {
	if diffs == nil {
		diffs = []compare.Diff{}
	}
	return Result{Equal: len(diffs) == 0, Count: len(diffs), Diffs: diffs}
}

// ErrorResult は比較できなかったときの Result を作る。
func ErrorResult(err error) Result {
	return Result{Error: err.Error(), Diffs: []compare.Diff{}}
}

// JSON は Result を JSON で出力する。
func JSON(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteFile は Result を JSON ファイルに書き出す（既存のファイルは上書きする）。
func WriteFile(path string, r Result) error {
	var buf bytes.Buffer
	if err := JSON(&buf, r); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
