/*
Copyright © 2026 ramsesyok
*/
package main

import "github.com/ramsesyok/xlsxdiff/cmd"

// version はリリースビルド時に -ldflags "-X main.version=..." で埋め込まれる。
var version = ""

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
