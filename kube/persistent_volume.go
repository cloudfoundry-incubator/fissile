package kube

import (
	"k8s.io/client-go/1.5/pkg/api"
)

func NewPersistentVolume(volume *RoleRunVolume) *api.PersistentVolume {
	return &api.PersistentVolume{}
}
