package main

import (
	"os"
	"runtime"

	"github.com/SUSE/termui"
	"github.com/SUSE/termui/sigint"
	"github.com/cloudfoundry-incubator/fissile/app"
	"github.com/cloudfoundry-incubator/fissile/cmd"

	"github.com/fatih/color"
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
