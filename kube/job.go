package kube

import (
	"github.com/hpcloud/fissile/model"

	extra "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
)

// NewJob creates a new Job for the given role
func NewJob(role *model.Role) *extra.Job {
	return &extra.Job{}
}
