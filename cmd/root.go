/*
Copyright © 2026 ramsesyok
*/
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ramsesyok/xlsxdiff/internal/compare"
	"github.com/ramsesyok/xlsxdiff/internal/ignore"
	"github.com/ramsesyok/xlsxdiff/internal/report"
)

// 終了コード。
const (
	exitEqual = 0 // 一致
	exitDiff  = 1 // 差分あり
	exitError = 2 // 引数・ファイル読込などのエラー
)

// errDiffFound は差分が見つかったことを表す。エラー表示はせず終了コード 1 にする。
var errDiffFound = errors.New("differences found")

var (
	expectPath       string
	actualPath       string
	expectDir        string
	actualDir        string
	ignoreConfig     string
	format           string
	raw              bool
	ignoreSheetOrder bool
	quiet            bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "xlsxdiff (--expect <file> --actual <file> | --expect-dir <dir> --actual-dir <dir>)",
	Short: "2 つの Excel ファイルのシート構成とセル内容が同じか確認する",
	Long: `2 つの Excel ファイル（.xlsx / .xlsm など）を比較し、
シート構成（シート名・並び順）と各セルの値が同じかを確認します。

--expect-dir / --actual-dir を指定すると、2 つのディレクトリ配下
（サブディレクトリを含む）の .xlsx ファイルを相対パスで対応付けて比較します。
ファイルの過不足も差分として報告します。

--ignore-config に YAML ファイルを指定すると、ファイル・シートごとに
列・行・セルを比較から除外できます。

終了コード:
  0  一致
  1  差分あり
  2  エラー（引数不正、ファイルが開けない など）`,
	Example: `  xlsxdiff --expect expected.xlsx --actual actual.xlsx
  xlsxdiff --expect expected.xlsx --actual actual.xlsx --format json
  xlsxdiff --expect-dir testdata/expected --actual-dir out
  xlsxdiff --expect-dir testdata/expected --actual-dir out --ignore-config ignore.yaml`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if format != "text" && format != "json" {
			return fmt.Errorf("invalid --format %q: must be text or json", format)
		}

		opts := compare.Options{Raw: raw, IgnoreSheetOrder: ignoreSheetOrder}
		if ignoreConfig != "" {
			rules, err := ignore.Load(ignoreConfig)
			if err != nil {
				return err
			}
			opts.Ignore = rules
		}
		var diffs []compare.Diff
		var err error
		if expectDir != "" {
			diffs, err = compare.Dirs(expectDir, actualDir, opts)
		} else {
			diffs, err = compare.Files(expectPath, actualPath, opts)
		}
		if err != nil {
			return err
		}

		if !quiet {
			// 当てはまらなかった無視設定は、ファイル名やシート名の誤記の可能性があるので知らせる。
			for _, r := range opts.Ignore.Unused() {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s line %d: file %q sheet %q matched no compared sheet\n", ignoreConfig, r.Line, r.File, r.Sheet)
			}
			out := cmd.OutOrStdout()
			if format == "json" {
				err = report.JSON(out, diffs)
			} else {
				err = report.Text(out, diffs)
			}
			if err != nil {
				return err
			}
		}

		if len(diffs) > 0 {
			return errDiffFound
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	os.Exit(exitCode(rootCmd.Execute()))
}

// exitCode は Execute の結果を終了コードに変換し、必要ならエラーを表示する。
func exitCode(err error) int {
	switch {
	case err == nil:
		return exitEqual
	case errors.Is(err, errDiffFound):
		return exitDiff
	default:
		fmt.Fprintln(rootCmd.ErrOrStderr(), "error:", err)
		return exitError
	}
}

func init() {
	rootCmd.Flags().StringVar(&expectPath, "expect", "", "期待値の Excel ファイルパス")
	rootCmd.Flags().StringVar(&actualPath, "actual", "", "実際の Excel ファイルパス")
	rootCmd.Flags().StringVar(&expectDir, "expect-dir", "", "期待値の Excel ファイルを置いたディレクトリ（サブディレクトリを含む）")
	rootCmd.Flags().StringVar(&actualDir, "actual-dir", "", "実際の Excel ファイルを置いたディレクトリ（サブディレクトリを含む）")
	rootCmd.Flags().StringVar(&ignoreConfig, "ignore-config", "", "比較から除外する列・行・セルを書いた YAML ファイル")
	rootCmd.Flags().StringVar(&format, "format", "text", "出力形式: text | json")
	rootCmd.Flags().BoolVar(&raw, "raw", false, "表示書式を適用せず生の値で比較する")
	rootCmd.Flags().BoolVar(&ignoreSheetOrder, "ignore-sheet-order", false, "シートの並び順の違いを無視する")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "何も出力せず終了コードのみ返す")
	// ファイル比較（--expect/--actual）かディレクトリ比較（--expect-dir/--actual-dir）の
	// どちらか一方を、両方のパスを揃えて指定させる。
	rootCmd.MarkFlagsRequiredTogether("expect", "actual")
	rootCmd.MarkFlagsRequiredTogether("expect-dir", "actual-dir")
	rootCmd.MarkFlagsMutuallyExclusive("expect", "expect-dir")
	rootCmd.MarkFlagsMutuallyExclusive("actual", "actual-dir")
	rootCmd.MarkFlagsOneRequired("expect", "expect-dir")
}
