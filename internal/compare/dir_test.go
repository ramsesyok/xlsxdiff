package compare

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/ramsesyok/xlsxdiff/internal/ignore"
)

// writeTree は dir 配下に files（相対パス → Sheet1!A1 の値）を作る。
// 拡張子が .xlsx 以外のファイルは中身をそのままテキストで書く。
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, value := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(filepath.Ext(rel), ".xlsx") {
			if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		f := excelize.NewFile()
		if err := f.SetCellValue("Sheet1", "A1", value); err != nil {
			t.Fatal(err)
		}
		if err := f.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

func TestDirs(t *testing.T) {
	tests := []struct {
		name   string
		expect map[string]string
		actual map[string]string
		want   []Diff
	}{
		{
			name:   "identical including subdirectories",
			expect: map[string]string{"a.xlsx": "1", "sub/b.xlsx": "2", "sub/deep/c.xlsx": "3"},
			actual: map[string]string{"a.xlsx": "1", "sub/b.xlsx": "2", "sub/deep/c.xlsx": "3"},
		},
		{
			name:   "cell differs in subdirectory",
			expect: map[string]string{"a.xlsx": "1", "sub/b.xlsx": "2"},
			actual: map[string]string{"a.xlsx": "1", "sub/b.xlsx": "X"},
			want:   []Diff{{Kind: CellValue, File: "sub/b.xlsx", Sheet: "Sheet1", Cell: "A1", Expected: "2", Actual: "X"}},
		},
		{
			name:   "file missing and extra",
			expect: map[string]string{"a.xlsx": "1", "sub/b.xlsx": "2"},
			actual: map[string]string{"a.xlsx": "1", "sub/c.xlsx": "2"},
			want: []Diff{
				{Kind: FileMissing, File: "sub/b.xlsx"},
				{Kind: FileExtra, File: "sub/c.xlsx"},
			},
		},
		{
			name:   "same name in different directory is a different file",
			expect: map[string]string{"x/a.xlsx": "1"},
			actual: map[string]string{"y/a.xlsx": "1"},
			want: []Diff{
				{Kind: FileMissing, File: "x/a.xlsx"},
				{Kind: FileExtra, File: "y/a.xlsx"},
			},
		},
		{
			name:   "non-xlsx and lock files are ignored",
			expect: map[string]string{"a.xlsx": "1", "note.txt": "x", "old.xls": "x"},
			actual: map[string]string{"a.xlsx": "1", "other.csv": "y", "~$a.xlsx": "lock"},
		},
		{
			name:   "upper-case extension is included",
			expect: map[string]string{"A.XLSX": "1"},
			actual: map[string]string{"A.XLSX": "2"},
			want:   []Diff{{Kind: CellValue, File: "A.XLSX", Sheet: "Sheet1", Cell: "A1", Expected: "1", Actual: "2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expDir, actDir := t.TempDir(), t.TempDir()
			writeTree(t, expDir, tt.expect)
			writeTree(t, actDir, tt.actual)

			got, err := Dirs(expDir, actDir, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diffs mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestDirsErrors(t *testing.T) {
	withFile := t.TempDir()
	writeTree(t, withFile, map[string]string{"a.xlsx": "1"})
	empty := t.TempDir()
	writeTree(t, empty, map[string]string{"readme.txt": "no excel here"})
	broken := t.TempDir()
	writeTree(t, broken, map[string]string{"a.xlsx": "1"})
	if err := os.WriteFile(filepath.Join(broken, "a.xlsx"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		expect, actual string
		wantErr        string
	}{
		{"expect dir has no xlsx", empty, withFile, "no .xlsx files found"},
		{"expect dir does not exist", filepath.Join(empty, "nope"), withFile, "read expect dir"},
		{"actual dir does not exist", withFile, filepath.Join(empty, "nope"), "read actual dir"},
		{"broken xlsx", withFile, broken, "open actual"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Dirs(tt.expect, tt.actual, Options{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestDirsWithIgnore(t *testing.T) {
	expDir, actDir := t.TempDir(), t.TempDir()
	writeTree(t, expDir, map[string]string{"a.xlsx": "1", "monthly/m1.xlsx": "2", "monthly/m2.xlsx": "3"})
	writeTree(t, actDir, map[string]string{"a.xlsx": "X", "monthly/m1.xlsx": "Y", "monthly/m2.xlsx": "Z"})

	rules, err := ignore.Parse([]byte("- file: \"monthly/*.xlsx\"\n  sheet: Sheet1\n  cell: A1\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Dirs(expDir, actDir, Options{Ignore: rules})
	if err != nil {
		t.Fatal(err)
	}
	want := []Diff{{Kind: CellValue, File: "a.xlsx", Sheet: "Sheet1", Cell: "A1", Expected: "1", Actual: "X"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffs mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFilesWithIgnoreUsesExpectFileName(t *testing.T) {
	expDir, actDir := t.TempDir(), t.TempDir()
	writeTree(t, expDir, map[string]string{"report.xlsx": "1"})
	writeTree(t, actDir, map[string]string{"output.xlsx": "2"})

	rules, err := ignore.Parse([]byte("- file: report.xlsx\n  sheet: Sheet1\n  column: A\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Files(filepath.Join(expDir, "report.xlsx"), filepath.Join(actDir, "output.xlsx"), Options{Ignore: rules})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want no diffs, got %#v", got)
	}
}

// 見出し行は比較し、その下のデータは最終行まで比較しない設定（"C2:C" など）を、実ファイルで確認する。
func TestIgnoreOpenRangesOnWorkbooks(t *testing.T) {
	// 見出し + データ 3 行のブックを作る。rows[0] が見出し。
	book := func(t *testing.T, rows [][]string) string {
		t.Helper()
		f := excelize.NewFile()
		defer f.Close()
		for i, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
				t.Fatal(err)
			}
		}
		path := filepath.Join(t.TempDir(), "report.xlsx")
		if err := f.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	expect := book(t, [][]string{
		{"id", "name", "time", "note"},
		{"1", "foo", "10:00", "a"},
		{"2", "bar", "10:01", "b"},
		{"3", "baz", "10:02", "c"},
	})

	tests := []struct {
		name   string
		config string
		actual [][]string
		want   []string // 差分になるセル
	}{
		{
			name:   "C2:C ignores only data in column C",
			config: `cell: "C2:C"`,
			actual: [][]string{
				{"id", "name", "time", "note"},
				{"1", "foo", "12:00", "a"},
				{"2", "bar", "12:01", "b"},
				{"3", "baz", "12:02", "c"},
			},
		},
		{
			name:   "C2:C still detects a changed header",
			config: `cell: "C2:C"`,
			actual: [][]string{
				{"id", "name", "date", "note"},
				{"1", "foo", "12:00", "a"},
				{"2", "bar", "12:01", "b"},
				{"3", "baz", "12:02", "c"},
			},
			want: []string{"C1"},
		},
		{
			name:   "C2:C still detects changes in other columns",
			config: `cell: "C2:C"`,
			actual: [][]string{
				{"id", "name", "time", "note"},
				{"1", "foo", "12:00", "a"},
				{"2", "BAR", "12:01", "b"},
				{"3", "baz", "12:02", "X"},
			},
			want: []string{"B3", "D4"},
		},
		{
			name:   "C2:C covers extra data rows in actual",
			config: `cell: "C2:C"`,
			actual: [][]string{
				{"id", "name", "time", "note"},
				{"1", "foo", "12:00", "a"},
				{"2", "bar", "12:01", "b"},
				{"3", "baz", "12:02", "c"},
				{"", "", "12:03", ""},
			},
		},
		{
			name:   "C2:D ignores data in columns C and D",
			config: `cell: "C2:D"`,
			actual: [][]string{
				{"id", "name", "time", "note"},
				{"1", "foo", "12:00", "x"},
				{"2", "bar", "12:01", "y"},
				{"3", "baz", "12:02", "z"},
			},
		},
		{
			name:   `row "2:" ignores all data rows but keeps the header`,
			config: `row: "2:"`,
			actual: [][]string{
				{"id", "NAME", "time", "note"},
				{"9", "x", "y", "z"},
				{"9", "x", "y", "z"},
				{"9", "x", "y", "z"},
			},
			want: []string{"B1"},
		},
		{
			name:   `column "C:" ignores column C and everything to the right`,
			config: `column: "C:"`,
			actual: [][]string{
				{"id", "name", "TIME", "NOTE", "extra"},
				{"1", "foo", "x", "y", "z"},
				{"2", "BAR", "x", "y", "z"},
				{"3", "baz", "x", "y", "z"},
			},
			want: []string{"B3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ignore.Parse([]byte("- file: report.xlsx\n  sheet: Sheet1\n  " + tt.config + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := Files(expect, book(t, tt.actual), Options{Ignore: rules})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, d := range diffs {
				got = append(got, d.Cell)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diff cells = %v, want %v (diffs: %+v)", got, tt.want, diffs)
			}
		})
	}
}
