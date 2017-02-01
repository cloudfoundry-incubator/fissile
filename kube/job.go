package kube

import (
	"github.com/hpcloud/fissile/model"

	extra "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"
)

func NewJob(role *model.Role) *extra.Job {
	return &extra.Job{}
}
