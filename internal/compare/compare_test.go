package compare

import (
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

// sheet はテスト用のシート定義。cells はセル名 → 値。
type sheet struct {
	name  string
	cells map[string]any
}

// newBook は指定したシートを順に持つブックを作る。
func newBook(t *testing.T, sheets ...sheet) *excelize.File {
	t.Helper()
	f := excelize.NewFile()
	t.Cleanup(func() { f.Close() })
	for i, s := range sheets {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", s.name); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.NewSheet(s.name); err != nil {
			t.Fatal(err)
		}
		for cell, v := range s.cells {
			if err := f.SetCellValue(s.name, cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	return f
}

func TestWorkbooks(t *testing.T) {
	tests := []struct {
		name   string
		expect []sheet
		actual []sheet
		opts   Options
		want   []Diff
	}{
		{
			name:   "identical",
			expect: []sheet{{"A", map[string]any{"A1": "x", "B2": 1}}, {"B", nil}},
			actual: []sheet{{"A", map[string]any{"A1": "x", "B2": 1}}, {"B", nil}},
		},
		{
			name:   "cell value differs",
			expect: []sheet{{"A", map[string]any{"A1": "x", "C3": 10}}},
			actual: []sheet{{"A", map[string]any{"A1": "y", "C3": 10}}},
			want:   []Diff{{Kind: CellValue, Sheet: "A", Cell: "A1", Expected: "x", Actual: "y"}},
		},
		{
			name:   "cell missing in actual",
			expect: []sheet{{"A", map[string]any{"A1": "x", "D5": "z"}}},
			actual: []sheet{{"A", map[string]any{"A1": "x"}}},
			want:   []Diff{{Kind: CellValue, Sheet: "A", Cell: "D5", Expected: "z", Actual: ""}},
		},
		{
			name:   "extra cell in actual",
			expect: []sheet{{"A", map[string]any{"A1": "x"}}},
			actual: []sheet{{"A", map[string]any{"A1": "x", "B1": "extra"}}},
			want:   []Diff{{Kind: CellValue, Sheet: "A", Cell: "B1", Expected: "", Actual: "extra"}},
		},
		{
			name:   "empty string equals blank cell",
			expect: []sheet{{"A", map[string]any{"A1": "x", "B1": ""}}},
			actual: []sheet{{"A", map[string]any{"A1": "x"}}},
		},
		{
			name:   "sheet missing and extra",
			expect: []sheet{{"A", nil}, {"B", nil}},
			actual: []sheet{{"A", nil}, {"C", nil}},
			want: []Diff{
				{Kind: SheetMissing, Sheet: "B"},
				{Kind: SheetExtra, Sheet: "C"},
			},
		},
		{
			name:   "sheet order differs",
			expect: []sheet{{"A", nil}, {"B", nil}},
			actual: []sheet{{"B", nil}, {"A", nil}},
			want:   []Diff{{Kind: SheetOrder, Expected: []string{"A", "B"}, Actual: []string{"B", "A"}}},
		},
		{
			name:   "sheet order ignored",
			expect: []sheet{{"A", nil}, {"B", nil}},
			actual: []sheet{{"B", nil}, {"A", nil}},
			opts:   Options{IgnoreSheetOrder: true},
		},
		{
			name:   "cells only compared on common sheets",
			expect: []sheet{{"A", map[string]any{"A1": 1}}, {"B", map[string]any{"A1": 1}}},
			actual: []sheet{{"A", map[string]any{"A1": 2}}},
			want: []Diff{
				{Kind: SheetMissing, Sheet: "B"},
				{Kind: CellValue, Sheet: "A", Cell: "A1", Expected: "1", Actual: "2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Workbooks(newBook(t, tt.expect...), newBook(t, tt.actual...), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diffs mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestWorkbooksRaw(t *testing.T) {
	// 同じ値 0.5 を書式違いで保存したブック。表示値は異なるが生の値は同じ。
	build := func(numFmt int) *excelize.File {
		f := newBook(t, sheet{"A", map[string]any{"A1": 0.5}})
		style, err := f.NewStyle(&excelize.Style{NumFmt: numFmt})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetCellStyle("A", "A1", "A1", style); err != nil {
			t.Fatal(err)
		}
		return f
	}
	expect, actual := build(9), build(2) // 9: "0%", 2: "0.00"

	diffs, err := Workbooks(expect, actual, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Errorf("formatted compare: want 1 diff, got %#v", diffs)
	}

	diffs, err = Workbooks(expect, actual, Options{Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("raw compare: want no diff, got %#v", diffs)
	}
}
