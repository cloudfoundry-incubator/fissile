package validation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Error is an implementation of the 'error' interface, which represents a
// field-level validation error.
type Error struct {
	Type     ErrorType
	Field    string
	BadValue interface{}
	Detail   string
}

// Error implements the error interface.
func (v *Error) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.ErrorBody())
}

// ErrorBody returns the error message without the field name.  This is useful
// for building nice-looking higher-level error reporting.
func (v *Error) ErrorBody() string {
	var s string
	switch v.Type {
	case ErrorTypeRequired, ErrorTypeForbidden, ErrorTypeTooLong, ErrorTypeInternal:
		s = fmt.Sprintf("%s", v.Type)
	default:
		var bad string
		badBytes, err := json.Marshal(v.BadValue)
		if err != nil {
			bad = err.Error()
		} else {
			bad = string(badBytes)
		}
		s = fmt.Sprintf("%s: %s", v.Type, bad)
	}
	if len(v.Detail) != 0 {
		s += fmt.Sprintf(": %s", v.Detail)
	}
	return s
}

// ErrorType is a machine readable value providing more detail about why
// a field is invalid.
type ErrorType string

const (
	// ErrorTypeNotFound is used to report failure to find a requested value
	// (e.g. looking up an ID).  See NotFound().
	ErrorTypeNotFound ErrorType = "FieldValueNotFound"
	// ErrorTypeRequired is used to report required values that are not
	// provided (e.g. empty strings, null values, or empty arrays).  See
	// Required().
	ErrorTypeRequired ErrorType = "FieldValueRequired"
	// ErrorTypeDuplicate is used to report collisions of values that must be
	// unique (e.g. unique IDs).  See Duplicate().
	ErrorTypeDuplicate ErrorType = "FieldValueDuplicate"
	// ErrorTypeInvalid is used to report malformed values (e.g. failed regex
	// match, too long, out of bounds).  See Invalid().
	ErrorTypeInvalid ErrorType = "FieldValueInvalid"
	// ErrorTypeNotSupported is used to report unknown values for enumerated
	// fields (e.g. a list of valid values).  See NotSupported().
	ErrorTypeNotSupported ErrorType = "FieldValueNotSupported"
	// ErrorTypeForbidden is used to report valid (as per formatting rules)
	// values which would be accepted under some conditions, but which are not
	// permitted by the current conditions (such as security policy).  See
	// Forbidden().
	ErrorTypeForbidden ErrorType = "FieldValueForbidden"
	// ErrorTypeTooLong is used to report that the given value is too long.
	// This is similar to ErrorTypeInvalid, but the error will not include the
	// too-long value.  See TooLong().
	ErrorTypeTooLong ErrorType = "FieldValueTooLong"
	// ErrorTypeGeneral is used to report general errors without additional
	// details.
	ErrorTypeGeneral ErrorType = "GeneralError"
	// ErrorTypeInternal is used to report other errors that are not related
	// to user input.  See InternalError().
	ErrorTypeInternal ErrorType = "InternalError"
)

// String converts a ErrorType into its corresponding canonical error message.
func (t ErrorType) String() string {
	switch t {
	case ErrorTypeNotFound:
		return "Not found"
	case ErrorTypeRequired:
		return "Required value"
	case ErrorTypeDuplicate:
		return "Duplicate value"
	case ErrorTypeInvalid:
		return "Invalid value"
	case ErrorTypeNotSupported:
		return "Unsupported value"
	case ErrorTypeForbidden:
		return "Forbidden"
	case ErrorTypeTooLong:
		return "Too long"
	case ErrorTypeGeneral:
		return "Error"
	case ErrorTypeInternal:
		return "Internal error"
	default:
		panic(fmt.Sprintf("unrecognized validation error: %q", string(t)))
	}
}

// NotFound returns a *Error indicating "value not found".  This is
// used to report failure to find a requested value (e.g. looking up an ID).
func NotFound(field string, value interface{}) *Error {
	return &Error{ErrorTypeNotFound, field, value, ""}
}

// Required returns a *Error indicating "value required".  This is used
// to report required values that are not provided (e.g. empty strings, null
// values, or empty arrays).
func Required(field string, detail string) *Error {
	return &Error{ErrorTypeRequired, field, "", detail}
}

// Duplicate returns a *Error indicating "duplicate value".  This is
// used to report collisions of values that must be unique (e.g. names or IDs).
func Duplicate(field string, value interface{}) *Error {
	return &Error{ErrorTypeDuplicate, field, value, ""}
}

// Invalid returns a *Error indicating "invalid value".  This is used
// to report malformed values (e.g. failed regex match, too long, out of bounds).
func Invalid(field string, value interface{}, detail string) *Error {
	return &Error{ErrorTypeInvalid, field, value, detail}
}

// NotSupported returns a *Error indicating "unsupported value".
// This is used to report unknown values for enumerated fields (e.g. a list of
// valid values).
func NotSupported(field string, value interface{}, validValues []string) *Error {
	detail := ""
	if validValues != nil && len(validValues) > 0 {
		detail = "supported values: " + strings.Join(validValues, ", ")
	}
	return &Error{ErrorTypeNotSupported, field, value, detail}
}

// Forbidden returns a *Error indicating "forbidden".  This is used to
// report valid (as per formatting rules) values which would be accepted under
// some conditions, but which are not permitted by current conditions (e.g.
// security policy).
func Forbidden(field string, detail string) *Error {
	return &Error{ErrorTypeForbidden, field, "", detail}
}

// TooLong returns a *Error indicating "too long".  This is used to
// report that the given value is too long.  This is similar to
// Invalid, but the returned error will not include the too-long
// value.
func TooLong(field string, value interface{}, maxLength int) *Error {
	return &Error{ErrorTypeTooLong, field, value, fmt.Sprintf("must have at most %d characters", maxLength)}
}

// GeneralError returns a *Error for a general failure.  This is used
// to signal that an error was found that has no structured details.  The
// err argument must be non-nil.
func GeneralError(field string, err error) *Error {
	return &Error{ErrorTypeGeneral, field, nil, err.Error()}
}

// InternalError returns a *Error indicating "internal error".  This is used
// to signal that an error was found that was not directly related to user
// input.  The err argument must be non-nil.
func InternalError(field string, err error) *Error {
	return &Error{ErrorTypeInternal, field, nil, err.Error()}
}

// ErrorList holds a set of Errors.  It is plausible that we might one day have
// non-field errors in this same umbrella package, but for now we don't, so
// we can keep it simple and leave ErrorList here.
type ErrorList []*Error

// Error implements the error interface.
func (v ErrorList) Error() string {
	return strings.Join(v.ErrorStrings(), "\n")
}

// ErrorStrings returns the underlying errors as a string slice, for testing
func (v ErrorList) ErrorStrings() []string {
	values := make([]string, 0, len(v))

	for _, item := range v {
		values = append(values, item.Error())
	}

	return values
}
