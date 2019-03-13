package util

import (
	"fmt"
	"strings"
)

// StringInSlice checks if the given string is in the given string slice, ignoring case differences.
func StringInSlice(needle string, haystack []string) bool {
	for _, element := range haystack {
		if strings.EqualFold(needle, element) {
			return true
		}
	}
	return false
}

// PrefixString prefixes the provided 'str' with 'prefix' using 'separator' between them.
func PrefixString(str, prefix, separator string) string {
	if prefix != "" {
		return fmt.Sprintf("%s%s%s", prefix, separator, str)
	}
	return str
}
