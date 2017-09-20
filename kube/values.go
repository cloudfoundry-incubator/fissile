package kube

import (
	"fmt"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
)

// MakeValues returns a Mapping with all default values for the Helm chart
func MakeValues(roleManifest *model.RoleManifest, defaults map[string]string) (helm.Node, error) {
	env := helm.NewEmptyMapping()
	for name, cv := range model.MakeMapOfVariables(roleManifest) {
		if !cv.Secret || cv.Generator == nil || cv.Generator.Type != model.GeneratorTypePassword {
			ok, value := cv.Value(defaults)
			if !ok {
				value = "~"
			}
			env.AddNode(name, helm.NewScalar(value, helm.Comment(cv.Description)))
		}
	}
	env.Sort()

	sizing := helm.NewEmptyMapping()
	for _, role := range roleManifest.Roles {
		if role.IsDevRole() || role.Run.FlightStage == model.FlightStageManual {
			continue
		}
		comment := fmt.Sprintf("This is dummy description for the %s role.", role.Name)
		comment += " It should come from the role manifest (which currently doesn't have one)."
		comment += " This is dummy text."
		entry := helm.NewEmptyMapping(helm.Comment(comment))

		if role.Run.Scaling.Min == role.Run.Scaling.Max {
			comment = fmt.Sprintf("The %s role cannot be scaled", role.Name)
		} else {
			comment = fmt.Sprintf("The %s role can scale between %d and %d instance",
				role.Name, role.Run.Scaling.Min, role.Run.Scaling.Max)
		}
		entry.AddInt("count", role.Run.Scaling.Min, helm.Comment(comment))
		entry.AddInt("memory", role.Run.Memory)
		entry.AddInt("vcpu_count", role.Run.VirtualCPUs)

		diskSizes := helm.NewEmptyMapping()
		for _, volume := range role.Run.PersistentVolumes {
			diskSizes.AddInt(makeVarName(volume.Tag), volume.Size)
		}
		for _, volume := range role.Run.SharedVolumes {
			diskSizes.AddInt(makeVarName(volume.Tag), volume.Size)
		}
		if len(diskSizes.Names()) > 0 {
			entry.AddNode("disk_sizes", diskSizes.Sort())
		}
		sizing.AddNode(makeVarName(role.Name), entry)
	}

	kube := helm.NewEmptyMapping()
	kube.AddNode("external_ip", helm.NewScalar("192.168.77.77"))
	kube.AddNode("storage_class", helm.NewMapping("persistent", "persistent", "shared", "shared"))

	values := helm.NewEmptyMapping()
	values.AddNode("env", env)
	values.AddNode("sizing", sizing.Sort())
	values.AddNode("kube", kube)

	return values, nil
}
