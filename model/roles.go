package model

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SUSE/fissile/util"
	"github.com/SUSE/fissile/validation"

	"gopkg.in/yaml.v2"
)

// RoleType is the type of the role; see the constants below
type RoleType string

// These are the types of roles available
const (
	RoleTypeBoshTask           = RoleType("bosh-task")           // A role that is a BOSH task
	RoleTypeBosh               = RoleType("bosh")                // A role that is a BOSH job
	RoleTypeDocker             = RoleType("docker")              // A role that is a raw Docker image
	RoleTypeColocatedContainer = RoleType("colocated-container") // A role that is supposed to be used by other roles to specify a colocated container
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

// RoleManifest represents a collection of roles
type RoleManifest struct {
	InstanceGroups InstanceGroups `yaml:"instance_groups"`
	Configuration  *Configuration `yaml:"configuration"`

	manifestFilePath string
}

// JobReference represents a job in the context of a role
type JobReference struct {
	*Job              `yaml:"-"`                 // The resolved job
	Name              string                     `yaml:"name"`         // The name of the job
	ReleaseName       string                     `yaml:"release_name"` // The release the job comes from
	ExportedProviders map[string]jobProvidesInfo `yaml:"provides"`
	ResolvedConsumers map[string]jobConsumesInfo `yaml:"consumes"`
}

// RoleTag are the acceptable tags
type RoleTag string

// The list of acceptable tags
const (
	RoleTagStopOnFailure     = RoleTag("stop-on-failure")
	RoleTagSequentialStartup = RoleTag("sequential-startup")
	RoleTagHeadless          = RoleTag("headless")
	RoleTagActivePassive     = RoleTag("active-passive")
)

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

// InstanceGroups is an array of Role*
type InstanceGroups []*InstanceGroup

// GeneratorType describes the type of generator used for the configuration value
type GeneratorType string

// These are the available generator types for configuration values
const (
	GeneratorTypePassword      = GeneratorType("Password")      // Password
	GeneratorTypeSSH           = GeneratorType("SSH")           // SSH key
	GeneratorTypeCACertificate = GeneratorType("CACertificate") // CA Certificate
	GeneratorTypeCertificate   = GeneratorType("Certificate")   // Certificate
)

// An AuthRule is a single rule for a RBAC authorization role
type AuthRule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

// An AuthRole is a role for RBAC authorization
type AuthRole []AuthRule

// An AuthAccount is a service account for RBAC authorization
type AuthAccount struct {
	Roles []string `yaml:"roles"`
}

// Configuration contains information about how to configure the
// resulting images
type Configuration struct {
	Authorization struct {
		Roles    map[string]AuthRole    `yaml:"roles,omitempty"`
		Accounts map[string]AuthAccount `yaml:"accounts,omitempty"`
	} `yaml:"auth,omitempty"`
	Templates map[string]string          `yaml:"templates"`
	Variables ConfigurationVariableSlice `yaml:"variables"`
}

// ConfigurationVariable is a configuration to be exposed to the IaaS
//
// Notes on the fields Type and Internal.
// 1. Type's legal values are `user` and `environment`.
//    `user` is default.
//
//    A `user` CV is rendered into k8s yml config files, etc. to make it available to roles who need it.
//    - An internal CV is rendered to all roles.
//    - A public CV is rendered only to the roles whose templates refer to the CV.
//
//    An `environment` CV comes from a script, not the user. Being
//    internal this way it is not rendered to any configuration files.
//
// 2. Internal's legal values are all YAML boolean values.
//    A public CV is used in templates
//    An internal CV is not, consumed in a script instead.
type ConfigurationVariable struct {
	Name          string                          `yaml:"name"`
	PreviousNames []string                        `yaml:"previous_names"`
	Default       interface{}                     `yaml:"default"`
	Description   string                          `yaml:"description"`
	Example       string                          `yaml:"example"`
	Generator     *ConfigurationVariableGenerator `yaml:"generator"`
	Type          CVType                          `yaml:"type"`
	Internal      bool                            `yaml:"internal,omitempty"`
	Secret        bool                            `yaml:"secret,omitempty"`
	Required      bool                            `yaml:"required,omitempty"`
	Immutable     bool                            `yaml:"immutable,omitempty"`
}

// Value fetches the value of config variable
func (config *ConfigurationVariable) Value(defaults map[string]string) (bool, string) {
	var value interface{}

	value = config.Default

	if defaultValue, ok := defaults[config.Name]; ok {
		value = defaultValue
	}

	if value == nil {
		return false, ""
	}

	var stringifiedValue string
	if valueAsString, ok := value.(string); ok {
		var err error
		stringifiedValue, err = strconv.Unquote(fmt.Sprintf(`"%s"`, valueAsString))
		if err != nil {
			stringifiedValue = valueAsString
		}
	} else {
		asJSON, _ := json.Marshal(value)
		stringifiedValue = string(asJSON)
	}

	return true, stringifiedValue
}

// CVType is the type of the configuration variable; see the constants below
type CVType string

const (
	// CVTypeUser is for user-specified variables (default)
	CVTypeUser = CVType("user")
	// CVTypeEnv is for script-specified variables
	CVTypeEnv = CVType("environment")
)

// CVMap is a map from variable name to ConfigurationVariable, for
// various places which require quick access/search/existence check.
type CVMap map[string]*ConfigurationVariable

// ConfigurationVariableSlice is a sortable slice of ConfigurationVariables
type ConfigurationVariableSlice []*ConfigurationVariable

// Len is the number of ConfigurationVariables in the slice
func (confVars ConfigurationVariableSlice) Len() int {
	return len(confVars)
}

// Less reports whether config variable at index i sort before the one at index j
func (confVars ConfigurationVariableSlice) Less(i, j int) bool {
	return strings.Compare(confVars[i].Name, confVars[j].Name) < 0
}

// Swap exchanges configuration variables at index i and index j
func (confVars ConfigurationVariableSlice) Swap(i, j int) {
	confVars[i], confVars[j] = confVars[j], confVars[i]
}

// ConfigurationVariableGenerator describes how to automatically generate values
// for a configuration variable
type ConfigurationVariableGenerator struct {
	ID        string        `yaml:"id"`
	Type      GeneratorType `yaml:"type"`
	ValueType string        `yaml:"value_type"`
}

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

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, releases []*Release, grapher util.ModelGrapher) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	roleManifest := RoleManifest{}
	roleManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &roleManifest); err != nil {
		return nil, err
	}
	if roleManifest.Configuration == nil {
		roleManifest.Configuration = &Configuration{}
	}
	if roleManifest.Configuration.Templates == nil {
		roleManifest.Configuration.Templates = map[string]string{}
	}

	err = roleManifest.resolveRoleManifest(releases, grapher)
	if err != nil {
		return nil, err
	}
	return &roleManifest, nil
}

// resolveRoleManifest takes a role manifest as loaded from disk, and validates
// it to ensure it has no errors, and that the various ancillary structures are
// correctly populated.
func (m *RoleManifest) resolveRoleManifest(releases []*Release, grapher util.ModelGrapher) error {
	mappedReleases := map[string]*Release{}

	for _, release := range releases {
		_, ok := mappedReleases[release.Name]

		if ok {
			return fmt.Errorf("Error - release %s has been loaded more than once", release.Name)
		}

		mappedReleases[release.Name] = release
		if grapher != nil {
			grapher.GraphNode("release/"+release.Name, map[string]string{"label": "release/" + release.Name})
		}
	}

	// See also 'GetVariablesForRole' (mustache.go).
	declaredConfigs := MakeMapOfVariables(m)

	allErrs := validation.ErrorList{}

	for i := len(m.InstanceGroups) - 1; i >= 0; i-- {
		instanceGroup := m.InstanceGroups[i]

		// Remove all instance groups that are not of the "bosh" or "bosh-task" type
		// Default type is considered to be "bosh".
		// The kept instance groups are validated.
		switch instanceGroup.Type {
		case "":
			instanceGroup.Type = RoleTypeBosh
		case RoleTypeBosh, RoleTypeBoshTask, RoleTypeColocatedContainer:
			// Nothing to do.
		case RoleTypeDocker:
			m.InstanceGroups = append(m.InstanceGroups[:i], m.InstanceGroups[i+1:]...)
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].type", instanceGroup.Name),
				instanceGroup.Type, "Expected one of bosh, bosh-task, docker, or colocated-container"))
		}

		allErrs = append(allErrs, validateRoleTags(instanceGroup)...)
		allErrs = append(allErrs, validateRoleRun(instanceGroup, m, declaredConfigs)...)
	}

	for _, instanceGroup := range m.InstanceGroups {
		instanceGroup.roleManifest = m

		if instanceGroup.Run != nil && instanceGroup.Run.ActivePassiveProbe != "" {
			if !instanceGroup.HasTag(RoleTagActivePassive) {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].run.active-passive-probe", instanceGroup.Name),
					instanceGroup.Run.ActivePassiveProbe,
					"Active/passive probes are only valid on instance groups with active-passive tag"))
			}
		}

		for _, jobReference := range instanceGroup.JobReferences {
			release, ok := mappedReleases[jobReference.ReleaseName]

			if !ok {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].jobs[%s]", instanceGroup.Name, jobReference.Name),
					jobReference.ReleaseName,
					"Referenced release is not loaded"))
				continue
			}

			job, err := release.LookupJob(jobReference.Name)
			if err != nil {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].jobs[%s]", instanceGroup.Name, jobReference.Name),
					jobReference.ReleaseName, err.Error()))
				continue
			}

			jobReference.Job = job
			if grapher != nil {
				_ = grapher.GraphNode(job.Fingerprint, map[string]string{"label": "job/" + job.Name})
			}

			if jobReference.ResolvedConsumers == nil {
				// No explicitly specified consumers
				jobReference.ResolvedConsumers = make(map[string]jobConsumesInfo)
			}

			for name, info := range jobReference.ResolvedConsumers {
				info.Name = name
				jobReference.ResolvedConsumers[name] = info
			}
		}

		instanceGroup.calculateRoleConfigurationTemplates()

		// Validate that specified colocated containers are configured and of the
		// correct type
		for idx, roleName := range instanceGroup.ColocatedContainers {
			if lookupRole := m.LookupInstanceGroup(roleName); lookupRole == nil {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", instanceGroup.Name, idx),
					roleName,
					"There is no such instance group defined"))

			} else if lookupRole.Type != RoleTypeColocatedContainer {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", instanceGroup.Name, idx),
					roleName,
					"The instance group is not of required type colocated-container"))
			}
		}
	}

	// Skip further validation if we fail to resolve any jobs
	// This lets us assume valid jobs in the validation routines
	if len(allErrs) == 0 {
		allErrs = append(allErrs, m.resolveLinks()...)
		allErrs = append(allErrs, validateVariableType(m.Configuration.Variables)...)
		allErrs = append(allErrs, validateVariableSorting(m.Configuration.Variables)...)
		allErrs = append(allErrs, validateVariablePreviousNames(m.Configuration.Variables)...)
		allErrs = append(allErrs, validateVariableUsage(m)...)
		allErrs = append(allErrs, validateTemplateUsage(m)...)
		allErrs = append(allErrs, validateNonTemplates(m)...)
		allErrs = append(allErrs, validateServiceAccounts(m)...)
		allErrs = append(allErrs, validateUnusedColocatedContainerRoles(m)...)
		allErrs = append(allErrs, validateColocatedContainerPortCollisions(m)...)
		allErrs = append(allErrs, validateColocatedContainerVolumeShares(m)...)
	}

	if len(allErrs) != 0 {
		return fmt.Errorf(allErrs.Errors())
	}

	return nil
}

// LookupInstanceGroup will find the given instance group in the role manifest
func (m *RoleManifest) LookupInstanceGroup(name string) *InstanceGroup {
	for _, instanceGroup := range m.InstanceGroups {
		if instanceGroup.Name == name {
			return instanceGroup
		}
	}
	return nil
}

// resolveLinks examines the BOSH links specified in the job specs and maps
// them to the correct role / job that can be looked up at runtime
func (m *RoleManifest) resolveLinks() validation.ErrorList {
	errors := make(validation.ErrorList, 0)

	// Build mappings of providers by name, and by type.  Note that the names
	// involved here are the aliases, where appropriate.
	providersByName := make(map[string]jobProvidesInfo)
	providersByType := make(map[string][]jobProvidesInfo)
	for _, instanceGroup := range m.InstanceGroups {
		for _, jobReference := range instanceGroup.JobReferences {
			var availableProviders []string
			for availableName, availableProvider := range jobReference.Job.AvailableProviders {
				availableProviders = append(availableProviders, availableName)
				if availableProvider.Type != "" {
					providersByType[availableProvider.Type] = append(providersByType[availableProvider.Type], jobProvidesInfo{
						jobLinkInfo: jobLinkInfo{
							Name:     availableProvider.Name,
							Type:     availableProvider.Type,
							RoleName: instanceGroup.Name,
							JobName:  jobReference.Name,
						},
						Properties: availableProvider.Properties,
					})
				}
			}
			for name, provider := range jobReference.ExportedProviders {
				info, ok := jobReference.Job.AvailableProviders[name]
				if !ok {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf("instance_groups[%s].jobs[%s].provides[%s]", instanceGroup.Name, jobReference.Name, name),
						fmt.Sprintf("Provider not found; available providers: %v", availableProviders)))
					continue
				}
				if provider.Alias != "" {
					name = provider.Alias
				}
				providersByName[name] = jobProvidesInfo{
					jobLinkInfo: jobLinkInfo{
						Name:     info.Name,
						Type:     info.Type,
						RoleName: instanceGroup.Name,
						JobName:  jobReference.Name,
					},
					Properties: info.Properties,
				}
			}
		}
	}

	// Resolve the consumers
	for _, instanceGroup := range m.InstanceGroups {
		for _, jobReference := range instanceGroup.JobReferences {
			expectedConsumers := make([]jobConsumesInfo, len(jobReference.Job.DesiredConsumers))
			copy(expectedConsumers, jobReference.Job.DesiredConsumers)
			// Deal with any explicitly marked consumers in the role manifest
			for consumerName, consumerInfo := range jobReference.ResolvedConsumers {
				consumerAlias := consumerName
				if consumerInfo.Alias != "" {
					consumerAlias = consumerInfo.Alias
				}
				if consumerAlias == "" {
					// There was a consumer with an explicitly empty name
					errors = append(errors, validation.Invalid(
						fmt.Sprintf(`instance_group[%s].job[%s]`, instanceGroup.Name, jobReference.Name),
						"name",
						fmt.Sprintf("consumer has no name")))
					continue
				}
				provider, ok := providersByName[consumerAlias]
				if !ok {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf(`instance_group[%s].job[%s].consumes[%s]`, instanceGroup.Name, jobReference.Name, consumerName),
						fmt.Sprintf(`consumer %s not found`, consumerAlias)))
					continue
				}
				jobReference.ResolvedConsumers[consumerName] = jobConsumesInfo{
					jobLinkInfo: provider.jobLinkInfo,
				}

				for i := range expectedConsumers {
					if expectedConsumers[i].Name == consumerName {
						expectedConsumers = append(expectedConsumers[:i], expectedConsumers[i+1:]...)
						break
					}
				}
			}
			// Handle any consumers not overridden in the role manifest
			for _, consumerInfo := range expectedConsumers {
				// Consumers don't _have_ to be listed; they can be automatically
				// matched to a published name, or to the only provider of the
				// same type in the whole deployment
				var provider jobProvidesInfo
				var ok bool
				if consumerInfo.Name != "" {
					provider, ok = providersByName[consumerInfo.Name]
				}
				if !ok && len(providersByType[consumerInfo.Type]) == 1 {
					provider = providersByType[consumerInfo.Type][0]
					ok = true
				}
				if ok {
					name := consumerInfo.Name
					if name == "" {
						name = provider.Name
					}
					info := jobReference.ResolvedConsumers[name]
					info.Name = provider.Name
					info.Type = provider.Type
					info.RoleName = provider.RoleName
					info.JobName = provider.JobName
					jobReference.ResolvedConsumers[name] = info
				} else if !consumerInfo.Optional {
					errors = append(errors, validation.Required(
						fmt.Sprintf(`instance_group[%s].job[%s].consumes[%s]`, instanceGroup.Name, jobReference.Name, consumerInfo.Name),
						fmt.Sprintf(`failed to resolve provider %s (type %s)`, consumerInfo.Name, consumerInfo.Type)))
				}
			}
		}
	}

	return errors
}

// SelectInstanceGroups will find only the given instance groups in the role manifest
func (m *RoleManifest) SelectInstanceGroups(roleNames []string) (InstanceGroups, error) {
	if len(roleNames) == 0 {
		// No names specified, assume all instance groups
		return m.InstanceGroups, nil
	}

	var results InstanceGroups
	var missingRoles []string

	for _, roleName := range roleNames {
		if instanceGroup := m.LookupInstanceGroup(roleName); instanceGroup != nil {
			results = append(results, instanceGroup)
		} else {
			missingRoles = append(missingRoles, roleName)
		}
	}
	if len(missingRoles) > 0 {
		return nil, fmt.Errorf("Some instance groups are unknown: %v", missingRoles)
	}

	return results, nil
}

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

	for k, v := range g.Configuration.Templates {
		templates[i] = fmt.Sprintf("%s: %s", k, v)
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
		g.Configuration.Templates = map[string]string{}
	}

	roleConfigs := map[string]string{}
	for k, v := range g.roleManifest.Configuration.Templates {
		roleConfigs[k] = v
	}

	for k, v := range g.Configuration.Templates {
		roleConfigs[k] = v
	}

	g.Configuration.Templates = roleConfigs
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

// validateVariableType checks that only legal values are used for
// the type field of variables, and resolves missing information to
// defaults. It reports all variables which are badly typed.
func validateVariableType(variables ConfigurationVariableSlice) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, cv := range variables {
		switch cv.Type {
		case "":
			cv.Type = CVTypeUser
		case CVTypeUser:
		case CVTypeEnv:
			if cv.Internal {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("configuration.variables[%s].type", cv.Name),
					cv.Type, `type conflicts with flag "internal"`))
			}
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("configuration.variables[%s].type", cv.Name),
				cv.Type, "Expected one of user, or environment"))
		}
	}

	return allErrs
}

// validateVariableSorting tests whether the parameters are properly sorted or not.
// It reports all variables which are out of order.
func validateVariableSorting(variables ConfigurationVariableSlice) validation.ErrorList {
	allErrs := validation.ErrorList{}

	previousName := ""
	for _, cv := range variables {
		if cv.Name < previousName {
			allErrs = append(allErrs, validation.Invalid("configuration.variables",
				previousName,
				fmt.Sprintf("Does not sort before '%s'", cv.Name)))
		} else if cv.Name == previousName {
			allErrs = append(allErrs, validation.Invalid("configuration.variables",
				previousName, "Appears more than once"))
		}
		previousName = cv.Name
	}

	return allErrs
}

// validateVariablePreviousNames tests whether PreviousNames of a variable are used either
// by as a Name or a PreviousName of another variable.
func validateVariablePreviousNames(variables ConfigurationVariableSlice) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, cvOuter := range variables {
		for _, previousOuter := range cvOuter.PreviousNames {
			for _, cvInner := range variables {
				if previousOuter == cvInner.Name {
					allErrs = append(allErrs, validation.Invalid("configuration.variables",
						cvOuter.Name,
						fmt.Sprintf("Previous name '%s' also exist as a new variable", cvInner.Name)))
				}
				for _, previousInner := range cvInner.PreviousNames {
					if cvOuter.Name != cvInner.Name && previousOuter == previousInner {
						allErrs = append(allErrs, validation.Invalid("configuration.variables",
							cvOuter.Name,
							fmt.Sprintf("Previous name '%s' also claimed by '%s'", previousOuter, cvInner.Name)))
					}
				}
			}
		}
	}

	return allErrs
}

// validateVariableUsage tests whether all parameters are used in a
// template or not.  It reports all variables which are not used by at
// least one template.  An exception are the variables marked with
// `internal: true`. These are not reported.  Use this to declare
// variables used only in the various scripts and not in templates.
// See also the notes on type `ConfigurationVariable` in this file.
func validateVariableUsage(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// See also 'GetVariablesForRole' (mustache.go).

	unusedConfigs := MakeMapOfVariables(roleManifest)
	if len(unusedConfigs) == 0 {
		return allErrs
	}

	// Iterate over all roles, jobs, templates, extract the used
	// variables. Remove each found from the set of unused
	// configs.

	for _, role := range roleManifest.InstanceGroups {
		for _, jobReference := range role.JobReferences {
			for _, property := range jobReference.Properties {
				propertyName := fmt.Sprintf("properties.%s", property.Name)

				if template, ok := role.Configuration.Templates[propertyName]; ok {
					varsInTemplate, err := parseTemplate(template)
					if err != nil {
						// Ignore bad template, cannot have sensible
						// variable references
						continue
					}
					for _, envVar := range varsInTemplate {
						if _, ok := unusedConfigs[envVar]; ok {
							delete(unusedConfigs, envVar)
						}
						if len(unusedConfigs) == 0 {
							// Everything got used, stop now.
							return allErrs
						}
					}
				}
			}
		}
	}

	// Iterate over the global templates, extract the used
	// variables. Remove each found from the set of unused
	// configs.

	// Note, we have to ignore bad templates (no sensible variable
	// references) and continue to check everything else.

	for _, template := range roleManifest.Configuration.Templates {
		varsInTemplate, err := parseTemplate(template)
		if err != nil {
			continue
		}
		for _, envVar := range varsInTemplate {
			if _, ok := unusedConfigs[envVar]; ok {
				delete(unusedConfigs, envVar)
			}
			if len(unusedConfigs) == 0 {
				// Everything got used, stop now.
				return allErrs
			}
		}
	}

	// We have only the unused variables left in the set. Report
	// those which are not internal.

	for cv, cvar := range unusedConfigs {
		if cvar.Internal {
			continue
		}

		allErrs = append(allErrs, validation.NotFound("configuration.variables",
			fmt.Sprintf("No templates using '%s'", cv)))
	}

	return allErrs
}

// validateTemplateUsage tests whether all templates use only declared variables or not.
// It reports all undeclared variables.
func validateTemplateUsage(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// See also 'GetVariablesForRole' (mustache.go), and LoadRoleManifest (caller, this file)
	declaredConfigs := MakeMapOfVariables(roleManifest)

	// Iterate over all instance groups, jobs, templates, extract the used
	// variables. Report all without a declaration.

	for _, instanceGroup := range roleManifest.InstanceGroups {

		// Note, we cannot use GetVariablesForRole here
		// because it will abort on bad templates. Here we
		// have to ignore them (no sensible variable
		// references) and continue to check everything else.

		for _, jobReference := range instanceGroup.JobReferences {
			for _, property := range jobReference.Properties {
				propertyName := fmt.Sprintf("properties.%s", property.Name)

				if template, ok := instanceGroup.Configuration.Templates[propertyName]; ok {
					varsInTemplate, err := parseTemplate(template)
					if err != nil {
						continue
					}
					for _, envVar := range varsInTemplate {
						if _, ok := declaredConfigs[envVar]; ok {
							continue
						}

						allErrs = append(allErrs, validation.NotFound("configuration.variables",
							fmt.Sprintf("No declaration of '%s'", envVar)))

						// Add a placeholder so that this variable is not reported again.
						// One report is good enough.
						declaredConfigs[envVar] = nil
					}
				}
			}
		}
	}

	// Iterate over the global templates, extract the used
	// variables. Report all without a declaration.

	for _, template := range roleManifest.Configuration.Templates {
		varsInTemplate, err := parseTemplate(template)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}
		for _, envVar := range varsInTemplate {
			if _, ok := declaredConfigs[envVar]; ok {
				continue
			}

			allErrs = append(allErrs, validation.NotFound("configuration.templates",
				fmt.Sprintf("No variable declaration of '%s'", envVar)))

			// Add a placeholder so that this variable is
			// not reported again.  One report is good
			// enough.
			declaredConfigs[envVar] = nil
		}
	}

	return allErrs
}

// validateRoleRun tests whether required fields in the RoleRun are
// set. Note, some of the fields have type-dependent checks. Some
// issues are fixed silently.
func validateRoleRun(instanceGroup *InstanceGroup, roleManifest *RoleManifest, declared CVMap) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run == nil {
		return append(allErrs, validation.Required(
			fmt.Sprintf("instance_groups[%s].run", instanceGroup.Name), ""))
	}

	if instanceGroup.Run.Scaling != nil && instanceGroup.Run.Scaling.HA == 0 {
		instanceGroup.Run.Scaling.HA = instanceGroup.Run.Scaling.Min
	}

	allErrs = append(allErrs, normalizeFlightStage(instanceGroup)...)
	allErrs = append(allErrs, validateHealthCheck(instanceGroup)...)
	allErrs = append(allErrs, validateRoleMemory(instanceGroup)...)
	allErrs = append(allErrs, validateRoleCPU(instanceGroup)...)

	for _, exposedPort := range instanceGroup.Run.ExposedPorts {
		allErrs = append(allErrs, ValidateExposedPorts(instanceGroup.Name, exposedPort)...)
	}

	if instanceGroup.Run.ServiceAccount != "" {
		accountName := instanceGroup.Run.ServiceAccount
		if _, ok := roleManifest.Configuration.Authorization.Accounts[accountName]; !ok {
			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("instance_groups[%s].run.service-account", instanceGroup.Name), accountName))
		}
	}

	// Backwards compat: convert separate volume lists to the centralized one
	for _, persistentVolume := range instanceGroup.Run.PersistentVolumes {
		persistentVolume.Type = VolumeTypePersistent
		instanceGroup.Run.Volumes = append(instanceGroup.Run.Volumes, persistentVolume)
	}
	for _, sharedVolume := range instanceGroup.Run.SharedVolumes {
		sharedVolume.Type = VolumeTypeShared
		instanceGroup.Run.Volumes = append(instanceGroup.Run.Volumes, sharedVolume)
	}
	instanceGroup.Run.PersistentVolumes = nil
	instanceGroup.Run.SharedVolumes = nil
	for _, volume := range instanceGroup.Run.Volumes {
		switch volume.Type {
		case VolumeTypePersistent:
		case VolumeTypeShared:
		case VolumeTypeHost:
		case VolumeTypeNone:
		case VolumeTypeEmptyDir:
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.volumes[%s]", instanceGroup.Name, volume.Tag),
				volume.Type,
				fmt.Sprintf("Invalid volume type '%s'", volume.Type)))
		}
	}

	// Normalize capabilities to upper case, if any.
	var capabilities []string
	for _, cap := range instanceGroup.Run.Capabilities {
		capabilities = append(capabilities, strings.ToUpper(cap))
	}
	instanceGroup.Run.Capabilities = capabilities

	if len(instanceGroup.Run.Environment) == 0 {
		return allErrs
	}

	if instanceGroup.Type == RoleTypeDocker {
		// The environment variables used by docker roles must
		// all be declared, report those which are not.

		for _, envVar := range instanceGroup.Run.Environment {
			if _, ok := declared[envVar]; ok {
				continue
			}

			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("instance_groups[%s].run.env", instanceGroup.Name),
				fmt.Sprintf("No variable declaration of '%s'", envVar)))
		}
	} else {
		// Bosh instance groups must not provide environment variables.

		allErrs = append(allErrs, validation.Forbidden(
			fmt.Sprintf("instance_groups[%s].run.env", instanceGroup.Name),
			"Non-docker instance group declares bogus parameters"))
	}

	return allErrs
}

// ValidateExposedPorts validates exposed port ranges. It also translates the legacy
// format of port ranges ("2000-2010") into the FirstPort and Count values.
func ValidateExposedPorts(name string, exposedPorts *RoleRunExposedPort) validation.ErrorList {
	allErrs := validation.ErrorList{}

	fieldName := fmt.Sprintf("instance_groups[%s].run.exposed-ports[%s]", name, exposedPorts.Name)

	// Validate Name
	if exposedPorts.Name == "" {
		allErrs = append(allErrs, validation.Required(fieldName+".name", ""))
	} else if regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$").FindString(exposedPorts.Name) == "" {
		allErrs = append(allErrs, validation.Invalid(fieldName+".name", exposedPorts.Name,
			"port names must be lowercase words separated by hyphens"))
	}
	if len(exposedPorts.Name) > 15 {
		allErrs = append(allErrs, validation.Invalid(fieldName+".name", exposedPorts.Name,
			"port name must be no more than 15 characters"))
	} else if len(exposedPorts.Name) > 9 && exposedPorts.CountIsConfigurable {
		// need to be able to append "-12345" and still be 15 chars or less
		allErrs = append(allErrs, validation.Invalid(fieldName+".name", exposedPorts.Name,
			"user configurable port name must be no more than 9 characters"))
	}

	// Validate Protocol
	allErrs = append(allErrs, validation.ValidateProtocol(exposedPorts.Protocol, fieldName+".protocol")...)

	// Validate Internal
	firstPort, lastPort, errs := validation.ValidatePortRange(exposedPorts.Internal, fieldName+".internal")
	allErrs = append(allErrs, errs...)
	exposedPorts.InternalPort = firstPort

	if exposedPorts.Count == 0 {
		exposedPorts.Count = lastPort + 1 - firstPort
	} else if lastPort+1-firstPort != exposedPorts.Count {
		allErrs = append(allErrs, validation.Invalid(fieldName+".count", exposedPorts.Count,
			fmt.Sprintf("count doesn't match port range %s", exposedPorts.Internal)))
	}

	// Validate External
	if exposedPorts.External == "" {
		exposedPorts.ExternalPort = exposedPorts.InternalPort
	} else {
		firstPort, lastPort, errs := validation.ValidatePortRange(exposedPorts.External, fieldName+".external")
		allErrs = append(allErrs, errs...)
		exposedPorts.ExternalPort = firstPort

		if lastPort+1-firstPort != exposedPorts.Count {
			allErrs = append(allErrs, validation.Invalid(fieldName+".external", exposedPorts.External,
				"internal and external ranges must have same number of ports"))
		}
	}

	if exposedPorts.Max == 0 {
		exposedPorts.Max = exposedPorts.Count
	}

	// Validate default port count; actual count will be validated at deploy time
	if exposedPorts.Count > exposedPorts.Max {
		allErrs = append(allErrs, validation.Invalid(fieldName+".count", exposedPorts.Count,
			fmt.Sprintf("default number of ports %d is larger than max allowed %d",
				exposedPorts.Count, exposedPorts.Max)))
	}

	// Clear out legacy fields to make sure they aren't still be used elsewhere in the code
	exposedPorts.Internal = ""
	exposedPorts.External = ""

	return allErrs
}

// validateRoleMemory validates memory requests and limits, and
// converts the old key (`memory`, run.MemRequest), to the new
// form. Afterward only run.Memory is valid.
func validateRoleMemory(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run.Memory == nil {
		if instanceGroup.Run.MemRequest != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.MemRequest,
				fmt.Sprintf("instance_groups[%s].run.memory", instanceGroup.Name))...)
		}
		instanceGroup.Run.Memory = &RoleRunMemory{Request: instanceGroup.Run.MemRequest}
		return allErrs
	}

	// assert: role.Run.Memory != nil

	if instanceGroup.Run.Memory.Request == nil {
		if instanceGroup.Run.MemRequest != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.MemRequest,
				fmt.Sprintf("instance_groups[%s].run.memory", instanceGroup.Name))...)
		}
		instanceGroup.Run.Memory.Request = instanceGroup.Run.MemRequest
	} else {
		allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.Memory.Request,
			fmt.Sprintf("instance_groups[%s].run.mem.request", instanceGroup.Name))...)
	}

	if instanceGroup.Run.Memory.Limit != nil {
		allErrs = append(allErrs, validation.ValidateNonnegativeField(*instanceGroup.Run.Memory.Limit,
			fmt.Sprintf("instance_groups[%s].run.mem.limit", instanceGroup.Name))...)
	}

	return allErrs
}

// validateRoleCPU validates cpu requests and limits, and converts the
// old key (`virtual-cpus`, run.VirtualCPUs), to the new
// form. Afterward only run.CPU is valid.
func validateRoleCPU(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run.CPU == nil {
		if instanceGroup.Run.VirtualCPUs != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.VirtualCPUs,
				fmt.Sprintf("instance_groups[%s].run.virtual-cpus", instanceGroup.Name))...)
		}
		instanceGroup.Run.CPU = &RoleRunCPU{Request: instanceGroup.Run.VirtualCPUs}
		return allErrs
	}

	// assert: role.Run.CPU != nil

	if instanceGroup.Run.CPU.Request == nil {
		if instanceGroup.Run.VirtualCPUs != nil {
			allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.VirtualCPUs,
				fmt.Sprintf("instance_groups[%s].run.virtual-cpus", instanceGroup.Name))...)
		}
		instanceGroup.Run.CPU.Request = instanceGroup.Run.VirtualCPUs
	} else {
		allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.CPU.Request,
			fmt.Sprintf("instance_groups[%s].run.cpu.request", instanceGroup.Name))...)
	}

	if instanceGroup.Run.CPU.Limit != nil {
		allErrs = append(allErrs, validation.ValidateNonnegativeFieldFloat(*instanceGroup.Run.CPU.Limit,
			fmt.Sprintf("instance_groups[%s].run.cpu.limit", instanceGroup.Name))...)
	}

	return allErrs
}

// validateHealthCheck reports a instance group with conflicting health checks
// in its probes
func validateHealthCheck(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if instanceGroup.Run.HealthCheck == nil {
		// No health checks, nothing to validate
		return allErrs
	}

	// Ensure that we don't have conflicting health checks
	if instanceGroup.Run.HealthCheck.Readiness != nil {
		allErrs = append(allErrs,
			validateHealthProbe(instanceGroup, "readiness",
				instanceGroup.Run.HealthCheck.Readiness)...)
	}
	if instanceGroup.Run.HealthCheck.Liveness != nil {
		allErrs = append(allErrs,
			validateHealthProbe(instanceGroup, "liveness",
				instanceGroup.Run.HealthCheck.Liveness)...)
	}

	return allErrs
}

// validateHealthProbe reports a instance group with conflicting health checks
// in the specified probe.
func validateHealthProbe(instanceGroup *InstanceGroup, probeName string, probe *HealthProbe) validation.ErrorList {
	allErrs := validation.ErrorList{}

	checks := make([]string, 0, 3)
	if probe.URL != "" {
		checks = append(checks, "url")
	}
	if len(probe.Command) > 0 {
		checks = append(checks, "command")
	}
	if probe.Port != 0 {
		checks = append(checks, "port")
	}
	if len(checks) > 1 {
		allErrs = append(allErrs, validation.Invalid(
			fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
			checks, "Expected at most one of url, command, or port"))
	}
	switch instanceGroup.Type {

	case RoleTypeBosh:
		if len(checks) == 0 {
			allErrs = append(allErrs, validation.Required(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s.command", instanceGroup.Name, probeName),
				"Health check requires a command"))
		} else if checks[0] != "command" {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				checks, "Only command health checks are supported for BOSH instance groups"))
		} else if probeName != "readiness" && len(probe.Command) > 1 {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s.command", instanceGroup.Name, probeName),
				probe.Command, fmt.Sprintf("%s check can only have one command", probeName)))
		}

	case RoleTypeBoshTask:
		if len(checks) > 0 {
			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				"bosh-task instance groups cannot have health checks"))
		}

	case RoleTypeDocker:
		if len(probe.Command) > 1 {
			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("instance_groups[%s].run.healthcheck.%s", instanceGroup.Name, probeName),
				"docker instance groups do not support multiple commands"))
		}

	default:
		// We should have caught the invalid role type when loading the role manifest
		panic("Unexpected role type " + string(instanceGroup.Type) + " in instance group " + instanceGroup.Name)
	}

	return allErrs
}

// normalizeFlightStage reports instance groups with a bad flightstage, and
// fixes all instance groups without a flight stage to use the default
// ('flight').
func normalizeFlightStage(instanceGroup *InstanceGroup) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// Normalize flight stage
	switch instanceGroup.Run.FlightStage {
	case "":
		instanceGroup.Run.FlightStage = FlightStageFlight
	case FlightStagePreFlight:
	case FlightStageFlight:
	case FlightStagePostFlight:
	case FlightStageManual:
	default:
		allErrs = append(allErrs, validation.Invalid(
			fmt.Sprintf("instance_groups[%s].run.flight-stage", instanceGroup.Name),
			instanceGroup.Run.FlightStage,
			"Expected one of flight, manual, post-flight, or pre-flight"))
	}

	return allErrs
}

// validateNonTemplates tests whether the global templates are
// constant or not. It reports the contant templates as errors (They
// should be opinions).
func validateNonTemplates(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// Iterate over the global templates, extract the used
	// variables. Report all templates not using any variable.

	for property, template := range roleManifest.Configuration.Templates {
		varsInTemplate, err := parseTemplate(template)
		if err != nil {
			// Ignore bad template, cannot have sensible
			// variable references
			continue
		}

		if len(varsInTemplate) == 0 {
			allErrs = append(allErrs, validation.Invalid("configuration.templates",
				template,
				fmt.Sprintf("Using '%s' as a constant", property)))
		}
	}

	return allErrs
}

func validateServiceAccounts(roleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}
	for accountName, accountInfo := range roleManifest.Configuration.Authorization.Accounts {
		for _, roleName := range accountInfo.Roles {
			if _, ok := roleManifest.Configuration.Authorization.Roles[roleName]; !ok {
				allErrs = append(allErrs, validation.NotFound(
					fmt.Sprintf("configuration.auth.accounts[%s].roles", accountName),
					roleName))
			}
		}
	}
	return allErrs
}

func validateUnusedColocatedContainerRoles(RoleManifest *RoleManifest) validation.ErrorList {
	counterMap := map[string]int{}
	for _, instanceGroup := range RoleManifest.InstanceGroups {
		// Initialise any instance group of type colocated container in the counter map
		if instanceGroup.Type == RoleTypeColocatedContainer {
			if _, ok := counterMap[instanceGroup.Name]; !ok {
				counterMap[instanceGroup.Name] = 0
			}
		}

		// Increase counter of configured colocated container names
		for _, roleName := range instanceGroup.ColocatedContainers {
			if _, ok := counterMap[roleName]; !ok {
				counterMap[roleName] = 0
			}

			counterMap[roleName]++
		}
	}

	allErrs := validation.ErrorList{}
	for roleName, usageCount := range counterMap {
		if usageCount == 0 {
			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("instance_group[%s]", roleName),
				"instance group is of type colocated container, but is not used by any other instance group as such"))
		}
	}

	return allErrs
}

func validateColocatedContainerPortCollisions(RoleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range RoleManifest.InstanceGroups {
		if len(instanceGroup.ColocatedContainers) > 0 {
			lookupMap := map[string][]string{}

			// Iterate over this instance group and all colocated container instance groups and store the
			// names of the groups for each protocol/port (there should be no list with
			// more than one entry)
			for _, toBeChecked := range append([]*InstanceGroup{instanceGroup}, instanceGroup.GetColocatedRoles()...) {
				for _, exposedPort := range toBeChecked.Run.ExposedPorts {
					for i := 0; i < exposedPort.Count; i++ {
						protocolPortTuple := fmt.Sprintf("%s/%d", exposedPort.Protocol, exposedPort.ExternalPort+i)
						if _, ok := lookupMap[protocolPortTuple]; !ok {
							lookupMap[protocolPortTuple] = []string{}
						}

						lookupMap[protocolPortTuple] = append(lookupMap[protocolPortTuple], toBeChecked.Name)
					}
				}
			}

			// Get a sorted list of the keys (protocol/port)
			protocolPortTuples := []string{}
			for protocolPortTuple := range lookupMap {
				protocolPortTuples = append(protocolPortTuples, protocolPortTuple)
			}
			sort.Strings(protocolPortTuples)

			// Now check if we have port collisions
			for _, protocolPortTuple := range protocolPortTuples {
				names := lookupMap[protocolPortTuple]

				if len(names) > 1 {
					allErrs = append(allErrs, validation.Invalid(
						fmt.Sprintf("instance_group[%s]", instanceGroup.Name),
						protocolPortTuple,
						fmt.Sprintf("port collision, the same protocol/port is used by: %s", strings.Join(names, ", "))))
				}
			}
		}
	}

	return allErrs
}

func validateRoleTags(instanceGroup *InstanceGroup) validation.ErrorList {
	var allErrs validation.ErrorList

	acceptableRoleTypes := map[RoleTag][]RoleType{
		RoleTagActivePassive:     []RoleType{RoleTypeBosh},
		RoleTagHeadless:          []RoleType{RoleTypeBosh, RoleTypeDocker},
		RoleTagSequentialStartup: []RoleType{RoleTypeBosh, RoleTypeDocker},
		RoleTagStopOnFailure:     []RoleType{RoleTypeBoshTask},
	}

	for tagNum, tag := range instanceGroup.Tags {
		switch tag {
		case RoleTagStopOnFailure:
		case RoleTagSequentialStartup:
		case RoleTagHeadless:
		case RoleTagActivePassive:
			if instanceGroup.Run == nil || instanceGroup.Run.ActivePassiveProbe == "" {
				allErrs = append(allErrs, validation.Required(
					fmt.Sprintf("instance_groups[%s].run.active-passive-probe", instanceGroup.Name),
					"active-passive instance groups must specify the correct probe"))
			}
			if instanceGroup.HasTag(RoleTagHeadless) {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
					tag,
					"headless instance groups may not be active-passive"))
			}

		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
				string(tag), "Unknown tag"))
			continue
		}

		if _, ok := acceptableRoleTypes[tag]; !ok {
			allErrs = append(allErrs, validation.InternalError(
				fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
				fmt.Errorf("Tag %s has no acceptable role list", tag)))
			continue
		}

		validTypeForTag := false
		for _, roleType := range acceptableRoleTypes[tag] {
			if roleType == instanceGroup.Type {
				validTypeForTag = true
				break
			}
		}
		if !validTypeForTag {
			var roleNames []string
			for _, roleType := range acceptableRoleTypes[tag] {
				roleNames = append(roleNames, string(roleType))
			}
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].tags[%d]", instanceGroup.Name, tagNum),
				tag,
				fmt.Sprintf("%s tag is only supported in [%s] instance groups, not %s", tag, strings.Join(roleNames, ", "), instanceGroup.Type)))
		}
	}

	return allErrs
}

func validateColocatedContainerVolumeShares(RoleManifest *RoleManifest) validation.ErrorList {
	allErrs := validation.ErrorList{}

	for _, instanceGroup := range RoleManifest.InstanceGroups {
		numberOfColocatedContainers := len(instanceGroup.ColocatedContainers)

		if numberOfColocatedContainers > 0 {
			emptyDirVolumesTags := []string{}
			emptyDirVolumesPath := map[string]string{}
			emptyDirVolumesCount := map[string]int{}

			// Compile a map of all emptyDir volumes with tag -> path of the main instance group
			for _, volume := range instanceGroup.Run.Volumes {
				if volume.Type == VolumeTypeEmptyDir {
					emptyDirVolumesTags = append(emptyDirVolumesTags, volume.Tag)
					emptyDirVolumesPath[volume.Tag] = volume.Path
					emptyDirVolumesCount[volume.Tag] = 0
				}
			}

			for _, colocatedRole := range instanceGroup.GetColocatedRoles() {
				for _, volume := range colocatedRole.Run.Volumes {
					if volume.Type == VolumeTypeEmptyDir {
						if _, ok := emptyDirVolumesCount[volume.Tag]; !ok {
							emptyDirVolumesCount[volume.Tag] = 0
						}

						emptyDirVolumesCount[volume.Tag]++

						if path, ok := emptyDirVolumesPath[volume.Tag]; ok {
							if path != volume.Path {
								// Same tag, but different paths
								allErrs = append(allErrs, validation.Invalid(
									fmt.Sprintf("instance_group[%s]", colocatedRole.Name),
									volume.Path,
									fmt.Sprintf("colocated instance group specifies a shared volume with tag %s, which path does not match the path of the main instance group shared volume with the same tag", volume.Tag)))
							}
						}
					}
				}
			}

			// Check the counters
			sort.Strings(emptyDirVolumesTags)
			for _, tag := range emptyDirVolumesTags {
				count := emptyDirVolumesCount[tag]
				if count != numberOfColocatedContainers {
					allErrs = append(allErrs, validation.Required(
						fmt.Sprintf("instance_group[%s]", instanceGroup.Name),
						fmt.Sprintf("container must use shared volumes of the main instance group: %s", tag)))
				}
			}
		}
	}

	return allErrs
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
