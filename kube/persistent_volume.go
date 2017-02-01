package kube

import (
	"github.com/hpcloud/fissile/model"

	"k8s.io/client-go/1.5/pkg/api"
)

func NewPersistentVolume(volume *model.RoleRunVolume) *api.PersistentVolume {
	return &api.PersistentVolume{}
}
