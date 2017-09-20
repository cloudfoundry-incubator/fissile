package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// NewStatefulSet returns a stateful set and a list of services for the given role
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

	spec := helm.NewEmptyMapping()
	spec.Add("serviceName", fmt.Sprintf("%s-set", role.Name))
	spec.AddNode("template", podTemplate)
	spec.AddNode("volumeClaimTemplates", helm.NewNodeList(claims...))

	statefulSet := newKubeConfig("apps/v1beta1", "StatefulSet", role.Name, helm.Comment(role.GetLongDescription()))
	replicaCheck(role, statefulSet, spec, svcList, settings)
	statefulSet.AddNode("spec", spec.Sort())

	return statefulSet.Sort(), svcList, nil
}

// getAllVolumeClaims returns the list of persistent and shared volume claims from a role
func getAllVolumeClaims(role *model.Role, createHelmChart bool) []helm.Node {
	claims := getVolumeClaims(role, role.Run.PersistentVolumes, "persistent", "ReadWriteOnce", createHelmChart)
	claims = append(claims, getVolumeClaims(role, role.Run.SharedVolumes, "shared", "ReadWriteMany", createHelmChart)...)
	return claims
}

// getVolumeClaims returns the list of persistent volume claims from a role
func getVolumeClaims(role *model.Role, volumeDefinitions []*model.RoleRunVolume, storageClass string, accessMode string, createHelmChart bool) []helm.Node {
	if createHelmChart {
		storageClass = fmt.Sprintf("{{ .Values.kube.storage_class.%s | quote }}", storageClass)
	}

	var claims []helm.Node
	for _, volume := range volumeDefinitions {
		meta := helm.NewMapping("name", volume.Tag)
		meta.AddNode("annotations", helm.NewMapping(VolumeStorageClassAnnotation, storageClass))

		var size string
		if createHelmChart {
			size = fmt.Sprintf("{{ .Values.sizing.%s.disk_sizes.%s }}G", makeVarName(role.Name), makeVarName(volume.Tag))
		} else {
			size = fmt.Sprintf("%dG", volume.Size)
		}

		spec := helm.NewNodeMapping("accessModes", helm.NewList(accessMode))
		spec.AddNode("resources", helm.NewNodeMapping("requests", helm.NewMapping("storage", size)))

		claim := helm.NewNodeMapping("metadata", meta)
		claim.AddNode("spec", spec)

		claims = append(claims, claim)
	}
	return claims
}
