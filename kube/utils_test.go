package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePortRange(t *testing.T) {
	assert := assert.New(t)

	samples := []struct {
		name  string
		input string
		min   int32
		max   int32
		err   string
	}{
		{
			name:  "single port",
			input: "1234",
			min:   1234,
			max:   1234,
		},
		{
			name:  "port range",
			input: "1234-5678",
			min:   1234,
			max:   5678,
		},
		{
			name:  "invalid number",
			input: "garbage",
			err:   `Port invalid number has invalid description port garbage: strconv.ParseInt: parsing "garbage": invalid syntax`,
		},
		{
			name:  "empty port range",
			input: "",
			err:   `Port empty port range has invalid description port : strconv.ParseInt: parsing "": invalid syntax`,
		},
		{
			name:  "invalid start port",
			input: "trash-1",
			err:   `Port invalid start port has invalid description starting port trash: strconv.ParseInt: parsing "trash": invalid syntax`,
		},
		{
			name:  "invalid end port",
			input: "1-junk",
			err:   `Port invalid end port has invalid description ending port junk: strconv.ParseInt: parsing "junk": invalid syntax`,
		},
		{
			name:  "inverted port range",
			input: "5678-1234",
			err:   `Port inverted port range has invalid description port range 5678-1234`,
		},
	}

	for _, sample := range samples {
		min, max, err := parsePortRange(sample.input, sample.name, "description")
		if sample.err != "" {
			assert.EqualError(err, sample.err, "Expected error in case %s", sample.name)
		} else if assert.NoError(err, "Unexpected error in case %s", sample.name) {
			assert.Equal(sample.min, min, "Unexpected start port in %s", sample.name)
			assert.Equal(sample.max, max, "Unexpected end port in %s", sample.name)
		}
	}
}
