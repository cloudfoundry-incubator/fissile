package model

// This part of the model encapsulates fissile's knowledge of pod
// security policies (psp). fissile actually does not know about any
// concrete psp at all. What it knows/has are abstract names for
// levels of privilege the writer of a manifest can assign to
// jobs. The operator deploying the chart resulting from such a
// manifest is then responsible for mapping the abstract names/levels
// to concrete policies implementing them.

// Pod security policy constants
const (
	PodSecurityPolicyNonPrivileged = "nonprivileged"
	PodSecurityPolicyPrivileged    = "privileged"
)

// PodSecurityPolicies returns the names of the pod security policies
// usable in fissile manifests
func PodSecurityPolicies() []string {
	return []string{
		PodSecurityPolicyNonPrivileged,
		PodSecurityPolicyPrivileged,
	}
}

// ValidPodSecurityPolicy checks if the argument is the name of a
// fissile pod security policy
func ValidPodSecurityPolicy(name string) bool {
	for _, legal := range PodSecurityPolicies() {
		if name == legal {
			return true
		}
	}
	return false
}

// MergePodSecurityPolicies takes two policies (names) and returns the
// policy (name) representing the union of their privileges.
func MergePodSecurityPolicies(policyA, policyB string) string {
	if policyA == PodSecurityPolicyPrivileged || policyB == PodSecurityPolicyPrivileged {
		return PodSecurityPolicyPrivileged
	}
	return PodSecurityPolicyNonPrivileged
}

// LeastPodSecurityPolicy returns the name of the bottom-level pod
// security policy (least-privileged)
func LeastPodSecurityPolicy() string {
	return PodSecurityPolicyNonPrivileged
}
