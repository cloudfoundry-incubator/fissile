package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeDockerName(t *testing.T) {
	assert := assert.New(t)

	for input, output := range map[string]string{
		"semver+build":  "semver-build",
		"branch-name":   "branch-name",
		"remote/branch": "remote-branch",
		"image:tag":     "image-tag",
		"[Ｇｏ]\n":        "-",
	} {
		assert.Equal(output, SanitizeDockerName(input), "Incorrect sanitization")
	}
}
