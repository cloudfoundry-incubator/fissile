package kube

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

// NewNamespace creates a k8s namespace with the given name
func NewNamespace(namespace string) *apiv1.Namespace {
	return &apiv1.Namespace{
		ObjectMeta: apiv1.ObjectMeta{
			Name: namespace,
		},
	}
}
