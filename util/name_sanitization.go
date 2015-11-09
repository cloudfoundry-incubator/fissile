package util

import (
	"regexp"
)

func SanitizeDockerName(name string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9_.-:]+")
	if err != nil {
		return "", err
	}

	return reg.ReplaceAllString(name, "-"), nil
}
