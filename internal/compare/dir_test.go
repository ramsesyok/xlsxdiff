package compare

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
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
