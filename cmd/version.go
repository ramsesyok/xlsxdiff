package cmd

import "runtime/debug"

// SetVersion は --version で表示するバージョンを設定する。
// 空の場合は go install 時に記録されたモジュールのバージョンを使う。
func SetVersion(v string) {
	if v == "" {
		v = "dev"
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	rootCmd.Version = v
}
