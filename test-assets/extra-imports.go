// +build plan9

// Package testassets exists to pull in extra dependencies that are used for tests
// so they get vendored correctly; this file is never built.
package testassets

import (
	"github.com/golang/mock/mockgen"
)

func unused() error {
	_, err := mockgen.ParseFile("source")
	return err
}
