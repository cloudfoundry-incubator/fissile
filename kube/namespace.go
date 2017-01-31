package kube

import (
	"k8s.io/client-go/1.5/pkg/api"
)

func NewNamespace(namespace string) *api.Namespace {
	return &api.Namespace{}
}
