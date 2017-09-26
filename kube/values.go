package kube

import (
	"fmt"
	"strings"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a Mapping with all default values for the Helm chart
func MakeValues(roleManifest *model.RoleManifest, defaults map[string]string) (helm.Node, error) {
	env := helm.NewMapping()
	for name, cv := range model.MakeMapOfVariables(roleManifest) {
		if strings.HasPrefix(name, "KUBE_SIZING_") {
			continue
		}
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			var value interface{}
			ok, value := cv.Value(defaults)
			if !ok {
				value = nil
			}
			comment := cv.Description
			if cv.Example != "" && cv.Example != value {
				comment += fmt.Sprintf("\nExample: %s", cv.Example)
			}
			env.Add(name, helm.NewNode(value, helm.Comment(comment)))
		}
	}
	env.Sort()

	sizing := helm.NewMapping()
	for _, role := range roleManifest.Roles {
		if role.IsDevRole() || role.Run.FlightStage == model.FlightStageManual {
			continue
		}

		entry := helm.NewMapping()
		var comment string
		if role.Run.Scaling.Min == role.Run.Scaling.Max {
			comment = fmt.Sprintf("The %s role cannot be scaled.", role.Name)
		} else {
			comment = fmt.Sprintf("The %s role can scale between %d and %d instances.",
				role.Name, role.Run.Scaling.Min, role.Run.Scaling.Max)
		}
		entry.Add("count", role.Run.Scaling.Min, helm.Comment(comment))
		entry.Add("memory", role.Run.Memory)
		entry.Add("vcpu_count", role.Run.VirtualCPUs)

		diskSizes := helm.NewMapping()
		for _, volume := range role.Run.PersistentVolumes {
			diskSizes.Add(makeVarName(volume.Tag), volume.Size)
		}
		for _, volume := range role.Run.SharedVolumes {
			diskSizes.Add(makeVarName(volume.Tag), volume.Size)
		}
		if len(diskSizes.Names()) > 0 {
			entry.Add("disk_sizes", diskSizes.Sort())
		}
		sizing.Add(makeVarName(role.Name), entry.Sort(), helm.Comment(role.GetLongDescription()))
	}

	kube := helm.NewMapping()
	kube.Add("external_ip", "192.168.77.77")
	kube.Add("storage_class", helm.NewMapping("persistent", "persistent", "shared", "shared"))

	values := helm.NewMapping()
	values.Add("env", env)
	values.Add("sizing", sizing.Sort())
	values.Add("kube", kube)

	return values, nil
}
