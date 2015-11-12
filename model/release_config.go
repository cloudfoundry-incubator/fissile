package model

// ReleaseConfig is a global deployment configuration key
type ReleaseConfig struct {
	Name        string
	Description string
	Jobs        Jobs
	UsageCount  int
}
