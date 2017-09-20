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
		min   int
		max   int
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
			err:   `Port invalid number has invalid description port garbage: strconv.Atoi: parsing "garbage": invalid syntax`,
		},
		{
			name:  "empty port range",
			input: "",
			err:   `Port empty port range has invalid description port : strconv.Atoi: parsing "": invalid syntax`,
		},
		{
			name:  "invalid start port",
			input: "trash-1",
			err:   `Port invalid start port has invalid description starting port trash: strconv.Atoi: parsing "trash": invalid syntax`,
		},
		{
			name:  "invalid end port",
			input: "1-junk",
			err:   `Port invalid end port has invalid description ending port junk: strconv.Atoi: parsing "junk": invalid syntax`,
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

func TestGetPortInfo(t *testing.T) {
	assert := assert.New(t)

	samples := []struct {
		desc     string
		name     string
		min      int
		max      int
		err      string
		expected []portInfo
	}{
		{
			desc: "single port",
			name: "single-port",
			min:  123,
			max:  123,
			expected: []portInfo{
				{
					name: "single-port",
					port: 123,
				},
			},
		},
		{
			desc: "long port names should be fixed",
			name: "port-with-a-very-long-name",
			min:  22,
			max:  22,
			expected: []portInfo{
				{
					name: "port-wi40a84c6a",
					port: 22,
				},
			},
		},
		{
			desc: "Odd port names should be sanitized",
			name: "-!port@NAME$--$here#-%Ｕｎｉｃｏｄｅ*",
			min:  1234,
			max:  1234,
			expected: []portInfo{{
				name: "portNAME-here",
				port: 1234,
			}},
		},
		{
			desc: "Invalid port names should be rejected",
			name: "-!-@-#-$-%-^-&-*-(-)-",
			min:  1234,
			max:  1234,
			err:  "Port name -!-@-#-$-%-^-&-*-(-)- does not contain any letters or digits",
		},
		{
			desc: "Port range should be supported",
			name: "port-range",
			min:  1234,
			max:  1236,
			expected: []portInfo{
				{
					name: "port-range-0",
					port: 1234,
				},
				{
					name: "port-range-1",
					port: 1235,
				},
				{
					name: "port-range-2",
					port: 1236,
				},
			},
		},
		{
			desc: "Port range with long name should be renamed",
			name: "long-port-range-name",
			min:  5678,
			max:  5679,
			expected: []portInfo{
				{
					name: "long-28321630-0",
					port: 5678,
				},
				{
					name: "long-28321630-1",
					port: 5679,
				},
			},
		},
	}

	for _, sample := range samples {
		actual, err := getPortInfo(sample.name, sample.min, sample.max)
		if sample.err != "" {
			assert.EqualError(err, sample.err, "expected error with port %s", sample.desc)
		} else {
			assert.Equal(sample.expected, actual, "unexpected results for port %s", sample.desc)
		}
	}
}
