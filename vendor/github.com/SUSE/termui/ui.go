// Package termui is a wrapper to enable disabling input echo and other terminal
// handling.
package termui

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/SUSE/termui/termpassword"
)

// UI is an abstraction around a terminal with various helper functions to
// write to it.
type UI struct {
	io.Reader
	io.Writer
	PasswordReader termpassword.Reader
}

// New creates a terminal UI which writes to output, reads from input
// and can be used to read passwords safely.
func New(input io.Reader, output io.Writer, passReader termpassword.Reader) *UI {
	return &UI{
		Reader:         input,
		Writer:         output,
		PasswordReader: passReader,
	}
}

// Prompt the user for a line of input.
func (u *UI) Prompt(promptText string, args ...interface{}) string {
	fmt.Fprintf(u, promptText+": ", args...)

	result, err := readLineUnbuffered(u)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(result)
}

// PromptDefault gets a line of input and provides a default if nothing is
// given.
func (u *UI) PromptDefault(promptText, defaultValue string, args ...interface{}) string {
	fmt.Fprintf(u, promptText+fmt.Sprintf(" [%s]: ", defaultValue), args...)

	result, err := readLineUnbuffered(u)
	if err != nil {
		panic(err)
	}
	result = strings.TrimSpace(result)

	if len(result) == 0 {
		return defaultValue
	}

	return result
}

// Print mirrors stdlib fmt.Print.
func (u *UI) Print(args ...interface{}) (int, error) {
	return fmt.Fprint(u, args...)
}

// Printf mirrors stdlib fmt.Printf.
func (u *UI) Printf(format string, args ...interface{}) (int, error) {
	return fmt.Fprintf(u, format, args...)
}

// Println mirrors stdlib fmt.Println.
func (u *UI) Println(args ...interface{}) (int, error) {
	return fmt.Fprintln(u, args...)
}

// readLineUnbuffered reads until the next newline without eating any extra
// bytes.
func readLineUnbuffered(reader io.Reader) (string, error) {
	buffer := &bytes.Buffer{}
	byt := make([]byte, 1)
	for {
		n, err := reader.Read(byt)
		if err != nil && err != io.EOF {
			return "", err
		}

		if n > 0 && byt[0] == '\r' {
			continue
		}
		if n == 0 || byt[0] == '\n' {
			break
		}
		buffer.WriteByte(byt[0])
	}

	return buffer.String(), nil
}
