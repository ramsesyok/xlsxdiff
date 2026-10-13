package compare

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// Dirs は 2 つのディレクトリ配下（サブディレクトリを含む）の .xlsx ファイルを比較する。
// ファイルは expectDir / actualDir からの相対パスで対応付け、
// ファイルの過不足を先に、続いて共通ファイルの差分を相対パス順に返す。
// 各 Diff の File には相対パス（区切り文字は "/"）が入る。
func Dirs(expectDir, actualDir string, opts Options) ([]Diff, error) {
	expFiles, err := listXLSX(expectDir)
	if err != nil {
		return nil, fmt.Errorf("read expect dir %q: %w", expectDir, err)
	}
	if len(expFiles) == 0 {
		return nil, fmt.Errorf("no .xlsx files found in expect dir %q", expectDir)
	}
	actFiles, err := listXLSX(actualDir)
	if err != nil {
		return nil, fmt.Errorf("read actual dir %q: %w", actualDir, err)
	}

	var diffs []Diff
	expSet, actSet := toSet(expFiles), toSet(actFiles)
	for _, f := range expFiles {
		if !actSet[f] {
			diffs = append(diffs, Diff{Kind: FileMissing, File: f})
		}
	}
	for _, f := range actFiles {
		if !expSet[f] {
			diffs = append(diffs, Diff{Kind: FileExtra, File: f})
		}
	}

	for _, f := range expFiles {
		if !actSet[f] {
			continue
		}
		fileDiffs, err := files(filepath.Join(expectDir, filepath.FromSlash(f)), filepath.Join(actualDir, filepath.FromSlash(f)), f, opts)
		if err != nil {
			return nil, err
		}
		for i := range fileDiffs {
			fileDiffs[i].File = f
		}
		diffs = append(diffs, fileDiffs...)
	}
	return diffs, nil
}

// listXLSX は dir 配下の .xlsx ファイルを相対パス（"/" 区切り）の昇順で返す。
// 拡張子の大文字・小文字は区別しない。Excel が開いている間に作る
// ロックファイル（~$ で始まるファイル）は除外する。
func listXLSX(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".xlsx") || strings.HasPrefix(d.Name(), "~$") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}
