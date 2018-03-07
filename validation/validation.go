package validation

import (
	"regexp"
	"strconv"
)

// ValidateNonnegativeField validates that given value is not negative.
func ValidateNonnegativeField(value int64, field string) ErrorList {
	allErrs := ErrorList{}
	if value < 0 {
		allErrs = append(allErrs, Invalid(field, value, `must be greater than or equal to 0`))
	}
	return allErrs
}

// ValidateNonnegativeFieldFloat validates that given value is not negative.
func ValidateNonnegativeFieldFloat(value float64, field string) ErrorList {
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

// These are the two regular expressions to detect plain ports, and port ranges.
var (
	patternPortSingular = regexp.MustCompile(`^(\d+)$`)
	patternPortRange    = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

// ValidatePortRange validates that the given value is a valid port
// range, with its elements in range 0 - 65535.  It accepts singular
// ports P, and port ranges of the form N-M.
func ValidatePortRange(portRange string, field string) (firstPort, lastPort int, allErrs ErrorList) {
	allErrs = ErrorList{}
	if matches := patternPortSingular.FindStringSubmatch(portRange); len(matches) == 2 {
		firstPort, _ = strconv.Atoi(matches[1])
		lastPort = firstPort
		if msg := IsValidPortNum(firstPort); msg != nil {
			allErrs = append(allErrs, Invalid(field, firstPort, msg.Error()))
		}
	} else if matches = patternPortRange.FindStringSubmatch(portRange); len(matches) == 3 {
		firstPort, _ = strconv.Atoi(matches[1])
		if msg := IsValidPortNum(firstPort); msg != nil {
			allErrs = append(allErrs, Invalid(field, firstPort, msg.Error()))
		}
		lastPort, _ = strconv.Atoi(matches[2])
		if msg := IsValidPortNum(lastPort); msg != nil {
			allErrs = append(allErrs, Invalid(field, lastPort, msg.Error()))
		}
		if len(allErrs) == 0 && firstPort > lastPort {
			allErrs = append(allErrs, Invalid(field, portRange, "last port can't be lower than first port"))
		}
	} else {
		allErrs = append(allErrs, Invalid(field, portRange, "invalid syntax"))
	}
	return
}

// ValidateProtocol validates that given value belongs to supported protocols
func ValidateProtocol(protocol string, field string) ErrorList {
	allErrs := ErrorList{}

	if err := IsValidProtocol(protocol); err != nil {
		allErrs = append(allErrs, NotSupported(field, protocol, []string{TCP, UDP}))

	}

	return allErrs
}
