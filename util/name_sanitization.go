package util

import (
	"regexp"
)

var (
	rgxDockerNames = regexp.MustCompile(`(?i)[^a-z0-9_.-]+`)
)

// SanitizeDockerName makes a string conform with the rules for Docker names
func SanitizeDockerName(name string) string {
	return rgxDockerNames.ReplaceAllString(name, "-")
}
