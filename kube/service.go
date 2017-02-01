package kube

import (
	apiv1 "k8s.io/client-go/1.5/pkg/api/v1"
)

// NewService creates a new k8s service in the given namespace
func NewService(namespace *apiv1.Namespace) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: apiv1.ObjectMeta{
			Name:      "",
			Namespace: namespace.ObjectMeta.Name,
		},
	}
}
