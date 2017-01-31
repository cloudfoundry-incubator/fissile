package kube

import (
	extra "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
)

func NewDeployment(role *ExtendedRole) *extra.Deployment {
	return &extra.Deployment{}
}
