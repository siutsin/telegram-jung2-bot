package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/siutsin/telegram-jung2-bot/tools/noshadow"
)

func main() {
	singlechecker.Main(noshadow.Analyser)
}
