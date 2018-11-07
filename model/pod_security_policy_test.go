package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodSecurityPolicies(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	policies := PodSecurityPolicies()

	assert.Len(policies, 2)
	assert.Contains(policies, "privileged")
	assert.Contains(policies, "nonprivileged")
}

func TestValidPodSecurityPolicy(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.True(ValidPodSecurityPolicy("privileged"))
	assert.True(ValidPodSecurityPolicy("nonprivileged"))
	assert.False(ValidPodSecurityPolicy("bogus"))
}

func TestMergePodSecurityPolicies(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.Equal(MergePodSecurityPolicies("privileged", "privileged"), "privileged")
	assert.Equal(MergePodSecurityPolicies("privileged", "nonprivileged"), "privileged")
	assert.Equal(MergePodSecurityPolicies("nonprivileged", "privileged"), "privileged")
	assert.Equal(MergePodSecurityPolicies("nonprivileged", "nonprivileged"), "nonprivileged")
}

func TestLeastPodSecurityPolicy(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	assert.Equal(LeastPodSecurityPolicy(), "nonprivileged")
}
