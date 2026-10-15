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

// result は JSON 出力（標準出力・--report のファイル）の内容。
type result struct {
	Equal *bool  `json:"equal"`
	Count *int   `json:"count"`
	Error string `json:"error"`
	Diffs []struct {
		Kind, File, Sheet, Cell, Expected, Actual string
	} `json:"diffs"`
}

// parseResult は s がちょうど 1 つの JSON 文書で、必須項目（equal, count, diffs）を持つことを確認して返す。
func parseResult(t *testing.T, s string) result {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(s))
	var r result
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("invalid json %q: %v", s, err)
	}
	if dec.More() {
		t.Fatalf("want exactly one JSON document, got %q", s)
	}
	if r.Equal == nil || r.Count == nil || r.Diffs == nil {
		t.Fatalf("json must have equal, count and diffs (diffs as [] when empty): %s", s)
	}
	if *r.Count != len(r.Diffs) {
		t.Errorf("count %d != len(diffs) %d: %s", *r.Count, len(r.Diffs), s)
	}
	return r
}

func TestRootExitCodesAndJSON(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	same := writeBook(t, "same.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")
	expDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "2"})
	sameDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "2"})
	diffDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "9", "sub/extra.xlsx": "3"})

	tests := []struct {
		name      string
		args      []string
		wantCode  int
		wantEqual bool
		wantCount int
		wantError string // JSON の error と stderr に含まれるべき文字列（エラー時のみ）
	}{
		{"equal", []string{"--expect", a, "--actual", same}, exitEqual, true, 0, ""},
		{"diff", []string{"--expect", a, "--actual", b}, exitDiff, false, 1, ""},
		{"dir equal", []string{"--expect-dir", expDir, "--actual-dir", sameDir}, exitEqual, true, 0, ""},
		{"dir diff", []string{"--expect-dir", expDir, "--actual-dir", diffDir}, exitDiff, false, 2, ""},
		{"explicit json format", []string{"--expect", a, "--actual", b, "--format", "json"}, exitDiff, false, 1, ""},
		{"missing flag", []string{"--expect", a}, exitError, false, 0, "missing [actual]"},
		{"no flags", nil, exitError, false, 0, "at least one of the flags in the group [expect expect-dir] is required"},
		{"missing dir flag", []string{"--expect-dir", expDir}, exitError, false, 0, "missing [actual-dir]"},
		{"file and dir mixed", []string{"--expect", a, "--actual", b, "--expect-dir", expDir, "--actual-dir", sameDir}, exitError, false, 0, "none of the others can be"},
		{"unknown flag", []string{"--expect", a, "--actual", b, "--nope"}, exitError, false, 0, "unknown flag: --nope"},
		{"file not found", []string{"--expect", a, "--actual", filepath.Join(t.TempDir(), "nope.xlsx")}, exitError, false, 0, "open actual"},
		{"bad format", []string{"--expect", a, "--actual", b, "--format", "xml"}, exitError, false, 0, "invalid --format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr)
			}
			// 差分の有無・エラーに関わらず、標準出力には常に JSON が出る。
			r := parseResult(t, stdout)
			if *r.Equal != tt.wantEqual || *r.Count != tt.wantCount {
				t.Errorf("equal=%v count=%d, want equal=%v count=%d: %s", *r.Equal, *r.Count, tt.wantEqual, tt.wantCount, stdout)
			}
			if tt.wantError == "" {
				if r.Error != "" || stderr != "" {
					t.Errorf("unexpected error: json=%q stderr=%q", r.Error, stderr)
				}
				return
			}
			if !strings.Contains(r.Error, tt.wantError) {
				t.Errorf("json error = %q, want to contain %q", r.Error, tt.wantError)
			}
			if !strings.Contains(stderr, tt.wantError) {
				t.Errorf("stderr = %q, want to contain %q", stderr, tt.wantError)
			}
		})
	}
}

func TestRootTextFormat(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")
	expDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "2"})
	diffDir := writeDir(t, map[string]string{"top.xlsx": "1", "sub/inner.xlsx": "9", "sub/extra.xlsx": "3"})

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"equal prints nothing", []string{"--expect", a, "--actual", a}, exitEqual, "", ""},
		{"diff", []string{"--expect", a, "--actual", b}, exitDiff, "Sheet1!A1: expected \"hello\", actual \"world\"\n1 difference(s) found\n", ""},
		{"dir diff", []string{"--expect-dir", expDir, "--actual-dir", diffDir}, exitDiff,
			"file \"sub/extra.xlsx\": unexpected in actual\nsub/inner.xlsx: Sheet1!A1: expected \"2\", actual \"9\"\n2 difference(s) found\n", ""},
		{"error goes to stderr only", []string{"--expect", a, "--actual", filepath.Join(t.TempDir(), "nope.xlsx")}, exitError, "", "open actual"},
		{"quiet", []string{"--expect", a, "--actual", b, "-q"}, exitDiff, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, append(tt.args, "--format", "text")...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) || (tt.wantStderr == "" && stderr != "") {
				t.Errorf("stderr = %q, want %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestRootQuiet(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")
	for _, args := range [][]string{
		{"--expect", a, "--actual", a, "-q"},
		{"--expect", a, "--actual", b, "-q"},
		{"--expect", a, "--actual", filepath.Join(t.TempDir(), "nope.xlsx"), "-q"},
	} {
		code, stdout, stderr := run(t, args...)
		if stdout != "" {
			t.Errorf("args %v: stdout = %q, want empty (code %d, stderr %q)", args, stdout, code, stderr)
		}
	}
}

func TestRootReport(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")
	missing := filepath.Join(t.TempDir(), "nope.xlsx")
	readReport := func(t *testing.T, path string) (string, result) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("report not written: %v", err)
		}
		return string(data), parseResult(t, string(data))
	}

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantCount  int
		wantError  string
		wantStdout string // "same": 標準出力はレポートと同一、"": 標準出力は空、それ以外: 含まれるべき文字列
	}{
		{"equal", []string{"--expect", a, "--actual", a}, exitEqual, 0, "", "same"},
		{"diff", []string{"--expect", a, "--actual", b}, exitDiff, 1, "", "same"},
		{"error", []string{"--expect", a, "--actual", missing}, exitError, 0, "open actual", "same"},
		{"argument error", []string{"--expect", a}, exitError, 0, "missing [actual]", "same"},
		{"quiet still writes report", []string{"--expect", a, "--actual", b, "-q"}, exitDiff, 1, "", ""},
		{"quiet error still writes report", []string{"--expect", a, "--actual", missing, "-q"}, exitError, 0, "open actual", ""},
		{"text stdout, json report", []string{"--expect", a, "--actual", b, "--format", "text"}, exitDiff, 1, "", "1 difference(s) found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")
			code, stdout, stderr := run(t, append(tt.args, "--report", path)...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr)
			}
			content, r := readReport(t, path)
			if *r.Count != tt.wantCount || (tt.wantError == "") != (r.Error == "") || !strings.Contains(r.Error, tt.wantError) {
				t.Errorf("report count=%d error=%q, want count=%d error containing %q", *r.Count, r.Error, tt.wantCount, tt.wantError)
			}
			switch tt.wantStdout {
			case "same":
				if stdout != content {
					t.Errorf("stdout differs from report\nstdout: %s\nreport: %s", stdout, content)
				}
			case "":
				if stdout != "" {
					t.Errorf("stdout = %q, want empty", stdout)
				}
			default:
				if !strings.Contains(stdout, tt.wantStdout) {
					t.Errorf("stdout = %q, want to contain %q", stdout, tt.wantStdout)
				}
			}
		})
	}
}

func TestRootReportOverwritesAndFailures(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")

	t.Run("overwrites an existing report", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "result.json")
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 4096)), 0o644); err != nil {
			t.Fatal(err)
		}
		run(t, "--expect", a, "--actual", a, "--report", path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if r := parseResult(t, string(data)); !*r.Equal {
			t.Errorf("report = %s", data)
		}
	})

	t.Run("unwritable report path is an error with one JSON on stdout", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "no", "such", "dir", "result.json")
		code, stdout, stderr := run(t, "--expect", a, "--actual", b, "--report", path)
		if code != exitError {
			t.Errorf("exit code = %d, want %d", code, exitError)
		}
		r := parseResult(t, stdout) // 結果とエラーの 2 つの JSON が出ていないこと
		if !strings.Contains(r.Error, "write report") || !strings.Contains(stderr, "write report") {
			t.Errorf("json error = %q, stderr = %q, want write report error", r.Error, stderr)
		}
		if strings.Count(stderr, "write report") != 1 {
			t.Errorf("write report error reported more than once: %q", stderr)
		}
	})
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

func TestRootJSONDiffContent(t *testing.T) {
	a := writeBook(t, "a.xlsx", "hello")
	b := writeBook(t, "b.xlsx", "world")

	code, stdout, _ := run(t, "--expect", a, "--actual", b)
	if code != exitDiff {
		t.Fatalf("exit code = %d, want %d", code, exitDiff)
	}
	r := parseResult(t, stdout)
	if *r.Equal || *r.Count != 1 || r.Error != "" {
		t.Fatalf("unexpected json: %s", stdout)
	}
	d := r.Diffs[0]
	if d.Kind != "cell_value" || d.Sheet != "Sheet1" || d.Cell != "A1" || d.Expected != "hello" || d.Actual != "world" || d.File != "" {
		t.Errorf("unexpected diff: %+v", d)
	}
	// 正常時は error キー自体を出力しない。
	if strings.Contains(stdout, `"error"`) {
		t.Errorf("error key must be omitted on success: %s", stdout)
	}
}
