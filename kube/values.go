package kube

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/fissile/helm"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
)

func formattedExample(example string) string {
	if len(example) > 0 {
		if strings.ContainsRune(example, '\n') {
			example = strings.TrimRight(example, "\n")
			example = strings.Join(strings.Split(example, "\n"), "\n  ")
			example = fmt.Sprintf("\nExample:\n  %s", example)
		} else {
			example = fmt.Sprintf("\nExample: %q", example)
		}
	}
	return example
}

// MakeValues returns a Mapping with all default values for the Helm chart.
func MakeValues(settings ExportSettings) helm.Node {
	values := MakeBasicValues()
	env := helm.NewMapping()
	secrets := helm.NewMapping()
	generated := helm.NewMapping()

	for name, cv := range model.MakeMapOfVariables(settings.RoleManifest) {
		if strings.HasPrefix(name, "KUBE_SIZING_") || cv.CVOptions.Type == model.CVTypeEnv {
			continue
		}
		// Immutable secrets that are generated cannot be overridden by the user
		// and any default value would always be ignored.
		if cv.CVOptions.Immutable && cv.Type != "" {
			continue
		}

		var value interface{}
		if !cv.CVOptions.Secret || cv.Type == "" {
			var ok bool
			if ok, value = cv.Value(); !ok {
				value = nil
			}
		}
		comment := cv.CVOptions.Description
		if cv.CVOptions.Secret {
			thisValue := "This value"
			if cv.Type != "" {
				comment += "\n" + thisValue + " uses a generated default."
				thisValue = "It"
			}
			if cv.CVOptions.Immutable {
				comment += "\n" + thisValue + " is immutable and must not be changed once set."
			}
			comment += formattedExample(cv.CVOptions.Example)
			if cv.Type == "" {
				secrets.Add(name, helm.NewNode(value, helm.Comment(comment)))
			} else {
				generated.Add(name, helm.NewNode(value, helm.Comment(comment)))
			}
		} else {
			comment += formattedExample(cv.CVOptions.Example)
			env.Add(name, helm.NewNode(value, helm.Comment(comment)))
		}
	}
	secrets.Sort()
	secrets.Merge(generated.Sort())
	values.Add("secrets", secrets.Sort())
	values.Add("env", env.Sort())

	sizing := helm.NewMapping()
	sizing.Set(helm.Comment(strings.Join(strings.Fields(`
		The sizing section contains configuration to change each individual instance
		group.  Due to limitations on the allowable names, any dashes ("-") in the
		instance group names are replaced with underscores ("_").
	`), " ")))
	for _, instanceGroup := range settings.RoleManifest.InstanceGroups {
		if instanceGroup.Run.FlightStage == model.FlightStageManual {
			continue
		}

		entry := helm.NewMapping()

		if !instanceGroup.IsPrivileged() {
			entry.Add("capabilities", helm.NewList(),
				helm.Comment("Additional privileges can be specified here"))
		}

		var comment string
		it := fmt.Sprintf("The %s instance group", makeVarName(instanceGroup.Name))

		var feature string
		enabled := "enabled"

		if instanceGroup.IfFeature != "" {
			feature = instanceGroup.IfFeature
		} else if instanceGroup.DefaultFeature != "" {
			feature = instanceGroup.DefaultFeature
		} else if instanceGroup.UnlessFeature != "" {
			feature = instanceGroup.UnlessFeature
			enabled = "disabled"
		}
		if feature != "" {
			canBe := "can be"
			if settings.RoleManifest.Features[feature] {
				canBe = "is"
			}
			comment = fmt.Sprintf("%s %s %s by the %s feature.\n", it, canBe, enabled, feature)
			it = "It"
		}

		if instanceGroup.Run.Scaling.Min == instanceGroup.Run.Scaling.Max {
			comment += fmt.Sprintf("%s cannot be scaled.", it)
		} else {
			comment += fmt.Sprintf("%s can scale between %d and %d instances.",
				it, instanceGroup.Run.Scaling.Min, instanceGroup.Run.Scaling.Max)

			if instanceGroup.Run.Scaling.MustBeOdd {
				comment += "\nThe instance count must be an odd number (not divisible by 2)."
			}
			if instanceGroup.Run.Scaling.HA != instanceGroup.Run.Scaling.Min {
				comment += fmt.Sprintf("\nFor high availability it needs at least %d instances.",
					instanceGroup.Run.Scaling.HA)
			}
		}
		entry.Add("count", instanceGroup.Run.Scaling.Min, helm.Comment(comment))
		if settings.UseMemoryLimits {
			var request helm.Node
			if instanceGroup.Run.Memory.Request == nil {
				request = helm.NewNode(nil)
			} else {
				request = helm.NewNode(int(*instanceGroup.Run.Memory.Request))
			}
			var limit helm.Node
			if instanceGroup.Run.Memory.Limit == nil {
				limit = helm.NewNode(nil)
			} else {
				limit = helm.NewNode(int(*instanceGroup.Run.Memory.Limit))
			}

			entry.Add("memory", helm.NewMapping(
				"request", request,
				"limit", limit),
				helm.Comment("Unit [MiB]"))
		}
		if settings.UseCPULimits {
			var request helm.Node
			if instanceGroup.Run.CPU.Request == nil {
				request = helm.NewNode(nil)
			} else {
				request = helm.NewNode(1000. * *instanceGroup.Run.CPU.Request)
			}
			var limit helm.Node
			if instanceGroup.Run.CPU.Limit == nil {
				limit = helm.NewNode(nil)
			} else {
				limit = helm.NewNode(1000. * *instanceGroup.Run.CPU.Limit)
			}

			entry.Add("cpu", helm.NewMapping(
				"request", request,
				"limit", limit),
				helm.Comment("Unit [millicore]"))
		}

		diskSizes := helm.NewMapping()
		for _, volume := range instanceGroup.Run.Volumes {
			switch volume.Type {
			case model.VolumeTypePersistent, model.VolumeTypeShared:
				diskSizes.Add(makeVarName(volume.Tag), volume.Size)
			}
		}
		if len(diskSizes.Names()) > 0 {
			entry.Add("disk_sizes", diskSizes.Sort())
		}
		ports := helm.NewMapping()
		for _, job := range instanceGroup.JobReferences {
			for _, port := range job.ContainerProperties.BoshContainerization.Ports {
				config := helm.NewMapping()
				if port.PortIsConfigurable {
					config.Add("port", port.ExternalPort)
				}
				if port.CountIsConfigurable {
					config.Add("count", port.Count)
				}
				if len(config.Names()) > 0 {
					ports.Add(makeVarName(port.Name), config)
				}
			}
		}
		if len(ports.Names()) > 0 {
			entry.Add("ports", ports.Sort())
		}

		entry.Add("affinity", helm.NewMapping(), helm.Comment("Node affinity rules can be specified here"))

		sizing.Add(makeVarName(instanceGroup.Name), entry.Sort(), helm.Comment(instanceGroup.GetLongDescription()))
	}
	values.Add("sizing", sizing.Sort())

	registry := settings.Registry
	if registry == "" {
		// Use DockerHub as default registry because our templates will *always* include
		// the registry in image names: $REGISTRY/$ORG/$IMAGE:$TAG, and that doesn't work
		// if registry is blank.
		registry = "docker.io"
	}
	// Override registry settings
	kube := values.Get("kube").(*helm.Mapping)
	kube.Add("registry", helm.NewMapping(
		"hostname", registry,
		"username", settings.Username,
		"password", settings.Password))
	kube.Add("organization", settings.Organization)
	if settings.AuthType != "" {
		kube.Add("auth", settings.AuthType)
	}
	psps := helm.NewMapping()
	for pspName := range settings.RoleManifest.Configuration.Authorization.PodSecurityPolicies {
		psps.Add(pspName, nil)
	}
	kube.Add("psp", psps.Sort())
	kube.Add(
		"limits", helm.NewMapping(
			"nproc", helm.NewMapping(
				"hard", "",
				"soft", "",
			),
		),
	)
	kube.Sort()

	enable := helm.NewMapping()
	for name, value := range settings.RoleManifest.Features {
		var ifFeatures []string
		var unlessFeatures []string
		for _, instanceGroup := range settings.RoleManifest.InstanceGroups {
			if instanceGroup.IfFeature == name {
				ifFeatures = append(ifFeatures, makeVarName(instanceGroup.Name))
			} else if instanceGroup.DefaultFeature == name {
				ifFeatures = append(ifFeatures, makeVarName(instanceGroup.Name))
			} else if instanceGroup.UnlessFeature == name {
				unlessFeatures = append(unlessFeatures, makeVarName(instanceGroup.Name))
			}
		}
		var comment string
		if len(ifFeatures) > 0 {
			comment = fmt.Sprintf("The %s feature enables these instance groups: %s",
				name, util.WordList(ifFeatures, "and"))
		}
		if len(unlessFeatures) > 0 {
			if len(comment) == 0 {
				comment = fmt.Sprintf("The %s feature disables these instance groups: %s",
					name, util.WordList(unlessFeatures, "and"))
			} else {
				comment += fmt.Sprintf("\nIt disables these instance groups: %s",
					util.WordList(unlessFeatures, "and"))
			}
		}
		enable.Add(name, value, helm.Comment(comment))
	}
	values.Add("enable", enable.Sort())

	ingress := helm.NewMapping()
	ingress.Add("annotations", helm.NewMapping(), helm.Comment("ingress.annotations allows specifying custom ingress annotations that gets merged to the default annotations."))
	ingress.Add("enabled", false, helm.Comment("ingress.enabled enables ingress support - working ingress controller necessary."))
	ingress.Add("tls", helm.NewMapping(), helm.Comment("ingress.tls.crt and ingress.tls.key, when specified, are used by the TLS secret for the Ingress resource."))
	values.Add("ingress", ingress.Sort())

	return values
}
