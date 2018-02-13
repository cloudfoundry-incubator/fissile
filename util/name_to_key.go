package util

import (
	"strings"
)

// ConvertNameToKey transforms a parameter name into a key acceptable to kube.
func ConvertNameToKey(name string) string {
	return strings.Replace(strings.ToLower(name), "_", "-", -1)
}
