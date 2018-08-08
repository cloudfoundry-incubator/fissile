package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/util"
)

// NewStatefulSet returns a stateful set and a list of services for the given role
func NewStatefulSet(role *model.InstanceGroup, settings ExportSettings, grapher util.ModelGrapher) (helm.Node, helm.Node, error) {
	// For each StatefulSet, we need two services -- one for the public (inside
	// the namespace) endpoint, and one headless service to control the pods.
	if role == nil {
		panic(fmt.Sprintf("No role given"))
	}

	podTemplate, err := NewPodTemplate(role, settings, grapher)
	if err != nil {
		return nil, nil, err
	}

	svcList, err := NewServiceList(role, true, settings)
	if err != nil {
		return nil, nil, err
	}

	claims := getVolumeClaims(role, settings.CreateHelmChart)

	spec := helm.NewMapping()
	spec.Add("serviceName", fmt.Sprintf("%s-set", role.Name))
	spec.Add("template", podTemplate)
	// "updateStrategy" is new in kube 1.7, so we don't add anything to non-helm configs
	// The default behaviour is "OnDelete"
	if settings.CreateHelmChart {
		strategy := helm.NewMapping("type", "RollingUpdate")
		spec.Add("updateStrategy", strategy, helm.Block("if "+minKubeVersion(1, 7)))
	}
	if len(claims) > 0 {
		spec.Add("volumeClaimTemplates", helm.NewNode(claims))
	}
	podManagementPolicy := "Parallel"
	if role.HasTag(model.RoleTagSequentialStartup) {
		podManagementPolicy = "OrderedReady"
	}
	spec.Add("podManagementPolicy", podManagementPolicy)

	statefulSet := newKubeConfig("apps/v1beta1", "StatefulSet", role.Name, helm.Comment(role.GetLongDescription()))
	statefulSet.Add("spec", spec)
	err = replicaCheck(role, statefulSet, svcList, settings)
	if err != nil {
		return nil, nil, err
	}
	err = generalCheck(role, statefulSet, settings)
	return statefulSet, svcList, err
}

// getVolumeClaims returns the list of persistent and shared volume claims from a role
func getVolumeClaims(role *model.InstanceGroup, createHelmChart bool) []helm.Node {
	var claims []helm.Node
	for _, volume := range role.Run.Volumes {
		var accessMode string
		switch volume.Type {
		case model.VolumeTypeHost, model.VolumeTypeNone, model.VolumeTypeEmptyDir:
			// These volume types don't have claims
			continue
		case model.VolumeTypePersistent:
			accessMode = "ReadWriteOnce"
		case model.VolumeTypeShared:
			accessMode = "ReadWriteMany"
		}
		storageClass := string(volume.Type)
		if createHelmChart {
			storageClass = fmt.Sprintf("{{ .Values.kube.storage_class.%s | quote }}", storageClass)
		}

		meta := helm.NewMapping("name", volume.Tag)
		annotationList := helm.NewMapping()
		annotationList.Add(VolumeStorageClassAnnotation, storageClass)
		for key, value := range volume.Annotations {
			annotationList.Add(key, value)
		}
		meta.Add("annotations", annotationList)

		var size string
		if createHelmChart {
			size = fmt.Sprintf("{{ .Values.sizing.%s.disk_sizes.%s }}G", makeVarName(role.Name), makeVarName(volume.Tag))
		} else {
			size = fmt.Sprintf("%dG", volume.Size)
		}

		spec := helm.NewMapping("accessModes", helm.NewList(accessMode))
		spec.Add("resources", helm.NewMapping("requests", helm.NewMapping("storage", size)))

		claim := helm.NewMapping("metadata", meta)
		claim.Add("spec", spec)

		claims = append(claims, claim)
	}
	return claims
}
