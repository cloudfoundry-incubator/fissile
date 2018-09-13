package model

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SUSE/fissile/util"

	"gopkg.in/yaml.v2"
)

// InstanceGroups is an array of Role*
type InstanceGroups []*InstanceGroup

// Len is the number of instance groups in the slice
func (igs InstanceGroups) Len() int {
	return len(igs)
}

// Less reports whether role at index i sort before role at index j
func (igs InstanceGroups) Less(i, j int) bool {
	return strings.Compare(igs[i].Name, igs[j].Name) < 0
}

// Swap exchanges roles at index i and index j
func (igs InstanceGroups) Swap(i, j int) {
	igs[i], igs[j] = igs[j], igs[i]
}

// InstanceGroup represents a collection of jobs that are colocated on a container
type InstanceGroup struct {
	Name                string          `yaml:"name"`
	Description         string          `yaml:"description"`
	EnvironScripts      []string        `yaml:"environment_scripts"`
	Scripts             []string        `yaml:"scripts"`
	PostConfigScripts   []string        `yaml:"post_config_scripts"`
	Type                RoleType        `yaml:"type,omitempty"`
	JobReferences       []*JobReference `yaml:"jobs"`
	Configuration       *Configuration  `yaml:"configuration"`
	Run                 *RoleRun        `yaml:"run"`
	Tags                []RoleTag       `yaml:"tags"`
	ColocatedContainers []string        `yaml:"colocated_containers,omitempty"`

	roleManifest *RoleManifest
}

// RoleType is the type of the role; see the constants below
type RoleType string

// These are the types of roles available
const (
	RoleTypeBoshTask           = RoleType("bosh-task")           // A role that is a BOSH task
	RoleTypeBosh               = RoleType("bosh")                // A role that is a BOSH job
	RoleTypeColocatedContainer = RoleType("colocated-container") // A role that is supposed to be used by other roles to specify a colocated container
)

// JobReference represents a job in the context of a role
type JobReference struct {
	*Job                `yaml:"-"`                 // The resolved job
	Name                string                     `yaml:"name"`    // The name of the job
	ReleaseName         string                     `yaml:"release"` // The release the job comes from
	ExportedProviders   map[string]jobProvidesInfo `yaml:"provides"`
	ResolvedConsumers   map[string]jobConsumesInfo `yaml:"consumes"`
	ContainerProperties JobContainerProperties     `yaml:"properties"`
}

// RoleRun describes how a role should behave at runtime
type RoleRun struct {
	Scaling            *RoleRunScaling       `yaml:"scaling"`
	Capabilities       []string              `yaml:"capabilities"`
	PersistentVolumes  []*RoleRunVolume      `yaml:"persistent-volumes"` // Backwards compat only
	SharedVolumes      []*RoleRunVolume      `yaml:"shared-volumes"`     // Backwards compat only
	Volumes            []*RoleRunVolume      `yaml:"volumes"`
	MemRequest         *int64                `yaml:"memory"`
	Memory             *RoleRunMemory        `yaml:"mem"`
	VirtualCPUs        *float64              `yaml:"virtual-cpus"`
	CPU                *RoleRunCPU           `yaml:"cpu"`
	ExposedPorts       []*RoleRunExposedPort `yaml:"exposed-ports"`
	FlightStage        FlightStage           `yaml:"flight-stage"`
	HealthCheck        *HealthCheck          `yaml:"healthcheck,omitempty"`
	ActivePassiveProbe string                `yaml:"active-passive-probe,omitempty"`
	ServiceAccount     string                `yaml:"service-account,omitempty"`
	Affinity           *RoleRunAffinity      `yaml:"affinity,omitempty"`
	Environment        []string              `yaml:"env"`
	ObjectAnnotations  *map[string]string    `yaml:"object-annotations,omitempty"`
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

// RoleRunExposedPort describes a port to be available to other roles, or the outside world
type RoleRunExposedPort struct {
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

// RoleTag are the acceptable tags
type RoleTag string

// The list of acceptable tags
const (
	RoleTagStopOnFailure     = RoleTag("stop-on-failure")
	RoleTagSequentialStartup = RoleTag("sequential-startup")
	RoleTagActivePassive     = RoleTag("active-passive")
)

// GetLongDescription returns the description of the instance group plus a list of all included jobs
func (g *InstanceGroup) GetLongDescription() string {
	desc := g.Description
	if len(desc) > 0 {
		desc += "\n\n"
	}
	desc += fmt.Sprintf("The %s instance group contains the following jobs:", g.Name)
	var noDesc []string
	also := ""
	for _, jobReference := range g.JobReferences {
		if jobReference.Description == "" {
			noDesc = append(noDesc, jobReference.Name)
		} else {
			desc += fmt.Sprintf("\n\n- %s: %s", jobReference.Name, jobReference.Description)
			also = "Also: "
		}
	}
	if len(noDesc) > 0 {
		desc += fmt.Sprintf("\n\n%s%s", also, strings.Join(noDesc, ", "))
	}
	return desc
}

// GetScriptPaths returns the paths to the startup / post configgin scripts for a instance group
func (g *InstanceGroup) GetScriptPaths() map[string]string {
	result := map[string]string{}

	for _, scriptList := range [][]string{g.EnvironScripts, g.Scripts, g.PostConfigScripts} {
		for _, script := range scriptList {
			if filepath.IsAbs(script) {
				// Absolute paths _inside_ the container; there is nothing to copy
				continue
			}
			result[script] = filepath.Join(filepath.Dir(g.roleManifest.manifestFilePath), script)
		}
	}

	return result

}

// GetScriptSignatures returns the SHA1 of all of the script file names and contents
func (g *InstanceGroup) GetScriptSignatures() (string, error) {
	hasher := sha1.New()

	paths := g.GetScriptPaths()
	scripts := make([]string, 0, len(paths))

	for filename := range paths {
		scripts = append(scripts, filename)
	}

	sort.Strings(scripts)

	for _, filename := range scripts {
		hasher.Write([]byte(filename))

		f, err := os.Open(paths[filename])
		if err != nil {
			return "", err
		}

		_, err = io.Copy(hasher, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetTemplateSignatures returns the SHA1 of all of the templates and contents
func (g *InstanceGroup) GetTemplateSignatures() (string, error) {
	hasher := sha1.New()

	i := 0
	templates := make([]string, len(g.Configuration.Templates))

	for _, templateDef := range g.Configuration.Templates {
		templates[i] = fmt.Sprintf("%v: %v", templateDef.Key, templateDef.Value)
		i++
	}

	sort.Strings(templates)

	for _, template := range templates {
		hasher.Write([]byte(template))
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetRoleDevVersion determines the version hash for the role, using the basic
// role dev version, and the aggregated spec and opinion
// information. In this manner opinion changes cause a rebuild of the
// associated role images.
func (g *InstanceGroup) GetRoleDevVersion(opinions *Opinions, tagExtra, fissileVersion string, grapher util.ModelGrapher) (string, error) {

	// Basic role version
	jobPkgVersion, inputSigs, err := g.getRoleJobAndPackagesSignature(grapher)
	if err != nil {
		return "", fmt.Errorf("Error calculating checksum for instance group %s: %s", g.Name, err.Error())
	}

	// Aggregate with the properties from the opinions, per each job in the
	// instance group.  This is similar to what NewDockerPopulator (and its
	// subordinate WriteConfigs) do, with an important difference:
	// - NDP/WC does not care about order. We do, as we need a stable hash for the
	//   configuration.
	signatures := []string{
		jobPkgVersion,
		fissileVersion,
		tagExtra,
	}
	extraGraphEdges := [][]string{
		[]string{"version/fissile/", fissileVersion},
		[]string{"extra/", tagExtra},
	}

	if opinions != nil {
		// Job order comes from the role manifest, and is sort of
		// fix. Avoid sorting for now.  Also note, if a property is
		// used multiple times, in different jobs, it will be added
		// that often. No deduplication across the jobs.
		for _, jobReference := range g.JobReferences {
			// Get properties ...
			properties, err := jobReference.GetPropertiesForJob(opinions)
			if err != nil {
				return "", err
			}

			// ... and flatten the nest into a simple k/v mapping.
			// Note, this is a total flattening, even over arrays.
			flatProps := FlattenOpinions(properties, true)

			// Get and sort the keys, ...
			var keys []string
			for property := range flatProps {
				keys = append(keys, property)
			}
			sort.Strings(keys)

			// ... then add them and their values to the hash precursor
			// For the graph output, adding all properties individually results in
			// too many nodes and makes graphviz fall over. So use the hash of them
			// all instead.
			propertyHasher := sha1.New()
			for _, property := range keys {
				value := flatProps[property]
				signatures = append(signatures, property, value)
				if grapher != nil {
					propertyHasher.Write([]byte(property))
					propertyHasher.Write([]byte{0x1F})
					propertyHasher.Write([]byte(value))
					propertyHasher.Write([]byte{0x1E})
				}
			}
			if grapher != nil {
				extraGraphEdges = append(extraGraphEdges, []string{
					fmt.Sprintf("properties/%s:", jobReference.Name),
					hex.EncodeToString(propertyHasher.Sum(nil))})
			}
		}
	}
	devVersion := AggregateSignatures(signatures)
	if grapher != nil {
		_ = grapher.GraphNode(devVersion, map[string]string{"label": "role/" + g.Name})
		for _, inputSig := range inputSigs {
			_ = grapher.GraphEdge(inputSig, jobPkgVersion, nil)
		}
		_ = grapher.GraphNode(jobPkgVersion, map[string]string{"label": "role/jobpkg/" + g.Name})
		_ = grapher.GraphEdge(jobPkgVersion, devVersion, nil)
		for _, extraGraphEdgeParts := range extraGraphEdges {
			prefix := extraGraphEdgeParts[0]
			value := extraGraphEdgeParts[1]
			valueHasher := sha1.New()
			valueHasher.Write([]byte(value))
			valueHash := hex.EncodeToString(valueHasher.Sum(nil))
			_ = grapher.GraphEdge(prefix+valueHash, devVersion, nil)
			_ = grapher.GraphNode(prefix+valueHash, map[string]string{"label": prefix + value})
		}
	}
	return devVersion, nil
}

// AggregateSignatures returns the SHA1 for a slice of strings
func AggregateSignatures(signatures []string) string {
	hasher := sha1.New()
	length := 0
	for _, signature := range signatures {
		// Hash the strings, with separator/terminator. We do
		// __not__ want {"ab","a"} and {"a","ba"} to hash to
		// the same value.
		hasher.Write([]byte(signature))
		hasher.Write([]byte("\x00"))
		length = length + len(signature)
	}
	// Hash in the total length of the input
	hasher.Write([]byte(fmt.Sprintf("%d", length)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// getRoleJobAndPackagesSignature gets the aggregate signature of all jobs and packages
// It also returns a list of all hashes involved in calculating the final result
func (g *InstanceGroup) getRoleJobAndPackagesSignature(grapher util.ModelGrapher) (string, []string, error) {
	roleSignature := ""
	var inputs []string
	var packages Packages

	// Jobs are *not* sorted because they are an array and the order may be
	// significant, in particular for bosh-task roles.
	for _, jobReference := range g.JobReferences {
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, jobReference.SHA1)
		packages = append(packages, jobReference.Packages...)
		inputs = append(inputs, jobReference.Fingerprint)
		if grapher != nil {
			_ = grapher.GraphNode(jobReference.Fingerprint,
				map[string]string{"label": fmt.Sprintf("job/%s/%s", jobReference.ReleaseName, jobReference.Name)})
			_ = grapher.GraphEdge("release/"+jobReference.ReleaseName, jobReference.Fingerprint, nil)
			for _, pkg := range jobReference.Packages {
				_ = grapher.GraphEdge("release/"+jobReference.ReleaseName, pkg.Fingerprint, nil)
			}
		}
	}

	sort.Sort(packages)
	for _, pkg := range packages {
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, pkg.SHA1)
		inputs = append(inputs, pkg.Fingerprint)
		if grapher != nil {
			_ = grapher.GraphNode(pkg.Fingerprint, map[string]string{"label": "pkg/" + pkg.Name})
		}
	}

	// Collect signatures for various script sections
	sig, err := g.GetScriptSignatures()
	if err != nil {
		return "", nil, err
	}
	roleSignature = fmt.Sprintf("%s\n%s", roleSignature, sig)

	// If there are templates, generate signature for them
	if g.Configuration != nil && g.Configuration.Templates != nil {
		sig, err = g.GetTemplateSignatures()
		if err != nil {
			return "", nil, err
		}
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, sig)
	}

	hasher := sha1.New()
	hasher.Write([]byte(roleSignature))
	return hex.EncodeToString(hasher.Sum(nil)), inputs, nil
}

// HasTag returns true if the role has a specific tag
func (g *InstanceGroup) HasTag(tag RoleTag) bool {
	for _, t := range g.Tags {
		if t == tag {
			return true
		}
	}

	return false
}

func (g *InstanceGroup) calculateRoleConfigurationTemplates() {
	if g.Configuration == nil {
		g.Configuration = &Configuration{}
	}
	if g.Configuration.Templates == nil {
		g.Configuration.Templates = yaml.MapSlice{}
	}

	roleConfigs := yaml.MapSlice{}
	for _, templateDef := range g.Configuration.Templates {
		k := templateDef.Key.(string)
		v := templateDef.Value

		roleConfigs = append(roleConfigs, yaml.MapItem{Key: k, Value: v})
	}

	for _, templateDef := range g.roleManifest.Configuration.Templates {
		k := templateDef.Key.(string)
		v := templateDef.Value

		if _, ok := getTemplate(roleConfigs, k); !ok {

			roleConfigs = append(roleConfigs, yaml.MapItem{Key: k, Value: v})
		}
	}

	g.Configuration.Templates = roleConfigs
}

// LookupJob will find the given job in this role, or nil if not found
func (g *InstanceGroup) LookupJob(name string) *JobReference {
	for _, jobReference := range g.JobReferences {
		if jobReference.Job.Name == name {
			return jobReference
		}
	}
	return nil
}

// IsPrivileged tests if the instance group capabilities enable fully privileged
// mode.
func (g *InstanceGroup) IsPrivileged() bool {
	for _, cap := range g.Run.Capabilities {
		if cap == "ALL" {
			return true
		}
	}
	return false
}

// IsColocated tests if the role is of type ColocatedContainer, or
// not. It returns true if this role is of that type, or false otherwise.
func (g *InstanceGroup) IsColocated() bool {
	return g.Type == RoleTypeColocatedContainer
}

// GetColocatedRoles lists all colocation roles references by this instance group
func (g *InstanceGroup) GetColocatedRoles() []*InstanceGroup {
	result := make([]*InstanceGroup, len(g.ColocatedContainers))
	for i, name := range g.ColocatedContainers {
		if role := g.roleManifest.LookupInstanceGroup(name); role != nil {
			result[i] = role
		}
	}

	return result
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
		Consumes           map[string]jobLinkInfo `json:"consumes"`
	}

	config.Parameters = make(map[string]string)
	config.Properties = make(map[string]interface{})
	config.Networks.Default = make(map[string]string)
	config.ExportedProperties = make([]string, 0)
	config.Consumes = make(map[string]jobLinkInfo)

	config.Job.Name = instanceGroup.Name

	for _, consumer := range j.ResolvedConsumers {
		config.Consumes[consumer.Name] = consumer.jobLinkInfo
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
