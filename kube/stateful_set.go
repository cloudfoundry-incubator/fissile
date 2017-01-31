package kube

import (
	extra "k8s.io/client-go/1.5/pkg/apis/apps/v1alpha1"
)

func NewStatefulSet(role *ExtendedRole) *extra.PetSet {
	return &extra.PetSet{}
}
