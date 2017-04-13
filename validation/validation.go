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

// ValidatePortRange validates that the given value is a valid port
// range, with its elements in range 0 - 65535.  It accepts singular
// ports P, and port ranges of the form N-M.
func ValidatePortRange(portrange string, field string) ErrorList {
	allErrs := ErrorList{}

	// Syntax:
	// % int
	// % int-int

	matches := patternPortSingular.FindStringSubmatch(portrange)
	if len(matches) == 0 {
		matches = patternPortRange.FindStringSubmatch(portrange)
		if len(matches) == 0 {
			return append(allErrs, Invalid(field, portrange, `invalid syntax`))
		}
	}
	// Note: matches[0] is the fist/left-most match. Here this is
	// entire string, if it matched, due to the ^...$ bracketing.
	// The captures, we only part we are interested in, start at
	// index __1__.

	for _, port := range matches[1:] {
		portInt, err := strconv.Atoi(port)
		if err != nil {
			// Note, this should not happen, given the regexes used.
			allErrs = append(allErrs, Invalid(field, port, `invalid number syntax`))
			continue
		}
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

// ValidateProtocol validates that given value belongs to supported protocols
func ValidateProtocol(protocol string, field string) ErrorList {
	allErrs := ErrorList{}

	if err := IsValidProtocol(protocol); err != nil {
		allErrs = append(allErrs, NotSupported(field, protocol, []string{TCP, UDP}))

	}

	return allErrs
}
