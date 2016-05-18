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

var version = "0+dev"

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

	if version == "" {
		ui.Println(color.RedString("Fissile was built incorrectly and its version string is missing"))
		sigint.DefaultHandler.Exit(1)
	}

	f := app.NewFissileApplication(version, ui)

	if err := cmd.Execute(f); err != nil {
		ui.Println(color.RedString("%v", err))
		sigint.DefaultHandler.Exit(1)
	}
}
