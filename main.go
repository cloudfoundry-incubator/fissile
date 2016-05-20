package main

import (
	"os"
	"runtime"

	"github.com/hpcloud/fissile/app"
	"github.com/hpcloud/fissile/cmd"

	"github.com/fatih/color"
	"github.com/hpcloud/termui"
	"github.com/hpcloud/termui/sigint"
)

var version = "0"

func main() {

	var ui *termui.UI

	if runtime.GOOS == "windows" {
		ui = termui.New(
			os.Stdin,
			color.Output,
			nil,
		)
	} else {
		ui = termui.New(
			os.Stdin,
			os.Stdout,
			nil,
		)
	}

	switch {
	case version == "":
		ui.Println(color.RedString("Fissile was built incorrectly and its version string is empty."))
		sigint.DefaultHandler.Exit(1)
	case version == "0":
		ui.Println(color.RedString("Fissile was built incorrectly and it doesn't have a proper version string."))
	}

	f := app.NewFissileApplication(version, ui)

	if err := cmd.Execute(f, version); err != nil {
		ui.Println(color.RedString("%v", err))
		sigint.DefaultHandler.Exit(1)
	}
}
