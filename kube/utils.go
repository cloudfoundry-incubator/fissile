package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
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

// MakeBasicValues returns a Mapping with the default values that do not depend
// on any configuration.  This is exported so the tests from other packages can
// access them.
func MakeBasicValues() *helm.Mapping {

	psp := helm.NewMapping()
	for _, pspName := range model.PodSecurityPolicies() {
		psp.Add(pspName, nil)
	}

	return helm.NewMapping(
		"kube", helm.NewMapping(
			"external_ips", helm.NewList(),
			"secrets_generation_counter", helm.NewNode(1, helm.Comment("Increment this counter to rotate all generated secrets")),
			"storage_class", helm.NewMapping("persistent", "persistent", "shared", "shared"),
			"psp", psp,
			"hostpath_available", helm.NewNode(false, helm.Comment("Whether HostPath volume mounts are available")),
			"registry", helm.NewMapping(
				"hostname", "docker.io",
				"username", "",
				"password", ""),
			"organization", "",
			"auth", nil),
		"config", helm.NewMapping(
			"HA", helm.NewNode(false, helm.Comment("Flag to activate high-availability mode")),
			"memory", helm.NewNode(helm.NewMapping(
				"requests", helm.NewNode(false, helm.Comment("Flag to activate memory requests")),
				"limits", helm.NewNode(false, helm.Comment("Flag to activate memory limits")),
			), helm.Comment("Global memory configuration")),
			"cpu", helm.NewNode(helm.NewMapping(
				"requests", helm.NewNode(false, helm.Comment("Flag to activate cpu requests")),
				"limits", helm.NewNode(false, helm.Comment("Flag to activate cpu limits")),
			), helm.Comment("Global CPU configuration"))),
		"env", helm.NewMapping(),
		"sizing", helm.NewMapping(),
		"secrets", helm.NewMapping(),
		"services", helm.NewMapping(
			"loadbalanced", false))
}
