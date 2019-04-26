package validation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePortOk(t *testing.T) {
	assert := assert.New(t)

	errs := ValidatePort("1", "")

	assert.NotNil(errs)
	assert.Empty(errs)
}

func TestValidatePortOutOfRange(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"-1", "0", "65536", "70000",
	}
	for _, port := range cases {
		errs := ValidatePort(port, "field")
		assert.NotNil(errs)
		assert.Len(errs, 1)
		assert.Contains(
			errs.ErrorStrings(),
			fmt.Sprintf("field: Invalid value: %s: must be between 1 and 65535, inclusive", port))
	}
}

func TestValidatePortBadSyntax(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"0-1", "q", "1.5",
	}
	for _, port := range cases {
		errs := ValidatePort(port, "field")
		assert.NotNil(errs)
		assert.Len(errs, 1)
		assert.Contains(errs.Error(), `invalid syntax`)
	}
}

func TestValidateProtocolOk(t *testing.T) {
	assert := assert.New(t)

	errs := ValidateProtocol("TCP", "")
	assert.NotNil(errs)
	assert.Empty(errs)

	errs = ValidateProtocol("UDP", "")
	assert.NotNil(errs)
	assert.Empty(errs)
}

func TestValidateProtocolOutOfRange(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"tcp", "udp", "-1", "whatever",
	}
	for _, proto := range cases {
		errs := ValidateProtocol(proto, "field")
		assert.NotNil(errs)
		assert.Len(errs, 1)
		assert.EqualError(errs,
			fmt.Sprintf(`field: Unsupported value: "%s": supported values: TCP, UDP`,
				proto))
	}
}

func TestValidatePortRangeOk(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"1", "1-2",
	}
	for i, arange := range cases {
		first, last, errs := ValidatePortRange(arange, "")
		assert.NotNil(errs)
		assert.Empty(errs)
		assert.Equal(first, 1)
		assert.Equal(last, i+1)
	}
}

func TestValidatePortRangeOutOfRange(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		therange string
		badvalue []string
	}{
		{"0", []string{"0"}},
		{"65536", []string{"65536"}},
		{"70000", []string{"70000"}},
		{"1-65536", []string{"65536"}},
		{"1-70000", []string{"70000"}},
		{"65600-70000", []string{"65600", "70000"}},
	}
	for _, acase := range cases {
		_, _, errs := ValidatePortRange(acase.therange, "field")
		assert.NotNil(errs)
		assert.Len(errs, len(acase.badvalue))
		assert.Contains(errs.Error(), `must be between 1 and 65535, inclusive`)
		for _, bad := range acase.badvalue {
			assert.Contains(errs.Error(), fmt.Sprintf("field: Invalid value: %s:", bad))
		}
	}
}

func TestValidatePortRangeBadSyntax(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"-1", "q", "1.5", "1-",
	}
	for _, arange := range cases {
		_, _, errs := ValidatePortRange(arange, "field")
		assert.NotNil(errs)
		assert.Len(errs, 1)
		assert.Contains(errs.Error(), `invalid syntax`)
	}
}
