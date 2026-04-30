package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/lewispb/wifiui/internal/ui"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("wifiui"),
			app.Size(unit.Dp(560), unit.Dp(820)),
			app.MinSize(unit.Dp(360), unit.Dp(420)),
		)
		if err := ui.Run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
