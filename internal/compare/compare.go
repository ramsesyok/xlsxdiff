// Package compare は 2 つの Excel ブックのシート構成とセル内容を比較する。
package compare

import (
	"fmt"
	"slices"

	"github.com/xuri/excelize/v2"
)

// DiffKind は差分の種類を表す。
type DiffKind string

const (
	// SheetMissing は expect にあるシートが actual に存在しないことを表す。
	SheetMissing DiffKind = "sheet_missing"
	// SheetExtra は actual に expect にないシートが存在することを表す。
	SheetExtra DiffKind = "sheet_extra"
	// SheetOrder は共通するシートの並び順が異なることを表す。
	SheetOrder DiffKind = "sheet_order"
	// CellValue はセルの値が異なることを表す。
	CellValue DiffKind = "cell_value"
	// FileMissing はディレクトリ比較で expect にあるファイルが actual に存在しないことを表す。
	FileMissing DiffKind = "file_missing"
	// FileExtra はディレクトリ比較で actual に expect にないファイルが存在することを表す。
	FileExtra DiffKind = "file_extra"
)

// Diff は 1 件の差分を表す。
type Diff struct {
	Kind DiffKind `json:"kind"`
	// File はディレクトリ比較時の相対パス。ファイル比較では空。
	File     string `json:"file,omitempty"`
	Sheet    string `json:"sheet,omitempty"`
	Cell     string `json:"cell,omitempty"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

// Options は比較の挙動を制御する。
type Options struct {
	// Raw が true の場合、表示書式を適用しない生の値で比較する。
	Raw bool
	// IgnoreSheetOrder が true の場合、シートの並び順の違いを無視する。
	IgnoreSheetOrder bool
}

// Files は 2 つの Excel ファイルを開いて比較する。
func Files(expectPath, actualPath string, opts Options) ([]Diff, error) {
	expect, err := excelize.OpenFile(expectPath)
	if err != nil {
		return nil, fmt.Errorf("open expect %q: %w", expectPath, err)
	}
	defer expect.Close()

	actual, err := excelize.OpenFile(actualPath)
	if err != nil {
		return nil, fmt.Errorf("open actual %q: %w", actualPath, err)
	}
	defer actual.Close()

	return Workbooks(expect, actual, opts)
}

// Workbooks は開済みの 2 つのブックを比較する。
// シート構成の差分を先に、続いて共通シートのセル差分を expect のシート順に返す。
func Workbooks(expect, actual *excelize.File, opts Options) ([]Diff, error) {
	expSheets := expect.GetSheetList()
	actSheets := actual.GetSheetList()

	diffs := compareSheetLists(expSheets, actSheets, opts)

	actSet := toSet(actSheets)
	for _, sheet := range expSheets {
		if !actSet[sheet] {
			continue
		}
		cellDiffs, err := compareSheet(expect, actual, sheet, opts)
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, cellDiffs...)
	}
	return diffs, nil
}

func compareSheetLists(exp, act []string, opts Options) []Diff {
	var diffs []Diff
	expSet, actSet := toSet(exp), toSet(act)

	for _, s := range exp {
		if !actSet[s] {
			diffs = append(diffs, Diff{Kind: SheetMissing, Sheet: s})
		}
	}
	for _, s := range act {
		if !expSet[s] {
			diffs = append(diffs, Diff{Kind: SheetExtra, Sheet: s})
		}
	}

	// 並び順は両者に共通するシートだけで判定する（欠落・余分と二重に報告しないため）。
	if !opts.IgnoreSheetOrder {
		expCommon := filter(exp, actSet)
		actCommon := filter(act, expSet)
		if !slices.Equal(expCommon, actCommon) {
			diffs = append(diffs, Diff{Kind: SheetOrder, Expected: expCommon, Actual: actCommon})
		}
	}
	return diffs
}

// compareSheet はシート内のセル値を比較する。
// 値が空のセルと存在しないセルは同じものとして扱う。
func compareSheet(expect, actual *excelize.File, sheet string, opts Options) ([]Diff, error) {
	ro := excelize.Options{RawCellValue: opts.Raw}
	expRows, err := expect.GetRows(sheet, ro)
	if err != nil {
		return nil, fmt.Errorf("read expect sheet %q: %w", sheet, err)
	}
	actRows, err := actual.GetRows(sheet, ro)
	if err != nil {
		return nil, fmt.Errorf("read actual sheet %q: %w", sheet, err)
	}

	var diffs []Diff
	for r := range max(len(expRows), len(actRows)) {
		expRow, actRow := at(expRows, r), at(actRows, r)
		for c := range max(len(expRow), len(actRow)) {
			ev, av := at(expRow, c), at(actRow, c)
			if ev == av {
				continue
			}
			name, err := excelize.CoordinatesToCellName(c+1, r+1)
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, Diff{Kind: CellValue, Sheet: sheet, Cell: name, Expected: ev, Actual: av})
		}
	}
	return diffs, nil
}

// at は範囲外ならゼロ値を返すインデックスアクセス。
func at[T any](s []T, i int) T {
	var zero T
	if i < len(s) {
		return s[i]
	}
	return zero
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func filter(ss []string, keep map[string]bool) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if keep[s] {
			out = append(out, s)
		}
	}
	return out
}
