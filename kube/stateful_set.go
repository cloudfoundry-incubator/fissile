package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

const volumeStorageClassAnnotation = "volume.beta.kubernetes.io/storage-class"

// NewStatefulSet returns a stateful set for the given role
func NewStatefulSet(role *model.Role, settings *ExportSettings) (helm.Node, helm.Node, error) {
	// For each StatefulSet, we need two services -- one for the public (inside
	// the namespace) endpoint, and one headless service to control the pods.
	if role == nil {
		panic(fmt.Sprintf("No role given"))
	}

	podTemplate, err := NewPodTemplate(role, settings)
	if err != nil {
		return nil, nil, err
	}

	svcList, err := NewClusterIPServiceList(role, true, settings)
	if err != nil {
		return nil, nil, err
	}

	claims := getAllVolumeClaims(role, settings.CreateHelmChart)

	spec := helm.NewIntMapping("replicas", role.Run.Scaling.Min)
	spec.AddStr("serviceName", fmt.Sprintf("%s-set", role.Name))
	spec.Add("template", podTemplate)
	spec.Add("volumeClaimTemplates", helm.NewList(claims...))

	statefulSet := newKubeConfig("apps/v1beta1", "StatefulSet", role.Name)
	statefulSet.Add("spec", spec)

	return statefulSet.Sort(), svcList, nil
}

// getAllVolumeClaims returns the list of persistent and shared volume claims from a role
func getAllVolumeClaims(role *model.Role, createHelmChart bool) []helm.Node {
	claims := getVolumeClaims(role.Run.PersistentVolumes, "persistent", "ReadWriteOnce", createHelmChart)
	claims = append(claims, getVolumeClaims(role.Run.SharedVolumes, "shared", "ReadWriteMany", createHelmChart)...)
	return claims
}

// getVolumeClaims returns the list of persistent volume claims from a role
func getVolumeClaims(volumeDefinitions []*model.RoleRunVolume, storageClass string, accessMode string, createHealmChart bool) []helm.Node {
	if createHealmChart {
		storageClass = fmt.Sprintf("{{ .Values.kube.storage_class.%s | quote }}", storageClass)
	}

	var claims []helm.Node
	for _, volume := range volumeDefinitions {
		meta := helm.NewStrMapping("name", volume.Tag)
		meta.Add("annotations", helm.NewStrMapping(volumeStorageClassAnnotation, storageClass))

		size := fmt.Sprintf("%dG", volume.Size)

		spec := helm.NewMapping("accessModes", helm.NewStrList(accessMode))
		spec.Add("resources", helm.NewMapping("requests", helm.NewStrMapping("storage", size)))

		claim := helm.NewMapping("metadata", meta)
		claim.Add("spec", spec)

		claims = append(claims, claim)
	}
	return claims
}
