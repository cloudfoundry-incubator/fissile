// +build windows

package termcursor

import (
	"os"
	"os/exec"
)

func Up(lines int) string {
	// TODO: On Windows, until get this right, simply clear the screen
	// any time we want to write over something

	c := exec.Command("cmd.exe", "/c", "cls")
	c.Stdout = os.Stdout
	c.Run()

	return ""
}

func ClearToEndOfLine() string {
	return ""
}

func ClearToEndOfDisplay() string {
	return ""
}

func Show() string {
	return ""
}

func Hide() string {
	return ""
}
