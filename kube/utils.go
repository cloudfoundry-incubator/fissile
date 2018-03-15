package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/helm"
)

const (
	// RoleNameLabel is a thing
	RoleNameLabel = "skiff-role-name"
	// VolumeStorageClassAnnotation is the annotation label for storage/v1beta1/StorageClass
	VolumeStorageClassAnnotation = "volume.beta.kubernetes.io/storage-class"
)

func newTypeMeta(apiVersion, kind string, modifiers ...helm.NodeModifier) *helm.Mapping {
	mapping := helm.NewMapping("apiVersion", apiVersion, "kind", kind)
	mapping.Set(modifiers...)
	return mapping
}

func newObjectMeta(name string) *helm.Mapping {
	meta := helm.NewMapping("name", name)
	meta.Add("labels", helm.NewMapping(RoleNameLabel, name))
	return meta
}

func newSelector(name string) *helm.Mapping {
	meta := helm.NewMapping()
	meta.Add("matchLabels", helm.NewMapping(RoleNameLabel, name))
	return meta
}

// newKubeConfig sets up generic a Kube config structure with minimal metadata
func newKubeConfig(apiVersion, kind string, name string, modifiers ...helm.NodeModifier) *helm.Mapping {
	mapping := newTypeMeta(apiVersion, kind, modifiers...)
	mapping.Add("metadata", newObjectMeta(name))
	return mapping
}

func makeVarName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}

func minKubeVersion(major, minor int) string {
	ver := ".Capabilities.KubeVersion"
	// "Major > major || (Major == major && Minor >= minor)"
	// The int conversions are necessary because Major/Minor in KubeVersion are strings
	// The `trimSuffix` is necessary because the Minor version on GKE is currently "8+".
	// We would use `regexFind "[0-9]+"` but that isn't available in helm 2.6.2
	return fmt.Sprintf(`or (gt (int %s.Major) %d) (and (eq (int %s.Major) %d) (ge (%s.Minor | trimSuffix "+" | int) %d))`, ver, major, ver, major, ver, minor)
}
