package util

import (
	"regexp"
	"strings"
)

var (
	rgxDockerNames = regexp.MustCompile(`(?i)[^a-z0-9_.-]+`)
)

// SanitizeDockerName makes a string conform with the rules for Docker names
func SanitizeDockerName(name string) string {
	// Don't mangle template expressions; it is up to the template to
	// guarantee that only valid characters will be generated.
	if strings.HasPrefix(name, "{{") && strings.HasSuffix(name, "}}") {
		return name
	}

	return rgxDockerNames.ReplaceAllString(name, "-")
}
