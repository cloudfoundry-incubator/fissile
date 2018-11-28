package model

import (
	"encoding/json"
	"errors"
)

// JobReference from the deployment manifest, references a job spec from a release by ReleaseName
type JobReference struct {
	*Job                `yaml:"-"`                 // The resolved job
	Name                string                     `yaml:"name"`    // The name of the job
	ReleaseName         string                     `yaml:"release"` // The release the job comes from
	ExportedProviders   map[string]JobProvidesInfo `yaml:"provides"`
	ResolvedConsumers   map[string]JobConsumesInfo `yaml:"consumes"`
	ContainerProperties JobContainerProperties     `yaml:"properties"`
}

// JobContainerProperties describes job configuration
type JobContainerProperties struct {
	BoshContainerization JobBoshContainerization `yaml:"bosh_containerization"`
}

// JobBoshContainerization describes settings specific to containerization
type JobBoshContainerization struct {
	PodSecurityPolicy   string           `yaml:"pod-security-policy,omitempty"`
	Ports               []JobExposedPort `yaml:"ports"`
	Run                 *RoleRun         `yaml:"run"`
	ColocatedContainers []string         `yaml:"colocated_containers,omitempty"`
	ServiceName         string           `yaml:"service_name,omitempty"`
}

// JobExposedPort describes a port to be available to other jobs, or the outside world
type JobExposedPort struct {
	Name                string `yaml:"name"`
	Protocol            string `yaml:"protocol"`
	External            string `yaml:"external"`
	Internal            string `yaml:"internal"`
	Public              bool   `yaml:"public"`
	Count               int    `yaml:"count"`
	Max                 int    `yaml:"max"`
	PortIsConfigurable  bool   `yaml:"port-configurable"`
	CountIsConfigurable bool   `yaml:"count-configurable"`
	InternalPort        int
	ExternalPort        int
}

func runPropertyPresent(j JobReference) bool {
	if j.ContainerProperties.BoshContainerization.Run == nil {
		return false
	}
	return true
}

func flightStagePresent(j JobReference) bool {
	if j.ContainerProperties.BoshContainerization.Run.FlightStage == "" {
		return false
	}
	return true
}

func healthCheckPresent(j JobReference) bool {
	if j.ContainerProperties.BoshContainerization.Run.HealthCheck == nil {
		return false
	}
	return true
}

func affinityPresent(j JobReference) bool {
	if j.ContainerProperties.BoshContainerization.Run.Affinity == nil {
		return false
	}
	return true
}

// JobReferences is a collection of pointers to job references
type JobReferences []*JobReference

// WithRunProperty returns all jobs with a BOSH containerization run property
// could cache this on InstanceGroup if it turns out to be expensive
func (jobs JobReferences) WithRunProperty() JobReferences {
	refs := JobReferences{}
	for _, j := range jobs {
		if j.ContainerProperties.BoshContainerization.Run != nil {
			refs = append(refs, j)
		}
	}
	return refs
}

type jobReferenceBoolProperty func(JobReference) bool

type jobReferenceIntegerProperty func(JobReference) int

type jobReferenceStringProperty func(JobReference) string

func (jobs JobReferences) atLeastOnce(validation jobReferenceBoolProperty) bool {
	for _, j := range jobs {
		if validation(*j) {
			return true
		}
	}

	return false
}

func (jobs JobReferences) equalOrMissing(validation jobReferenceBoolProperty) bool {
	var found *JobReference

	for _, j := range jobs {
		if validation(*j) {
			if found != nil && found != j {
				return false
			}
			found = j
		}
	}

	return true
}

func (jobs JobReferences) uniqueStringProperty(property jobReferenceStringProperty) (string, error) {
	found := ""
	for _, j := range jobs {
		test := property(*j)
		if test != "" && found != "" {
			return found, errors.New("property specified multiple times")
		}
		found = test
	}
	return found, nil
}

func (jobs JobReferences) atMostOnce(exists jobReferenceBoolProperty) bool {
	found := false
	for _, j := range jobs {
		if exists(*j) {
			if found {
				return false
			}
			found = true
		}
	}

	return true
}

func (jobs JobReferences) firstFlightStage() FlightStage {
	for _, j := range jobs {
		if flightStagePresent(*j) {
			return j.ContainerProperties.BoshContainerization.Run.FlightStage
		}
	}
	return ""
}

func (jobs JobReferences) firstHealthCheck() *HealthCheck {
	for _, j := range jobs {
		if j.ContainerProperties.BoshContainerization.Run.HealthCheck != nil {
			return j.ContainerProperties.BoshContainerization.Run.HealthCheck
		}
	}
	return nil
}

func (jobs JobReferences) firstAffinity() *RoleRunAffinity {
	for _, j := range jobs {
		if j.ContainerProperties.BoshContainerization.Run.Affinity != nil {
			return j.ContainerProperties.BoshContainerization.Run.Affinity
		}
	}
	return nil
}

// WriteConfigs merges the job's spec with the opinions and returns the result as JSON.
func (j *JobReference) WriteConfigs(instanceGroup *InstanceGroup, lightOpinionsPath, darkOpinionsPath string) ([]byte, error) {
	var config struct {
		Job struct {
			Name string `json:"name"`
		} `json:"job"`
		Parameters map[string]string      `json:"parameters"`
		Properties map[string]interface{} `json:"properties"`
		Networks   struct {
			Default map[string]string `json:"default"`
		} `json:"networks"`
		ExportedProperties []string               `json:"exported_properties"`
		Consumes           map[string]JobLinkInfo `json:"consumes"`
	}

	config.Parameters = make(map[string]string)
	config.Properties = make(map[string]interface{})
	config.Networks.Default = make(map[string]string)
	config.ExportedProperties = make([]string, 0)
	config.Consumes = make(map[string]JobLinkInfo)

	config.Job.Name = instanceGroup.Name

	for _, consumer := range j.ResolvedConsumers {
		config.Consumes[consumer.Name] = consumer.JobLinkInfo
	}

	opinions, err := NewOpinions(lightOpinionsPath, darkOpinionsPath)
	if err != nil {
		return nil, err
	}
	properties, err := j.Job.GetPropertiesForJob(opinions)
	if err != nil {
		return nil, err
	}
	config.Properties = properties

	for _, provider := range j.Job.AvailableProviders {
		config.ExportedProperties = append(config.ExportedProperties, provider.Properties...)
	}

	// Write out the configuration
	return json.MarshalIndent(config, "", "    ") // 4-space indent
}
