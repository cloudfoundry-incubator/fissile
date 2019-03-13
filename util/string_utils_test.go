package util_test

import (
	"testing"

	"code.cloudfoundry.org/fissile/util"
	"github.com/stretchr/testify/assert"
)

func TestStringInSlice(t *testing.T) {
	tests := []struct {
		name     string
		needle   string
		haystack []string
		expected bool
	}{
		{
			name:     "should not match",
			needle:   "one",
			haystack: []string{"two", "three", "four"},
			expected: false,
		},
		{
			name:     "should match exactly",
			needle:   "two",
			haystack: []string{"one", "two", "three", "four"},
			expected: true,
		},
		{
			name:     "should match case-insensitive",
			needle:   "foUr",
			haystack: []string{"ONE", "TWO", "THREE", "FOUR"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := util.StringInSlice(tt.needle, tt.haystack)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestPrefixString(t *testing.T) {
	tests := []struct {
		name      string
		str       string
		prefix    string
		separator string
		expected  string
	}{
		{
			name:      "empty prefix",
			str:       "something",
			prefix:    "",
			separator: ".",
			expected:  "something",
		},
		{
			name:      "empty separator",
			str:       "something",
			prefix:    "do",
			separator: "",
			expected:  "dosomething",
		},
		{
			name:      "all set",
			str:       "something",
			prefix:    "do",
			separator: "-",
			expected:  "do-something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := util.PrefixString(tt.str, tt.prefix, tt.separator)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
