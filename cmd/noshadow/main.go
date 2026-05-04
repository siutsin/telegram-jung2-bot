// Command noshadow runs the noshadow analyser as a standalone checker.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/siutsin/telegram-jung2-bot/tools/noshadow"
)

// main runs the standalone singlechecker entrypoint.
func main() {
	singlechecker.Main(noshadow.NewAnalyser(noshadow.Options{}))
}
