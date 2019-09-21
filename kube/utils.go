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
	// AppVersionLabel is to indicate the version of app. It is used to add contextual information in
	// distributed tracing and the metric telemetry collected by Istio
	AppVersionLabel = "version"
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
	if role.HasTag(model.RoleTagIstioManaged) && settings.CreateHelmChart {
		matchLabels.Add(AppNameLabel, role.Name, helm.Block("if .Values.config.use_istio"))
		matchLabels.Add(AppVersionLabel, `{{ default .Chart.Version .Chart.AppVersion | quote }}`, helm.Block("if .Values.config.use_istio"))
	}

	meta := helm.NewMapping("matchLabels", matchLabels)

	return meta
}

// ConfigBuilder sets up a generic Kube resource structure with minimal metadata.
type ConfigBuilder struct {
	settings   *ExportSettings
	apiVersion string
	kind       string
	name       string
	modifiers  []helm.NodeModifier

	err error
}

// NewConfigBuilder constructs a new ConfigBuilder.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

func (b *ConfigBuilder) setError(err error) {
	if b.err == nil {
		b.err = err
	} else {
		b.err = fmt.Errorf("%v, %v", b.err, err)
	}
}

// SetSettings sets the export settings to be used by the builder.
func (b *ConfigBuilder) SetSettings(settings *ExportSettings) *ConfigBuilder {
	b.settings = settings
	return b
}

// SetAPIVersion sets the kube API version of the resource to build.
func (b *ConfigBuilder) SetAPIVersion(apiVersion string) *ConfigBuilder {
	b.apiVersion = apiVersion
	return b
}

// SetConditionalAPIVersion sets the kube API version of the resource to build;
// if that API version is not available, use a fallback instead (to be
// compatible with older releases of kube).  If we are not building a helm
// chart, the desired API version is always used.
func (b *ConfigBuilder) SetConditionalAPIVersion(apiVersion, fallbackAPIVersion string) *ConfigBuilder {
	if b.settings == nil || !b.settings.CreateHelmChart {
		b.apiVersion = apiVersion
	} else {
		b.apiVersion = fmt.Sprintf(
			`{{ ternary "%s" "%s" (.Capabilities.APIVersions.Has "%s") }}`,
			apiVersion,
			fallbackAPIVersion,
			apiVersion)
	}
	return b
}

// SetKind sets the kubernetes resource kind of the resource to build.
func (b *ConfigBuilder) SetKind(kind string) *ConfigBuilder {
	b.kind = kind
	return b
}

// SetName sets the name of the resource to build.
func (b *ConfigBuilder) SetName(name string) *ConfigBuilder {
	if len(name) > 63 {
		b.setError(fmt.Errorf("kube name exceeds 63 characters"))
	}
	b.name = name
	return b
}

// SetNameHelmExpression sets the name of the resource to build as a Helm template expression.
func (b *ConfigBuilder) SetNameHelmExpression(name string) *ConfigBuilder {
	if b.settings == nil {
		b.setError(fmt.Errorf("name was set as a Helm expression before settings was set"))
	} else if !b.settings.CreateHelmChart {
		b.setError(fmt.Errorf("name is a Helm expression, but not creating a helm chart"))
	}
	if !strings.HasPrefix(name, "{{") {
		b.setError(fmt.Errorf(`name "%s" does not start with "{{"`, name))
	}
	if !strings.HasSuffix(name, "}}") {
		b.setError(fmt.Errorf(`name "%s" does not end with "}}"`, name))
	}
	if strings.ContainsRune(name, '\n') {
		b.setError(fmt.Errorf(`name "%q" contains new line characters`, name))
	}
	b.name = name
	return b
}

// AddModifier adds a modifier to be used by the builder.
func (b *ConfigBuilder) AddModifier(modifier helm.NodeModifier) *ConfigBuilder {
	b.modifiers = append(b.modifiers, modifier)
	return b
}

// Build the final kube resource.
func (b *ConfigBuilder) Build() (*helm.Mapping, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.settings == nil {
		return nil, fmt.Errorf("settings was not set")
	}

	labels := helm.NewMapping(RoleNameLabel, b.name) // "app.kubernetes.io/component"
	istioAppLabel := map[string]bool{
		"StatefulSet": true,
		"Deployment":  true,
		"Service":     true,
		"Pod":         true,
	}
	istioVersionLabel := map[string]bool{
		"StatefulSet": true,
		"Deployment":  true,
		"Pod":         true,
	}

	if b.settings.CreateHelmChart {
		// XXX skiff-role-name is the legacy RoleNameLabel and will be removed in a future release
		labels.Add("skiff-role-name", b.name)
		labels.Add("app.kubernetes.io/instance", `{{ .Release.Name | quote }}`)
		labels.Add("app.kubernetes.io/managed-by", `{{ .Release.Service | quote }}`)
		labels.Add("app.kubernetes.io/name", `{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" | quote }}`)
		labels.Add("app.kubernetes.io/version", `{{ default .Chart.Version .Chart.AppVersion | quote }}`)
		// labels.Add("app.kubernetes.io/part-of", `???`)
		labels.Add("helm.sh/chart", `{{ printf "%s-%s" .Chart.Name (.Chart.Version | replace "+" "_") | quote }}`)
		if istioAppLabel[b.kind] {
			labels.Add(AppNameLabel, b.name, helm.Block("if .Values.config.use_istio"))
		}
		if istioVersionLabel[b.kind] {
			labels.Add(AppVersionLabel, `{{ default .Chart.Version .Chart.AppVersion | quote }}`, helm.Block("if .Values.config.use_istio"))
		}
	}

	config := newTypeMeta(b.apiVersion, b.kind, b.modifiers...)
	config.Add("metadata", helm.NewMapping("name", b.name, "labels", labels))

	return config, nil
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

	return helm.NewMapping(
		"kube", helm.NewMapping(
			"external_ips", helm.NewList(),
			"secrets_generation_counter", helm.NewNode(1, helm.Comment("Increment this counter to rotate all generated secrets")),
			"storage_class", helm.NewMapping("persistent", "persistent", "shared", "shared"),
			"psp", helm.NewMapping(),
			"hostpath_available", helm.NewNode(false, helm.Comment("Whether HostPath volume mounts are available")),
			"registry", helm.NewMapping(
				"hostname", "docker.io",
				"username", "",
				"password", ""),
			"organization", "",
			"auth", nil,
			"limits", helm.NewMapping(
				"nproc", helm.NewMapping(
					"hard", "2048",
					"soft", "1024",
				),
			),
		),
		"config", helm.NewMapping(
			"HA", helm.NewNode(false, helm.Comment("Flag to activate high-availability mode")),
			"HA_strict", helm.NewNode(true, helm.Comment("Flag to verify instance counts against HA minimums")),
			"memory", helm.NewNode(helm.NewMapping(
				"requests", helm.NewNode(false, helm.Comment("Flag to activate memory requests")),
				"limits", helm.NewNode(false, helm.Comment("Flag to activate memory limits")),
			), helm.Comment("Global memory configuration")),
			"cpu", helm.NewNode(helm.NewMapping(
				"requests", helm.NewNode(false, helm.Comment("Flag to activate cpu requests")),
				"limits", helm.NewNode(false, helm.Comment("Flag to activate cpu limits")),
			), helm.Comment("Global CPU configuration")),
			"use_istio", helm.NewNode(false, helm.Comment("Flag to specify whether to add Istio related annotations and labels"))),
		"bosh", helm.NewMapping("instance_groups", helm.NewList()),
		"env", helm.NewMapping(),
		"sizing", helm.NewMapping(),
		"secrets", helm.NewMapping(),
		"services", helm.NewMapping("loadbalanced", false),
		"ingress", helm.NewMapping("enabled", false))
}
