package termpassword

import "io"

//go:generate counterfeiter -o fakes/fake_password_reader.go . Reader

// Reader allows invisible input for password entry.
type Reader interface {
	PromptForPassword(promptText string, args ...interface{}) string
}

type passwordReader struct {
	io.Reader
}

// NewReader constructor. Requires an exit handler
func NewReader() Reader {
	return &passwordReader{}
}
