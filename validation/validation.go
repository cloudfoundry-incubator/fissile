package validation

import "strconv"

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
