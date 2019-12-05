package util

import (
	"fmt"
	"strconv"
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

// QuoteList returns an array of quoted strings.
func QuoteList(words []string) []string {
	quoted := make([]string, len(words))
	for i := range words {
		quoted[i] = strconv.Quote(words[i])
	}
	return quoted
}

// WordList returns a string of all words in the 'words' slice. For 2 or more words the
// last word is prefixed by `conjunction`, and if there are at least 3 words, then they
// are separated by the oxford comma.
//
// WordList([]string{"foo", "bar"}, "and") => "foo and bar"
// WordList([]string{"foo", "bar", "baz"}, "or") => "foo, bar, or baz"
func WordList(words []string, conjunction string) string {
	length := len(words)
	switch length {
	case 0:
		return ""
	case 1:
		return words[0]
	case 2:
		return words[0] + " " + conjunction + " " + words[1]
	default:
		return fmt.Sprintf("%s, %s %s", strings.Join(words[:length-1], ", "), conjunction, words[length-1])
	}
}
