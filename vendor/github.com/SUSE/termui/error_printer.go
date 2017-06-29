package termui

import (
	"github.com/fatih/color"
	"github.com/SUSE/termui/sigint"
)

var (
	exitFunction = sigint.DefaultHandler.Exit
)

const (
	// CodeUnknownError is a generic error code for errors without one
	CodeUnknownError = iota + 1
)

// Error objects support having an error code in addition to the message
type Error interface {
	error
	Code() int
}

// ErrorPrinter can be used to stop repetitive calls to the global like calls
// which all take a terminal.UI
type ErrorPrinter struct {
	ui *UI
}

// NewErrorPrinter returns an error printer.
func NewErrorPrinter(ui *UI) *ErrorPrinter {
	return &ErrorPrinter{
		ui: ui,
	}
}

// PrintAndExit prints the error to the UI and exits.
func (e ErrorPrinter) PrintAndExit(err error) {
	PrintAndExit(e.ui, err)
}

// PrintWarning prints the error as a warning.
func (e ErrorPrinter) PrintWarning(err error) {
	PrintWarning(e.ui, err)
}

// PrintAndExit prints the error to the ui and exits the program.
func PrintAndExit(ui *UI, err error) {
	errCode := getErrorCode(err)

	errMsg := color.RedString("Error (%d): %s", errCode, err)
	ui.Println(errMsg)
	exitFunction(errCode)
}

// PrintWarning to the ui.
func PrintWarning(ui *UI, err error) {
	errCode := getErrorCode(err)

	errMsg := color.YellowString("Warning (%d): %s", errCode, err)
	ui.Println(errMsg)
}

func getErrorCode(err error) int {
	if errWithCode, ok := err.(Error); ok {
		return errWithCode.Code()
	}

	return CodeUnknownError
}
