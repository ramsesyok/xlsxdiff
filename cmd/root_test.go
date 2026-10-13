package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/xuri/excelize/v2"
)

// writeBook は Sheet1 の A1 に value を持つ xlsx を一時ディレクトリに書き出す。
func writeBook(t *testing.T, name, value string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetCellValue("Sheet1", "A1", value); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeDir は dir 配下に files（相対パス → Sheet1!A1 の値）の xlsx を作り、dir を返す。
func writeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, value := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
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
	return dir
}

// run は rootCmd を args で実行し、終了コードと stdout / stderr を返す。
func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	// rootCmd はグローバルなので、前のテストのフラグ値を既定値に戻す。
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	code := exitCode(rootCmd.Execute())
	return code, stdout.String(), stderr.String()
}

func TestRootExitCodes(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	same := writeBook(t, "same.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")
	expDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "2"})
	sameDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "2"})
	diffDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "9", "sub/extra.xlsx": "3"})

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"equal", []string{"--expect", a, "--actual", same}, exitEqual, "", ""},
		{"diff", []string{"--expect", a, "--actual", b}, exitDiff, `Sheet1!A1: expected "hello", actual "world"`, ""},
		{"quiet", []string{"--expect", a, "--actual", b, "-q"}, exitDiff, "", ""},
		{"missing flag", []string{"--expect", a}, exitError, "", "missing [actual]"},
		{"no flags", nil, exitError, "", "at least one of the flags in the group [expect expect-dir] is required"},
		{"missing dir flag", []string{"--expect-dir", expDir}, exitError, "", "missing [actual-dir]"},
		{"file and dir mixed", []string{"--expect", a, "--actual", b, "--expect-dir", expDir, "--actual-dir", sameDir}, exitError, "", "none of the others can be"},
		{"dir equal", []string{"--expect-dir", expDir, "--actual-dir", sameDir}, exitEqual, "", ""},
		{"dir diff", []string{"--expect-dir", expDir, "--actual-dir", diffDir}, exitDiff,
			"file \"sub/extra.xlsx\": unexpected in actual\nsub/inner.xlsx: Sheet1!A1: expected \"2\", actual \"9\"\n2 difference(s) found", ""},
		{"file not found", []string{"--expect", a, "--actual", filepath.Join(t.TempDir(), "nope.xlsx")}, exitError, "", "open actual"},
		{"bad format", []string{"--expect", a, "--actual", b, "--format", "xml"}, exitError, "", "invalid --format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr)
			}
			if tt.wantStdout == "" && stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stdout, tt.wantStdout) {
				t.Errorf("stdout = %q, want to contain %q", stdout, tt.wantStdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want to contain %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestRootIgnoreConfig(t *testing.T) {
	expDir := writeDir(t, map[string]string{"report.xlsx": "1", "sub/detail.xlsx": "2"})
	actDir := writeDir(t, map[string]string{"report.xlsx": "1", "sub/detail.xlsx": "changed"})
	writeConfig := func(content string) string {
		path := filepath.Join(t.TempDir(), "ignore.yaml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	matching := writeConfig("- file: sub/detail.xlsx\n  sheet: Sheet1\n  cell: A1\n")
	typo := writeConfig("- file: sub/detail.xlsx\n  sheet: Sheet1\n  cell: A1\n- file: sub/detai.xlsx\n  sheet: Sheet1\n  row: 1\n")
	invalid := writeConfig("- file: sub/detail.xlsx\n  column: 1\n")

	tests := []struct {
		name       string
		config     string
		extra      []string
		wantCode   int
		wantStderr string
	}{
		{"ignored cell makes dirs equal", matching, nil, exitEqual, ""},
		{"unused rule warns but still compares", typo, nil, exitEqual, `line 4: file "sub/detai.xlsx" sheet "Sheet1" matched no compared sheet`},
		{"quiet suppresses warning", typo, []string{"-q"}, exitEqual, ""},
		{"invalid config is an error", invalid, nil, exitError, "sheet is required"},
		{"missing config file is an error", filepath.Join(t.TempDir(), "none.yaml"), nil, exitError, "read ignore config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--expect-dir", expDir, "--actual-dir", actDir, "--ignore-config", tt.config}, tt.extra...)
			code, _, stderr := run(t, args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr)
			}
			if tt.wantStderr == "" && stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want to contain %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestRootJSON(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")

	code, stdout, _ := run(t, "--expect", a, "--actual", b, "--format", "json")
	if code != exitDiff {
		t.Fatalf("exit code = %d, want %d", code, exitDiff)
	}
	var got struct {
		Equal bool `json:"equal"`
		Diffs []struct {
			Kind, Sheet, Cell, Expected, Actual string
		} `json:"diffs"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json %q: %v", stdout, err)
	}
	if got.Equal || len(got.Diffs) != 1 || got.Diffs[0].Cell != "A1" || got.Diffs[0].Kind != "cell_value" {
		t.Errorf("unexpected json: %s", stdout)
	}

	code, stdout, _ = run(t, "--expect", a, "--actual", a, "--format", "json")
	if code != exitEqual || !strings.Contains(stdout, `"equal": true`) {
		t.Errorf("equal json: code=%d stdout=%s", code, stdout)
	}
}
