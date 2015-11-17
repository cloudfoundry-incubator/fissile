// +build darwin freebsd linux netbsd openbsd

package termcursor

import "fmt"

const csi = "\033["

// Up moves cursor up N lines.
func Up(lines int) string {
	return fmt.Sprintf("%s%dA", csi, lines)
}

// ClearToEndOfLine clears all text to the end of the line.
func ClearToEndOfLine() string {
	return fmt.Sprintf("%s%dK", csi, 0)
}

// ClearToEndOfDisplay clears all text to the end of the display.
func ClearToEndOfDisplay() string {
	return fmt.Sprintf("%s%dJ", csi, 0)
}

// Show the cursor
func Show() string {
	return csi + "?25h"
}

// Hide the cursor
func Hide() string {
	return csi + "?25l"
}
