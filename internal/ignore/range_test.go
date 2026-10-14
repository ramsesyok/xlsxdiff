package ignore

import (
	"reflect"
	"strings"
	"testing"
)

// 列・行・セルの値 1 つを解釈した結果（範囲）を、表現のバリエーションごとに確認する。
// 列番号・行番号は 1 始まり。maxColumn = 16384（XFD）、maxRow = 1048576。

func TestAddColumn(t *testing.T) {
	tests := []struct {
		value string
		want  span
	}{
		// 単一
		{"A", span{1, 1}},
		{"C", span{3, 3}},
		{"c", span{3, 3}},
		{"AA", span{27, 27}},
		{"XFD", span{maxColumn, maxColumn}},
		{" C ", span{3, 3}},
		// 範囲
		{"C:E", span{3, 5}},
		{"E:C", span{3, 5}}, // 逆順は入れ替える
		{"c:e", span{3, 5}},
		{"C:c", span{3, 3}},
		{"C:C", span{3, 3}},
		{"A:XFD", span{1, maxColumn}},
		{" C : E ", span{3, 5}},
		// 最終列まで
		{"C:", span{3, maxColumn}},
		{"c:", span{3, maxColumn}},
		{"A:", span{1, maxColumn}},
		{"XFD:", span{maxColumn, maxColumn}},
		{"C: ", span{3, maxColumn}},
		{" C :", span{3, maxColumn}},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			var r Rule
			if err := r.add("column", tt.value); err != nil {
				t.Fatal(err)
			}
			if want := []span{tt.want}; !reflect.DeepEqual(r.columns, want) {
				t.Errorf("columns = %v, want %v", r.columns, want)
			}
			if r.rows != nil || r.cells != nil {
				t.Errorf("column value must not add rows or cells: %+v", r)
			}
		})
	}
}

func TestAddRow(t *testing.T) {
	tests := []struct {
		value string
		want  span
	}{
		// 単一
		{"1", span{1, 1}},
		{"2", span{2, 2}},
		{"1048576", span{maxRow, maxRow}},
		{" 2 ", span{2, 2}},
		// 範囲
		{"2:5", span{2, 5}},
		{"5:2", span{2, 5}}, // 逆順は入れ替える
		{"2:2", span{2, 2}},
		{"1:1048576", span{1, maxRow}},
		{" 2 : 5 ", span{2, 5}},
		// 最終行まで
		{"2:", span{2, maxRow}},
		{"1:", span{1, maxRow}},
		{"1048576:", span{maxRow, maxRow}},
		{"2: ", span{2, maxRow}},
		{" 2 :", span{2, maxRow}},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			var r Rule
			if err := r.add("row", tt.value); err != nil {
				t.Fatal(err)
			}
			if want := []span{tt.want}; !reflect.DeepEqual(r.rows, want) {
				t.Errorf("rows = %v, want %v", r.rows, want)
			}
			if r.columns != nil || r.cells != nil {
				t.Errorf("row value must not add columns or cells: %+v", r)
			}
		})
	}
}

func TestAddCell(t *testing.T) {
	tests := []struct {
		value string
		want  rect
	}{
		// 単一
		{"B1", rect{span{2, 2}, span{1, 1}}},
		{"b1", rect{span{2, 2}, span{1, 1}}},
		{"XFD1048576", rect{span{maxColumn, maxColumn}, span{maxRow, maxRow}}},
		{" B1 ", rect{span{2, 2}, span{1, 1}}},
		// 範囲
		{"B1:D3", rect{span{2, 4}, span{1, 3}}},
		{"D3:B1", rect{span{2, 4}, span{1, 3}}}, // 逆順は入れ替える
		{"B3:D1", rect{span{2, 4}, span{1, 3}}}, // 行だけ逆順
		{"D1:B3", rect{span{2, 4}, span{1, 3}}}, // 列だけ逆順
		{"b1:d3", rect{span{2, 4}, span{1, 3}}},
		{"C2:C2", rect{span{3, 3}, span{2, 2}}},
		{"A1:XFD1048576", rect{span{1, maxColumn}, span{1, maxRow}}},
		{" B1 : D3 ", rect{span{2, 4}, span{1, 3}}},
		// 終点を列名だけにすると最終行まで
		{"C2:C", rect{span{3, 3}, span{2, maxRow}}},
		{"c2:c", rect{span{3, 3}, span{2, maxRow}}},
		{"C2:c", rect{span{3, 3}, span{2, maxRow}}},
		{"C2:E", rect{span{3, 5}, span{2, maxRow}}},
		{"E2:C", rect{span{3, 5}, span{2, maxRow}}}, // 列が逆順
		{"C1:C", rect{span{3, 3}, span{1, maxRow}}},
		{"A1:XFD", rect{span{1, maxColumn}, span{1, maxRow}}},
		{"XFD1:XFD", rect{span{maxColumn, maxColumn}, span{1, maxRow}}},
		{"C1048576:C", rect{span{3, 3}, span{maxRow, maxRow}}},
		{" C2 : C ", rect{span{3, 3}, span{2, maxRow}}},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			var r Rule
			if err := r.add("cell", tt.value); err != nil {
				t.Fatal(err)
			}
			if want := []rect{tt.want}; !reflect.DeepEqual(r.cells, want) {
				t.Errorf("cells = %v, want %v", r.cells, want)
			}
			if r.columns != nil || r.rows != nil {
				t.Errorf("cell value must not add columns or rows: %+v", r)
			}
		})
	}
}

func TestAddInvalid(t *testing.T) {
	tests := []struct {
		kind, value string
		wantErr     string // エラーメッセージに含まれるべき文字列
	}{
		// column
		{"column", "", `column ""`},
		{"column", "1", `column "1"`},
		{"column", "C1", `column "C1"`},
		{"column", "XFE", `column "XFE"`},
		{"column", "XFE:", `column "XFE:"`},
		{"column", "A:XFE", `column "A:XFE"`},
		{"column", ":", `column ":"`},
		{"column", ":C", `column ":C"`},
		{"column", "C:1", `column "C:1"`},
		{"column", "C:E:G", `column "C:E:G"`},
		{"column", "C::", `column "C::"`},
		{"column", "C-E", `column "C-E"`},
		// row
		{"row", "", `row ""`},
		{"row", "0", `row "0"`},
		{"row", "-1", `row "-1"`},
		{"row", "2.5", `row "2.5"`},
		{"row", "A", `row "A"`},
		{"row", "1048577", `row "1048577"`},
		{"row", "0:", `row "0:"`},
		{"row", "1048577:", `row "1048577:"`},
		{"row", "2:0", `row "2:0"`},
		{"row", "2:A", `row "2:A"`},
		{"row", "A:", `row "A:"`},
		{"row", ":", `row ":"`},
		{"row", ":5", `row ":5"`},
		{"row", "2::", `row "2::"`},
		{"row", "2:5:7", `row "2:5:7"`},
		{"row", "2-5", `row "2-5"`},
		// cell: "C2:" は曖昧なので、終わりの列を書くよう案内する
		{"cell", "C2:", `write the last column too, such as "C2:C" for column C from row 2`},
		{"cell", "c2:", `such as "C2:C"`},
		{"cell", "AB10:", `such as "AB10:AB" for column AB from row 10`},
		{"cell", " C2 : ", `such as "C2:C"`},
		// cell: その他の誤り
		{"cell", "", `cell ""`},
		{"cell", "1A", `cell "1A"`},
		{"cell", "C", `cell "C"`},
		{"cell", "2", `cell "2"`},
		{"cell", "C0", `cell "C0"`},
		{"cell", "XFE1", `cell "XFE1"`},
		{"cell", "C1048577", `cell "C1048577"`},
		{"cell", ":", `cell ":"`},
		{"cell", ":C", `cell ":C"`},
		{"cell", ":C2", `cell ":C2"`},
		{"cell", "C:C2", `cell "C:C2"`},
		{"cell", "C:E", `cell "C:E"`},
		{"cell", "2:5", `cell "2:5"`},
		{"cell", "C2:2", `cell "C2:2"`},
		{"cell", "C2:C0", `cell "C2:C0"`},
		{"cell", "C2:XFE", `cell "C2:XFE"`},
		{"cell", "C0:C", `cell "C0:C"`},
		{"cell", "C2:E5:F", `cell "C2:E5:F"`},
		{"cell", "C2::", `cell "C2::"`},
		{"cell", "C2-C5", `cell "C2-C5"`},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.value, func(t *testing.T) {
			var r Rule
			err := r.add(tt.kind, tt.value)
			if err == nil {
				t.Fatalf("expected error, got columns=%v rows=%v cells=%v", r.columns, r.rows, r.cells)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
			if r.columns != nil || r.rows != nil || r.cells != nil {
				t.Errorf("invalid value must not add anything: %+v", r)
			}
		})
	}
}

// YAML として書いたときのバリエーション（引用符の有無、リスト、他の指定との組み合わせ）を確認する。
func TestParseOpenRangesInYAML(t *testing.T) {
	const head = "- file: a.xlsx\n  sheet: S\n"
	tests := []struct {
		name    string
		body    string
		columns []span
		rows    []span
		cells   []rect
	}{
		{"quoted row to end", `  row: "2:"` + "\n", nil, []span{{2, maxRow}}, nil},
		{"single-quoted row to end", `  row: '2:'` + "\n", nil, []span{{2, maxRow}}, nil},
		{"quoted column to end", `  column: "C:"` + "\n", []span{{3, maxColumn}}, nil, nil},
		{"unquoted cell to last row", "  cell: C2:C\n", nil, nil, []rect{{span{3, 3}, span{2, maxRow}}}},
		{"quoted cell to last row", `  cell: "C2:E"` + "\n", nil, nil, []rect{{span{3, 5}, span{2, maxRow}}}},
		{"flow list mixing forms", `  row: [1, "3:5", "10:"]` + "\n", nil, []span{{1, 1}, {3, 5}, {10, maxRow}}, nil},
		{"flow list with unquoted open row", "  row: [2:, 5]\n", nil, []span{{2, maxRow}, {5, 5}}, nil},
		{"block list", "  cell:\n    - C2:C\n    - \"E2:F\"\n    - B1\n", nil, nil,
			[]rect{{span{3, 3}, span{2, maxRow}}, {span{5, 6}, span{2, maxRow}}, {span{2, 2}, span{1, 1}}}},
		{"all kinds open-ended", `  column: "X:"` + "\n" + `  row: "100:"` + "\n  cell: C2:C\n",
			[]span{{24, maxColumn}}, []span{{100, maxRow}}, []rect{{span{3, 3}, span{2, maxRow}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := Parse([]byte(head + tt.body))
			if err != nil {
				t.Fatal(err)
			}
			if len(rules.rules) != 1 {
				t.Fatalf("want 1 rule, got %d", len(rules.rules))
			}
			r := rules.rules[0]
			if !reflect.DeepEqual(r.columns, tt.columns) || !reflect.DeepEqual(r.rows, tt.rows) || !reflect.DeepEqual(r.cells, tt.cells) {
				t.Errorf("got columns=%v rows=%v cells=%v\nwant columns=%v rows=%v cells=%v",
					r.columns, r.rows, r.cells, tt.columns, tt.rows, tt.cells)
			}
		})
	}
}

func TestParseOpenRangesYAMLErrors(t *testing.T) {
	const head = "- file: a.xlsx\n  sheet: S\n"
	tests := []struct {
		name    string
		body    string
		wantErr []string
	}{
		// 末尾の ":" は YAML のキー区切りになるので、引用符が必要（ヒントを出す）
		{"unquoted row to end", "  row: 2:\n", []string{"yaml", `end with :`, `"2:"`}},
		{"unquoted column to end", "  column: C:\n", []string{"yaml", `end with :`}},
		{"unquoted ambiguous cell", "  cell: C2:\n", []string{"yaml", `end with :`}},
		// 引用符で囲めば YAML は通り、曖昧な "C2:" は内容のエラーになる
		{"quoted ambiguous cell", `  cell: "C2:"` + "\n", []string{"line 3", `such as "C2:C"`}},
		{"ambiguous cell in list", `  cell: [B1, "C2:"]` + "\n", []string{"line 3", `cell "C2:"`}},
		{"ambiguous cell in block list", "  cell:\n    - B1\n    - \"D5:\"\n", []string{"line 5", `such as "D5:D"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(head + tt.body))
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

// 最終行・最終列までの指定が、境界のセルで正しく判定されることを確認する。
func TestOpenRangeBoundaries(t *testing.T) {
	rules, err := Parse([]byte("- file: a.xlsx\n  sheet: S\n  cell: \"C2:D\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	skip := rules.For("a.xlsx", "S")
	tests := []struct {
		col, row int
		ignored  bool
	}{
		{3, 1, false},      // C1: 見出しは比較する
		{4, 1, false},      // D1
		{3, 2, true},       // C2: 始点
		{4, 2, true},       // D2
		{3, maxRow, true},  // C1048576: 最終行
		{4, maxRow, true},  // D1048576
		{2, 2, false},      // B2: 左隣の列
		{5, 2, false},      // E2: 右隣の列
		{2, maxRow, false}, // B1048576
		{5, maxRow, false}, // E1048576
		{1, 1, false},      // A1
		{maxColumn, 2, false},
	}
	for _, tt := range tests {
		if got := skip(tt.col, tt.row); got != tt.ignored {
			t.Errorf("cell (col %d, row %d): ignored = %v, want %v", tt.col, tt.row, got, tt.ignored)
		}
	}
}
