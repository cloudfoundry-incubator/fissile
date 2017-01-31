package kube

import (
	"k8s.io/client-go/1.5/pkg/api"
)

func NewService(namespace string) *api.Service {
	return &api.Service{}
}
