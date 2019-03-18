package model

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// RoleRun describes how a role should behave at runtime
type RoleRun struct {
	Scaling            *RoleRunScaling  `yaml:"scaling"`
	Capabilities       []string         `yaml:"capabilities"`
	PersistentVolumes  []*RoleRunVolume `yaml:"persistent-volumes"` // Backwards compat only
	SharedVolumes      []*RoleRunVolume `yaml:"shared-volumes"`     // Backwards compat only
	Volumes            []*RoleRunVolume `yaml:"volumes"`
	MemRequest         *int64           `yaml:"memory"`
	Memory             *RoleRunMemory   `yaml:"mem"`
	VirtualCPUs        *float64         `yaml:"virtual-cpus"`
	CPU                *RoleRunCPU      `yaml:"cpu"`
	FlightStage        FlightStage      `yaml:"flight-stage"`
	HealthCheck        *HealthCheck     `yaml:"healthcheck,omitempty"`
	ActivePassiveProbe string           `yaml:"active-passive-probe,omitempty"`
	ServiceAccount     string           `yaml:"service-account,omitempty"`
	Affinity           *RoleRunAffinity `yaml:"affinity,omitempty"`
}

// RoleRunAffinity describes how a role should behave with regard to node / pod selection
type RoleRunAffinity struct {
	PodAntiAffinity interface{} `yaml:"podAntiAffinity,omitempty"`
	PodAffinity     interface{} `yaml:"podAffinity,omitempty"`
	NodeAffinity    interface{} `yaml:"nodeAffinity,omitempty"`
}

// RoleRunMemory describes how a role should behave with regard to memory usage.
type RoleRunMemory struct {
	Request *int64 `yaml:"request"`
	Limit   *int64 `yaml:"limit"`
}

// RoleRunCPU describes how a role should behave with regard to cpu usage.
type RoleRunCPU struct {
	Request *float64 `yaml:"request"`
	Limit   *float64 `yaml:"limit"`
}

// RoleRunScaling describes how a role should scale out at runtime
type RoleRunScaling struct {
	Min       int  `yaml:"min"`
	Max       int  `yaml:"max"`
	Default   int  `yaml:"default"`
	HA        int  `yaml:"ha,omitempty"`
	MustBeOdd bool `yaml:"must_be_odd,omitempty"`
}

// RoleRunVolume describes a volume to be attached at runtime
type RoleRunVolume struct {
	Type        VolumeType        `yaml:"type"`
	Path        string            `yaml:"path"`
	Tag         string            `yaml:"tag"`
	Size        int               `yaml:"size"`
	Annotations map[string]string `yaml:"annotations"`
}

func (v RoleRunVolume) fingerprint() string {
	hasher := sha1.New()
	hasher.Write([]byte(v.Type))
	hasher.Write([]byte(v.Path))
	hasher.Write([]byte(v.Tag))
	hasher.Write([]byte(strconv.Itoa(v.Size)))
	hasher.Write([]byte(fmt.Sprintf("%v", v.Annotations)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// VolumeType is the type of volume to create
type VolumeType string

// These are the volume type available
const (
	VolumeTypePersistent = VolumeType("persistent") // A volume that is only used for this instance of the role
	VolumeTypeShared     = VolumeType("shared")     // A volume that acts as shared storage between multiple roles / instances
	VolumeTypeHost       = VolumeType("host")       // A volume that is a mount of a host directory
	VolumeTypeNone       = VolumeType("none")       // A volume that isn't mounted to anything
	VolumeTypeEmptyDir   = VolumeType("emptyDir")   // A volume that is shared between containers
)

// FlightStage describes when a role should be executed
type FlightStage string

// These are the flight stages available
const (
	FlightStagePreFlight  = FlightStage("pre-flight")  // A role that runs before the main jobs start
	FlightStageFlight     = FlightStage("flight")      // A role that is a main job
	FlightStagePostFlight = FlightStage("post-flight") // A role that runs after the main jobs are up
	FlightStageManual     = FlightStage("manual")      // A role that only runs via user intervention
)

// HealthCheck describes a non-standard health check endpoint
type HealthCheck struct {
	Liveness  *HealthProbe `yaml:"liveness,omitempty"`  // Details of liveness probe configuration
	Readiness *HealthProbe `yaml:"readiness,omitempty"` // Ditto for readiness probe
}

// HealthProbe holds the configuration for liveness and readiness
// probes based on the HealthCheck containing them.
type HealthProbe struct {
	URL              string            `yaml:"url"`                         // URL for a HTTP GET to return 200~399. Cannot be used with other checks.
	Headers          map[string]string `yaml:"headers"`                     // Custom headers; only used for URL.
	Command          []string          `yaml:"command,omitempty"`           // Individual commands to run inside the container; each is interpreted as a shell command. Cannot be used with other checks.
	Port             int               `yaml:"port"`                        // Port for a TCP probe. Cannot be used with other checks.
	InitialDelay     int               `yaml:"initial_delay,omitempty"`     // Initial Delay in seconds, default 3, minimum 1
	Period           int               `yaml:"period,omitempty"`            // Period in seconds, default 10, minimum 1
	Timeout          int               `yaml:"timeout,omitempty"`           // Timeout in seconds, default 3, minimum 1
	SuccessThreshold int               `yaml:"success_threshold,omitempty"` // Success threshold in seconds, default 1, minimum 1
	FailureThreshold int               `yaml:"failure_threshold,omitempty"` // Failure threshold in seconds, default 3, minimum 1
}

func maxInteger(jobs JobReferences, getProperty jobReferenceIntegerProperty) int {
	max := 0
	for _, j := range jobs {
		test := getProperty(*j)
		if test > max {
			max = test
		}
	}
	return max
}

func anyMustBeOdd(jobs JobReferences) bool {
	for _, job := range jobs {
		if job.ContainerProperties.BoshContainerization.Run.Scaling.MustBeOdd {
			return true
		}
	}
	return false
}

// setScaling calculates scaling by looking at every jobs run properties
func (r *RoleRun) setScaling(jobReferences JobReferences) {
	r.Scaling = &RoleRunScaling{}
	for _, j := range jobReferences {
		if j.ContainerProperties.BoshContainerization.Run.Scaling == nil {
			j.ContainerProperties.BoshContainerization.Run.Scaling = &RoleRunScaling{}
		}
	}
	r.Scaling.HA = maxInteger(jobReferences, func(j JobReference) int {
		return j.ContainerProperties.BoshContainerization.Run.Scaling.HA
	})
	r.Scaling.Max = maxInteger(jobReferences, func(j JobReference) int {
		return j.ContainerProperties.BoshContainerization.Run.Scaling.Max
	})
	r.Scaling.Min = maxInteger(jobReferences, func(j JobReference) int {
		return j.ContainerProperties.BoshContainerization.Run.Scaling.Min
	})
	r.Scaling.Default = maxInteger(jobReferences, func(j JobReference) int {
		return j.ContainerProperties.BoshContainerization.Run.Scaling.Default
	})
	r.Scaling.MustBeOdd = anyMustBeOdd(jobReferences)

	if r.Scaling.HA == 0 {
		r.Scaling.HA = r.Scaling.Min
	}
	if r.Scaling.Default < r.Scaling.Min {
		r.Scaling.Default = r.Scaling.Min
	}
}

// setCapabilities merges from all jobs and normalizes capabilities to upper case
func (r *RoleRun) mergeCapabilities(jobReferences JobReferences) {
	seen := map[string]int{}
	for _, j := range jobReferences {
		for _, c := range j.ContainerProperties.BoshContainerization.Run.Capabilities {
			seen[strings.ToUpper(c)] = 1
		}
	}

	for k := range seen {
		r.Capabilities = append(r.Capabilities, k)
	}
}

// setVolumes collects uniq volumes from every job using a fingerprint, also
// handles old volume entries for backwards compatiblity
func (r *RoleRun) mergeVolumes(jobReferences JobReferences) {
	seen := map[string]bool{}

	for _, j := range jobReferences {
		for _, v := range j.ContainerProperties.BoshContainerization.Run.Volumes {
			fp := v.fingerprint()
			if ok := seen[fp]; !ok {
				seen[fp] = true
				r.Volumes = append(r.Volumes, v)

			}
		}

		// Backwards compat: convert separate volume lists to the centralized one
		for _, v := range j.ContainerProperties.BoshContainerization.Run.PersistentVolumes {
			v.Type = VolumeTypePersistent
			fp := v.fingerprint()
			if ok := seen[fp]; !ok {
				seen[fp] = true
				r.Volumes = append(r.Volumes, v)

			}
		}
		for _, v := range j.ContainerProperties.BoshContainerization.Run.SharedVolumes {
			v.Type = VolumeTypeShared
			fp := v.fingerprint()
			if ok := seen[fp]; !ok {
				seen[fp] = true
				r.Volumes = append(r.Volumes, v)

			}
		}
		j.ContainerProperties.BoshContainerization.Run.PersistentVolumes = nil
		j.ContainerProperties.BoshContainerization.Run.SharedVolumes = nil
	}
}

func (r *RoleRun) setMaxFields(jobReferences JobReferences) {
	var maxMem, maxMemLimit, maxMemRequest *int64
	var maxVirtualCPUs, maxCPULimit, maxCPURequest *float64

	for _, j := range jobReferences {
		run := j.ContainerProperties.BoshContainerization.Run
		if run.MemRequest != nil {
			if test := run.MemRequest; maxMem == nil || *test > *maxMem {
				maxMem = test
			}
		}
		if run.Memory != nil {
			if test := run.Memory.Limit; maxMemLimit == nil || *test > *maxMemLimit {
				maxMemLimit = test
			}
			if test := run.Memory.Request; maxMemRequest == nil || *test > *maxMemRequest {
				maxMemRequest = test
			}
		}
		if run.VirtualCPUs != nil {
			if test := run.VirtualCPUs; maxVirtualCPUs == nil || *test > *maxVirtualCPUs {
				maxVirtualCPUs = test
			}
		}
		if run.CPU != nil {
			if test := run.CPU.Limit; maxCPULimit == nil || *test > *maxCPULimit {
				maxCPULimit = test
			}
			if test := run.CPU.Request; maxCPURequest == nil || *test > *maxCPURequest {
				maxCPURequest = test
			}
		}
	}
	r.MemRequest = maxMem
	if maxMemLimit != nil || maxMemRequest != nil {
		r.Memory = &RoleRunMemory{Limit: maxMemLimit, Request: maxMemRequest}
	}
	r.VirtualCPUs = maxVirtualCPUs
	if maxCPULimit != nil || maxCPURequest != nil {
		r.CPU = &RoleRunCPU{Limit: maxCPULimit, Request: maxCPURequest}
	}
}
