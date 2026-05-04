// Command gobritannia runs the gobritannia analyser as a standalone checker.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/siutsin/telegram-jung2-bot/tools/gobritannia"
)

// main runs the standalone singlechecker entrypoint.
func main() {
	singlechecker.Main(gobritannia.NewAnalyser(gobritannia.Options{}))
}
