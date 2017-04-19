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
		assert.Contains(errs.Errors(), `must be between 1 and 65535, inclusive`)
		assert.Contains(errs.Errors(), fmt.Sprintf("field: Invalid value: %s:", port))
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
		assert.Contains(errs.Errors(), `invalid syntax`)
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
		assert.Equal(
			fmt.Sprintf(`field: Unsupported value: "%s": supported values: TCP, UDP`,
				proto),
			errs.Errors())
	}
}

func TestValidatePortRangeOk(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"1", "1-2",
	}
	for _, arange := range cases {
		errs := ValidatePortRange(arange, "")
		assert.NotNil(errs)
		assert.Empty(errs)
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
		errs := ValidatePortRange(acase.therange, "field")
		assert.NotNil(errs)
		assert.Len(errs, len(acase.badvalue))
		assert.Contains(errs.Errors(), `must be between 1 and 65535, inclusive`)
		for _, bad := range acase.badvalue {
			assert.Contains(errs.Errors(), fmt.Sprintf("field: Invalid value: %s:", bad))
		}
	}
}

func TestValidatePortRangeBadSyntax(t *testing.T) {
	assert := assert.New(t)

	cases := []string{
		"-1", "q", "1.5", "1-",
	}
	for _, arange := range cases {
		errs := ValidatePortRange(arange, "field")
		assert.NotNil(errs)
		assert.Len(errs, 1)
		assert.Contains(errs.Errors(), `invalid syntax`)
	}
}
