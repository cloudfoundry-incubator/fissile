package kube

import (
	"github.com/hpcloud/fissile/model"

	extra "k8s.io/client-go/1.5/pkg/apis/apps/v1alpha1"
)

func NewStatefulSet(role *model.Role) *extra.PetSet {
	return &extra.PetSet{}
}
