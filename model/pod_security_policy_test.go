package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestPodSecurityPolicyPrivilegeEscalationAllowed(t *testing.T) {
	t.Parallel()
	samples := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "explicit true",
			input:    `{allowPrivilegeEscalation: true}`,
			expected: true,
		},
		{
			name:     "explicit false",
			input:    `{allowPrivilegeEscalation: false}`,
			expected: false,
		},
		{
			name:     "not set",
			input:    `{default: false}`,
			expected: false,
		},
	}
	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			t.Parallel()
			var policy PodSecurityPolicy
			err := yaml.Unmarshal([]byte(sample.input), &policy)
			require.NoError(t, err, "Failed to unmarshal input")
			assert.Equal(t, sample.expected, policy.PrivilegeEscalationAllowed())
		})
	}
}
