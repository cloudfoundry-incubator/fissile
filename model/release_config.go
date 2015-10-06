package model

import (
//"gopkg.in/yaml.v2"
)

// ReleaseConfig is a global deployment configuration key
type ReleaseConfig struct {
	Name        string
	Description string
	Jobs        []*Job
	UsageCount  int
}

// HasConflictingDefaults returns true if not all defaults match
//func (r *ReleaseConfig) HasConflictingDefaults() (bool, error) {
//	for _, job := range r.Jobs {

//	}
//}
