package util

// Verbosity is an enumeration describing how verbose we should be
type Verbosity int

// Known verbosity flags
const (
	VerbosityQuiet   = iota // Be more quiet than normal
	VerbosityDefault = iota // Normal amounts of output
	VerbosityVerbose = iota // Extra detailed information
	VerbosityDebug   = iota // Excessive output for troubleshooting
)
