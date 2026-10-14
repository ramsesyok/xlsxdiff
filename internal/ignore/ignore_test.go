package ignore

import (
	"strings"
	"testing"
)

// cellRef はテスト用のセル指定（列・行は 1 始まり）。
type cellRef struct{ col, row int }

func TestParseAndMatch(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		file    string
		sheet   string
		ignored []cellRef
		kept    []cellRef
	}{
		{
			name:   "single column",
			config: "- file: a.xlsx\n  sheet: S\n  column: B\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{2, 1}, {2, 1048576}},
			kept:    []cellRef{{1, 1}, {3, 1}},
		},
		{
			name:   "lower-case column and column range",
			config: "- file: a.xlsx\n  sheet: S\n  column: [a, \"D:F\"]\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{1, 5}, {4, 1}, {6, 1}},
			kept:    []cellRef{{2, 1}, {3, 1}, {7, 1}},
		},
		{
			name:   "row number and row range written in reverse",
			config: "- file: a.xlsx\n  sheet: S\n  row: [2, 9:7]\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{1, 2}, {50, 7}, {1, 9}},
			kept:    []cellRef{{1, 1}, {1, 3}, {1, 10}},
		},
		{
			name:   "cell and cell range",
			config: "- file: a.xlsx\n  sheet: S\n  cell: [B1, \"C3:D4\"]\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{2, 1}, {3, 3}, {4, 4}, {3, 4}},
			kept:    []cellRef{{1, 1}, {2, 2}, {5, 4}, {3, 5}},
		},
		{
			name:   "column, row and cell in one entry",
			config: "- file: a.xlsx\n  sheet: S\n  column: A\n  row: 3\n  cell: E5\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{1, 9}, {9, 3}, {5, 5}},
			kept:    []cellRef{{2, 2}},
		},
		{
			name:   "row to the last row",
			config: "- file: a.xlsx\n  sheet: S\n  row: \"2:\"\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{1, 2}, {1, 1048576}},
			kept:    []cellRef{{1, 1}},
		},
		{
			name:   "column to the last column",
			config: "- file: a.xlsx\n  sheet: S\n  column: \"C:\"\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{3, 1}, {16384, 1}},
			kept:    []cellRef{{2, 1}},
		},
		{
			name:   "cell range to the last row keeps the header",
			config: "- file: a.xlsx\n  sheet: S\n  cell: C2:C\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{3, 2}, {3, 1048576}},
			kept:    []cellRef{{3, 1}, {2, 2}, {4, 2}},
		},
		{
			name:   "multi-column cell range to the last row, lower case and reversed",
			config: "- file: a.xlsx\n  sheet: S\n  cell: [\"c2:e\", \"H5:G\"]\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{3, 2}, {5, 100}, {7, 5}, {8, 1048576}},
			kept:    []cellRef{{4, 1}, {6, 2}, {7, 4}, {9, 5}},
		},
		{
			name:   "wildcard in file",
			config: "- file: \"monthly/*.xlsx\"\n  sheet: S\n  cell: A1\n",
			file:   "monthly/2026-10.xlsx", sheet: "S",
			ignored: []cellRef{{1, 1}},
		},
		{
			name:   "wildcard does not cross folders",
			config: "- file: \"*.xlsx\"\n  sheet: S\n  cell: A1\n",
			file:   "monthly/2026-10.xlsx", sheet: "S",
			kept: []cellRef{{1, 1}},
		},
		{
			name:   "other sheet is not affected",
			config: "- file: a.xlsx\n  sheet: S\n  cell: A1\n",
			file:   "a.xlsx", sheet: "T",
			kept: []cellRef{{1, 1}},
		},
		{
			name:   "multiple entries for the same file",
			config: "- file: a.xlsx\n  sheet: S\n  cell: A1\n- file: a.xlsx\n  sheet: S\n  cell: B2\n",
			file:   "a.xlsx", sheet: "S",
			ignored: []cellRef{{1, 1}, {2, 2}},
			kept:    []cellRef{{1, 2}},
		},
		{
			name:   "Japanese file and sheet names",
			config: "- file: あああ.xlsx\n  sheet: 集計\n  column: A\n",
			file:   "あああ.xlsx", sheet: "集計",
			ignored: []cellRef{{1, 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := Parse([]byte(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			skip := rules.For(tt.file, tt.sheet)
			for _, c := range tt.ignored {
				if skip == nil || !skip(c.col, c.row) {
					t.Errorf("cell (col %d, row %d) should be ignored", c.col, c.row)
				}
			}
			for _, c := range tt.kept {
				if skip != nil && skip(c.col, c.row) {
					t.Errorf("cell (col %d, row %d) should be compared", c.col, c.row)
				}
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr []string
	}{
		{"not a list", "file: a.xlsx\n", []string{"line 1", "must be a list"}},
		{"missing sheet", "- file: a.xlsx\n  column: A\n", []string{"line 1", "sheet is required"}},
		{"missing file", "- sheet: S\n  column: A\n", []string{"file is required"}},
		{"missing target", "- file: a.xlsx\n  sheet: S\n", []string{"at least one of column, row or cell"}},
		{"numeric column", "- file: a.xlsx\n  sheet: S\n  column: 1\n", []string{"line 3", `column "1"`, `"A:C"`}},
		{"column out of range", "- file: a.xlsx\n  sheet: S\n  column: XFE\n", []string{`column "XFE"`}},
		{"letter row", "- file: a.xlsx\n  sheet: S\n  row: A\n", []string{`row "A"`}},
		{"row zero", "- file: a.xlsx\n  sheet: S\n  row: 0\n", []string{`row "0"`}},
		{"numeric cell pair", "- file: a.xlsx\n  sheet: S\n  cell: [1, 2]\n", []string{`cell "1"`, `cell "2"`}},
		{"bad cell", "- file: a.xlsx\n  sheet: S\n  cell: 1A\n", []string{`cell "1A"`}},
		{"ambiguous open cell range", "- file: a.xlsx\n  sheet: S\n  cell: \"C2:\"\n", []string{`cell "C2:"`, `"C2:C"`, "last column too"}},
		{"open start column", "- file: a.xlsx\n  sheet: S\n  column: \":C\"\n", []string{`column ":C"`}},
		{"open start row", "- file: a.xlsx\n  sheet: S\n  row: \":5\"\n", []string{`row ":5"`}},
		{"open start cell", "- file: a.xlsx\n  sheet: S\n  cell: \":C\"\n", []string{`cell ":C"`}},
		{"unknown key", "- file: a.xlsx\n  sheet: S\n  colum: A\n", []string{"line 3", `unknown key "colum"`}},
		{"backslash", "- file: 'sub\\a.xlsx'\n  sheet: S\n  cell: A1\n", []string{"use / as the folder separator"}},
		{"bad pattern", "- file: \"[a.xlsx\"\n  sheet: S\n  cell: A1\n", []string{"invalid pattern"}},
		{"nested list", "- file: a.xlsx\n  sheet: S\n  cell: [[A1]]\n", []string{"single values"}},
		{
			"errors from several entries are all reported",
			"- file: a.xlsx\n  sheet: S\n  column: 1\n- file: b.xlsx\n  row: 2\n",
			[]string{"line 3", "line 4", "sheet is required"},
		},
		{"invalid yaml", "- file: a.xlsx\n   sheet: S\n", []string{"yaml"}},
		{"unquoted trailing colon", "- file: a.xlsx\n  sheet: S\n  row: 2:\n", []string{"yaml", `end with :`, `"2:"`}},
		{"unquoted leading star", "- file: *.xlsx\n  sheet: S\n  cell: A1\n", []string{"yaml", `quote values that start with *`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.config))
			if err == nil {
				t.Fatal("expected error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestUnused(t *testing.T) {
	rules, err := Parse([]byte("- file: a.xlsx\n  sheet: S\n  cell: A1\n- file: b.xlsx\n  sheet: S\n  cell: A1\n"))
	if err != nil {
		t.Fatal(err)
	}
	rules.For("a.xlsx", "S")
	unused := rules.Unused()
	if len(unused) != 1 || unused[0].File != "b.xlsx" || unused[0].Line != 4 {
		t.Errorf("unused = %+v, want the b.xlsx rule at line 4", unused)
	}
}

func TestNilAndEmpty(t *testing.T) {
	var nilRules *Rules
	if nilRules.For("a.xlsx", "S") != nil || nilRules.Unused() != nil {
		t.Error("nil rules must not ignore anything")
	}
	rules, err := Parse([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if rules.For("a.xlsx", "S") != nil {
		t.Error("empty config must not ignore anything")
	}
}
