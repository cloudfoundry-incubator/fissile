package validation

import (
	"fmt"
	"strconv"

	roles "github.com/hpcloud/fissile/model"
)

// ValidateNonnegativeField validates that given value is not negative.
func ValidateNonnegativeField(value int64, field string) ErrorList {
	allErrs := ErrorList{}
	if value < 0 {
		allErrs = append(allErrs, Invalid(field, value, `must be greater than or equal to 0`))
	}
	return allErrs
}

// ValidatePort validates that given value is valid and in range 0 - 65535.
func ValidatePort(port string, field string) ErrorList {
	allErrs := ErrorList{}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		allErrs = append(allErrs, Invalid(field, port, `invalid syntax`))
	} else {

		if msg := IsValidPortNum(portInt); msg != nil {
			allErrs = append(allErrs, Invalid(field, portInt, msg.Error()))
		}
	}

	return allErrs
}

// ValidateProtocol validates that given value belongs to supported protocols
func ValidateProtocol(protocol string, field string) ErrorList {
	allErrs := ErrorList{}

	if err := IsValidProtocol(protocol); err != nil {
		allErrs = append(allErrs, NotSupported(field, protocol, []string{TCP, UDP}))

	}

	return allErrs
}

// ValidateRoleRun tests whether required fields in the RoleRun are set.
func ValidateRoleRun(run *roles.RoleRun) ErrorList {
	allErrs := ErrorList{}

	if run == nil {
		return append(allErrs, Required("run", ""))
	}

	allErrs = append(allErrs, ValidateNonnegativeField(int64(run.Memory), `run.memory`)...)
	allErrs = append(allErrs, ValidateNonnegativeField(int64(run.VirtualCPUs), `run.virtual-cpus`)...)

	for i := range run.ExposedPorts {

		if run.ExposedPorts[i].Name == "" {
			allErrs = append(allErrs, Required(`run.exposed-ports.name`, ""))
		}

		allErrs = append(allErrs, ValidatePort(run.ExposedPorts[i].External, fmt.Sprintf(`run.exposed-ports[%s].external`, run.ExposedPorts[i].Name))...)
		allErrs = append(allErrs, ValidatePort(run.ExposedPorts[i].Internal, fmt.Sprintf(`run.exposed-ports[%s].internal`, run.ExposedPorts[i].Name))...)

		allErrs = append(allErrs, ValidateProtocol(run.ExposedPorts[i].Protocol, fmt.Sprintf(`run.exposed-ports[%s].protocol`, run.ExposedPorts[i].Name))...)
	}

	return allErrs
}
