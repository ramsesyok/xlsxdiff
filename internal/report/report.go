// Package report は比較結果を出力する。
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ramsesyok/xlsxdiff/internal/compare"
)

// Text は人向けの形式で差分を出力する。差分が無ければ何も出力しない。
func Text(w io.Writer, diffs []compare.Diff) error {
	for _, d := range diffs {
		var err error
		switch d.Kind {
		case compare.SheetMissing:
			_, err = fmt.Fprintf(w, "sheet %q: missing in actual\n", d.Sheet)
		case compare.SheetExtra:
			_, err = fmt.Fprintf(w, "sheet %q: unexpected in actual\n", d.Sheet)
		case compare.SheetOrder:
			_, err = fmt.Fprintf(w, "sheet order: expected %q, actual %q\n", d.Expected, d.Actual)
		case compare.CellValue:
			_, err = fmt.Fprintf(w, "%s!%s: expected %q, actual %q\n", d.Sheet, d.Cell, d.Expected, d.Actual)
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

// JSON は機械向けの形式で差分を出力する。差分が無くても equal: true を出力する。
func JSON(w io.Writer, diffs []compare.Diff) error {
	if diffs == nil {
		diffs = []compare.Diff{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Equal bool           `json:"equal"`
		Diffs []compare.Diff `json:"diffs"`
	}{Equal: len(diffs) == 0, Diffs: diffs})
}
