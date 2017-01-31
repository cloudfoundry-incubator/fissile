package kube

import (
	extra "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
)

func NewJob(role *ExtendedRole) *extra.Job {
	return &extra.Job{}
}
