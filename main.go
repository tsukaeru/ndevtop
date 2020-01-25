package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/mattn/go-runewidth"
)

func main() {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "this program only supports Linux")
		os.Exit(1)
	}

	runewidth.DefaultCondition.EastAsianWidth = false

	if err := NewApp().Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
