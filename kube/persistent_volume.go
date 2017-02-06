package kube

import (
	"github.com/hpcloud/fissile/model"

	"k8s.io/client-go/pkg/api"
)

// NewPersistentVolume creates a PersistentVolume for the given RoleRunVolume
func NewPersistentVolume(volume *model.RoleRunVolume) *api.PersistentVolume {
	return &api.PersistentVolume{}
}
