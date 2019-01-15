package util

import (
	"strings"
)

// StringInSlice checks if the given string is in the given string slice, ignoring case differences
func StringInSlice(needle string, haystack []string) bool {
	for _, element := range haystack {
		if strings.EqualFold(needle, element) {
			return true
		}
	}
	return false
}
