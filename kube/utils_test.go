package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
