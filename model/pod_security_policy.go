package model

// This part of the model encapsulates fissile's knowledge of pod
// security policies (psp). fissile actually does not know about any
// concrete psp at all. What it knows/has are abstract names for
// levels of privilege the writer of a manifest can assign to
// jobs. The operator deploying the chart resulting from such a
// manifest is then responsible for mapping the abstract names/levels
// to concrete policies implementing them.

import (
	"fmt"
)

const (
	// PodSecurityPolicyNonPrivileged is a backwards compatibility marker to
	// indicate an instance group does not require a privileged PSP
	PodSecurityPolicyNonPrivileged = "nonprivileged"
	// PodSecurityPolicyPrivileged marks an instance group as requiring a
	// privileged PSP
	PodSecurityPolicyPrivileged = "privileged"
)

// PodSecurityPolicy defines a pod security policy
type PodSecurityPolicy struct {
	Definition interface{}
}

// UnmarshalYAML implements the yaml.v2/Unmarshaler interface.
// We don't want to describe the whole PodSecurityPolicySpec, so just hold all
// of it in an interface{} instead
func (policy *PodSecurityPolicy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := unmarshal(&policy.Definition)
	if err != nil {
		return fmt.Errorf("Error unmarshalling PSP spec: %v", err)
	}
	return nil
}
