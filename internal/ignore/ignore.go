// Package ignore は比較から除外するセル（列・行・セル範囲）の設定を読み込み、照合する。
//
// 設定ファイルは YAML のリストで、各要素に対象ファイル（file）、シート（sheet）と、
// 除外する列（column）・行（row）・セル（cell）を Excel の表記で書く。
//
//   - file: report.xlsx      # 相対パス。* ? [ ] のワイルドカード可（/ はまたがない）
//     sheet: 集計            # 必須。シート名の完全一致
//     column: [A, "C:E"]     # 列名または列範囲
//     row: "2:5"             # 行番号または行範囲
//     cell: B1               # セル名またはセル範囲（B1:D3）
package ignore

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	yaml "go.yaml.in/yaml/v3"
)

// Excel の上限（XFD 列、1048576 行）。
const (
	maxColumn = 16384
	maxRow    = 1048576
)

// span は両端を含む番号の範囲（列番号または行番号、1 始まり）。
type span struct{ from, to int }

func (s span) has(n int) bool { return s.from <= n && n <= s.to }

// rect は両端を含むセル範囲。
type rect struct{ cols, rows span }

// Rule は設定ファイルの 1 要素。
type Rule struct {
	Line  int    // 設定ファイル上の行番号（警告・エラー表示用）
	File  string // ファイルのパターン（"/" 区切りの相対パス）
	Sheet string

	columns []span
	rows    []span
	cells   []rect
}

func (r *Rule) covers(col, row int) bool {
	for _, c := range r.columns {
		if c.has(col) {
			return true
		}
	}
	for _, rw := range r.rows {
		if rw.has(row) {
			return true
		}
	}
	for _, c := range r.cells {
		if c.cols.has(col) && c.rows.has(row) {
			return true
		}
	}
	return false
}

// Rules は読み込んだ設定全体。どのルールが実際に使われたかも記録する。
type Rules struct {
	rules []Rule
	used  []bool
}

// Load は設定ファイルを読み込む。
func Load(filename string) (*Rules, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read ignore config: %w", err)
	}
	rules, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("ignore config %s:\n  %s", filename, strings.ReplaceAll(err.Error(), "\n", "\n  "))
	}
	return rules, nil
}

// lineError は設定ファイルの行番号付きの誤り。
type lineError struct {
	line int
	msg  string
}

func (e lineError) Error() string { return fmt.Sprintf("line %d: %s", e.line, e.msg) }

// Parse は設定ファイルの内容を解釈する。誤りは行番号順に 1 行ずつまとめて返す。
func Parse(data []byte) (*Rules, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// よくある原因は、* で始まるパターンを引用符で囲んでいないこと（YAML ではエイリアス記号になる）。
		return nil, fmt.Errorf("%w\nhint: quote values that start with * such as \"*.xlsx\"", err)
	}
	rs := &Rules{}
	if len(doc.Content) == 0 {
		return rs, nil // 空のファイル
	}
	root := doc.Content[0]
	if root.Kind != yaml.SequenceNode {
		return nil, lineError{root.Line, `the config must be a list of entries starting with "- file: ..."`}
	}

	var errs []lineError
	for _, item := range root.Content {
		rule, itemErrs := parseRule(item)
		errs = append(errs, itemErrs...)
		if len(itemErrs) == 0 {
			rs.rules = append(rs.rules, rule)
		}
	}
	if len(errs) > 0 {
		slices.SortStableFunc(errs, func(a, b lineError) int { return a.line - b.line })
		joined := make([]error, len(errs))
		for i, e := range errs {
			joined[i] = e
		}
		return nil, errors.Join(joined...)
	}
	rs.used = make([]bool, len(rs.rules))
	return rs, nil
}

func parseRule(n *yaml.Node) (Rule, []lineError) {
	rule := Rule{Line: n.Line}
	errorf := func(line int, format string, args ...any) lineError {
		return lineError{line, fmt.Sprintf(format, args...)}
	}
	if n.Kind != yaml.MappingNode {
		return rule, []lineError{errorf(n.Line, `each entry must be a mapping such as "file: ..."`)}
	}

	var errs []lineError
	hasTarget := false
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		switch key.Value {
		case "file", "sheet":
			if val.Kind != yaml.ScalarNode || val.Value == "" {
				errs = append(errs, errorf(val.Line, "%s must be a non-empty string", key.Value))
				continue
			}
			if key.Value == "sheet" {
				rule.Sheet = val.Value
				continue
			}
			if strings.Contains(val.Value, `\`) {
				errs = append(errs, errorf(val.Line, "file %q: use / as the folder separator", val.Value))
				continue
			}
			if _, err := path.Match(val.Value, ""); err != nil {
				errs = append(errs, errorf(val.Line, "file %q: invalid pattern", val.Value))
				continue
			}
			rule.File = val.Value
		case "column", "row", "cell":
			hasTarget = true
			values, err := scalars(val)
			if err != nil {
				errs = append(errs, errorf(val.Line, "%s: %v", key.Value, err))
				continue
			}
			for _, v := range values {
				if err := rule.add(key.Value, v.Value); err != nil {
					errs = append(errs, errorf(v.Line, "%v", err))
				}
			}
		default:
			errs = append(errs, errorf(key.Line, "unknown key %q (allowed: file, sheet, column, row, cell)", key.Value))
		}
	}

	if rule.File == "" && !hasKey(n, "file") {
		errs = append(errs, errorf(n.Line, "file is required"))
	}
	if rule.Sheet == "" && !hasKey(n, "sheet") {
		errs = append(errs, errorf(n.Line, "sheet is required"))
	}
	if !hasTarget {
		errs = append(errs, errorf(n.Line, "at least one of column, row or cell is required"))
	}
	return rule, errs
}

// scalars は単一の値または値のリストを、値ノードのリストとして返す。
func scalars(n *yaml.Node) ([]*yaml.Node, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		return []*yaml.Node{n}, nil
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				return nil, errors.New("list items must be single values")
			}
		}
		if len(n.Content) == 0 {
			return nil, errors.New("empty list")
		}
		return n.Content, nil
	}
	return nil, errors.New("must be a value or a list of values")
}

func hasKey(n *yaml.Node, key string) bool {
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return true
		}
	}
	return false
}

var (
	columnRe = regexp.MustCompile(`^[A-Za-z]+$`)
	rowRe    = regexp.MustCompile(`^[0-9]+$`)
)

// add は column / row / cell の 1 つの値（単一または "始点:終点"）を解釈して追加する。
func (r *Rule) add(kind, value string) error {
	from, to, isRange := strings.Cut(strings.TrimSpace(value), ":")
	if !isRange {
		to = from
	}
	switch kind {
	case "column":
		a, errA := columnNumber(from)
		b, errB := columnNumber(to)
		if errA != nil || errB != nil {
			return fmt.Errorf("column %q: use column letters such as \"A\" or a range such as \"A:C\"", value)
		}
		r.columns = append(r.columns, ordered(a, b))
	case "row":
		a, errA := rowNumber(from)
		b, errB := rowNumber(to)
		if errA != nil || errB != nil {
			return fmt.Errorf("row %q: use a row number such as 2 or a range such as \"2:5\"", value)
		}
		r.rows = append(r.rows, ordered(a, b))
	case "cell":
		c1, r1, errA := excelize.CellNameToCoordinates(strings.TrimSpace(from))
		c2, r2, errB := excelize.CellNameToCoordinates(strings.TrimSpace(to))
		if errA != nil || errB != nil {
			return fmt.Errorf("cell %q: use a cell name such as \"B1\" or a range such as \"B1:D3\"", value)
		}
		r.cells = append(r.cells, rect{cols: ordered(c1, c2), rows: ordered(r1, r2)})
	}
	return nil
}

func columnNumber(s string) (int, error) {
	s = strings.TrimSpace(s)
	if !columnRe.MatchString(s) {
		return 0, errors.New("not a column name")
	}
	n, err := excelize.ColumnNameToNumber(s)
	if err != nil || n > maxColumn {
		return 0, errors.New("column out of range")
	}
	return n, nil
}

func rowNumber(s string) (int, error) {
	s = strings.TrimSpace(s)
	if !rowRe.MatchString(s) {
		return 0, errors.New("not a row number")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > maxRow {
		return 0, errors.New("row out of range")
	}
	return n, nil
}

func ordered(a, b int) span {
	if a > b {
		a, b = b, a
	}
	return span{a, b}
}

// For は file（"/" 区切りの相対パス）の sheet に当てはまるルールをまとめた判定関数を返す。
// 当てはまるルールが無ければ nil を返す。当てはまったルールは使用済みとして記録する。
func (rs *Rules) For(file, sheet string) func(col, row int) bool {
	if rs == nil {
		return nil
	}
	var matched []*Rule
	for i := range rs.rules {
		r := &rs.rules[i]
		if r.Sheet != sheet {
			continue
		}
		if ok, _ := path.Match(r.File, file); !ok {
			continue
		}
		rs.used[i] = true
		matched = append(matched, r)
	}
	if len(matched) == 0 {
		return nil
	}
	return func(col, row int) bool {
		for _, r := range matched {
			if r.covers(col, row) {
				return true
			}
		}
		return false
	}
}

// Unused は比較したどのファイル・シートにも当てはまらなかったルールを返す。
func (rs *Rules) Unused() []Rule {
	if rs == nil {
		return nil
	}
	var out []Rule
	for i, r := range rs.rules {
		if !rs.used[i] {
			out = append(out, r)
		}
	}
	return out
}
