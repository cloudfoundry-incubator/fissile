package validation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeFuncs(t *testing.T) {
	testCases := []struct {
		fn       func() *Error
		expected ErrorType
	}{
		{
			func() *Error { return Invalid("a", "b", "c") },
			ErrorTypeInvalid,
		},
		{
			func() *Error { return NotSupported("a", "b", nil) },
			ErrorTypeNotSupported,
		},
		{
			func() *Error { return Duplicate("a", "b") },
			ErrorTypeDuplicate,
		},
		{
			func() *Error { return NotFound("a", "b") },
			ErrorTypeNotFound,
		},
		{
			func() *Error { return Required("a", "b") },
			ErrorTypeRequired,
		},
		{
			func() *Error { return InternalError("a", fmt.Errorf("b")) },
			ErrorTypeInternal,
		},
	}

	for _, testCase := range testCases {
		err := testCase.fn()
		assert.Equal(t, err.Type, testCase.expected)
	}
}

func TestErrorUsefulMessage(t *testing.T) {
	s := Invalid("foo", "bar", "deet").Error()
	for _, part := range []string{"foo", "bar", "deet", ErrorTypeInvalid.String()} {
		assert.Contains(t, s, part)
	}

	type complicated struct {
		Baz   int
		Qux   string
		Inner interface{}
		KV    map[string]int
	}
	s = Invalid(
		"foo",
		&complicated{
			Baz:   1,
			Qux:   "aoeu",
			Inner: &complicated{Qux: "asdf"},
			KV:    map[string]int{"Billy": 2},
		},
		"detail",
	).Error()
	for _, part := range []string{
		"foo", ErrorTypeInvalid.String(),
		"Baz", "Qux", "Inner", "KV", "detail",
		"1", "aoeu", "asdf", "Billy", "2",
	} {
		assert.Contains(t, s, part)
	}
}
