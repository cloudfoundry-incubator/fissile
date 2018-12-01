package kube

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
)

const (
	// RoleNameLabel is the recommended kube label to specify the rolename
	RoleNameLabel = "app.kubernetes.io/component"
	// AppNameLabel is to add contextual information in distributed tracing for Istio
	AppNameLabel = "app"
	// AppVersionLable is to indicate the version of app. It is used to add contextual information in
	// distributed tracing and the metric telemetry collected by Istio
	AppVersionLable = "version"
	// VolumeStorageClassAnnotation is the annotation label for storage/v1beta1/StorageClass
	VolumeStorageClassAnnotation = "volume.beta.kubernetes.io/storage-class"
)

func newTypeMeta(apiVersion, kind string, modifiers ...helm.NodeModifier) *helm.Mapping {
	mapping := helm.NewMapping("apiVersion", apiVersion, "kind", kind)
	mapping.Set(modifiers...)
	return mapping
}

func newSelector(role *model.InstanceGroup, settings ExportSettings) *helm.Mapping {
	// XXX We need to match on legacy RoleNameLabel to maintain upgradability of stateful sets
	matchLabels := helm.NewMapping("skiff-role-name", role.Name)
	if settings.IstioComplied && role.HasTag(model.RoleTagIstioManaged) {
		matchLabels.Add(AppNameLabel, role.Name)
		if settings.CreateHelmChart {
			matchLabels.Add(AppVersionLable, `{{ default .Chart.Version .Chart.AppVersion | quote }}`)
		}
	}
	meta := helm.NewMapping("matchLabels", matchLabels)

	return meta
}

// newKubeConfig sets up generic a Kube config structure with minimal metadata
func newKubeConfig(settings ExportSettings, apiVersion, kind, name string, modifiers ...helm.NodeModifier) *helm.Mapping {
	labels := helm.NewMapping(RoleNameLabel, name) // "app.kubernetes.io/component"
	if settings.IstioComplied {
		labels.Add(AppNameLabel, name)
	}

	if settings.CreateHelmChart {
		// XXX skiff-role-name is the legacy RoleNameLabel and will be removed in a future release
		labels.Add("skiff-role-name", name)
		// app: XXX is used by Istio
		labels.Add("app.kubernetes.io/instance", `{{ .Release.Name | quote }}`)
		labels.Add("app.kubernetes.io/managed-by", `{{ .Release.Service | quote }}`)
		labels.Add("app.kubernetes.io/name", `{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" | quote }}`)
		labels.Add("app.kubernetes.io/version", `{{ default .Chart.Version .Chart.AppVersion | quote }}`)
		// labels.Add("app.kubernetes.io/part-of", `???`)
		labels.Add("helm.sh/chart", `{{ printf "%s-%s" .Chart.Name (.Chart.Version | replace "+" "_") | quote }}`)
		if settings.IstioComplied && kind == "StatefulSet" {
			labels.Add(AppVersionLable, `{{ default .Chart.Version .Chart.AppVersion | quote }}`)
		}
	}

	config := newTypeMeta(apiVersion, kind, modifiers...)
	config.Add("metadata", helm.NewMapping("name", name, "labels", labels))

	return config
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
		"bosh", helm.NewMapping("instance_groups", helm.NewList()),
		"env", helm.NewMapping(),
		"sizing", helm.NewMapping(),
		"secrets", helm.NewMapping(),
		"services", helm.NewMapping(
			"loadbalanced", false))
}
