package main

import (
	"flag"
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

	var duration uint
	flag.UintVar(&duration, "d", DefaultDuration, "set data update `interval` in seconds")
	flag.Parse()

	if duration == 0 {
		fmt.Fprintln(os.Stderr, "interval must be larger than 0")
		os.Exit(1)
	}

	runewidth.DefaultCondition.EastAsianWidth = false

	if err := NewApp(duration).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
