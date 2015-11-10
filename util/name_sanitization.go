package util

import (
	"regexp"
)

// SanitizeDockerName makes a string conform with the rules for Docker names
func SanitizeDockerName(name string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9_.-:]+")
	if err != nil {
		return "", err
	}

	return reg.ReplaceAllString(name, "-"), nil
}
