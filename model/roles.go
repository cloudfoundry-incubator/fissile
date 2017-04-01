package model

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

// RoleType is the type of the role; see the constants below
type RoleType string

// These are the types of roles available
const (
	RoleTypeBoshTask = RoleType("bosh-task") // A role that is a BOSH task
	RoleTypeBosh     = RoleType("bosh")      // A role that is a BOSH job
	RoleTypeDocker   = RoleType("docker")    // A role that is a raw Docker image
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

// RoleManifest represents a collection of roles
type RoleManifest struct {
	Roles         Roles          `yaml:"roles"`
	Configuration *Configuration `yaml:"configuration"`

	manifestFilePath string
	rolesByName      map[string]*Role
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name              string         `yaml:"name"`
	Jobs              Jobs           `yaml:"_,omitempty"`
	EnvironScripts    []string       `yaml:"environment_scripts"`
	Scripts           []string       `yaml:"scripts"`
	PostConfigScripts []string       `yaml:"post_config_scripts"`
	Type              RoleType       `yaml:"type,omitempty"`
	JobNameList       []*roleJob     `yaml:"jobs"`
	Configuration     *Configuration `yaml:"configuration"`
	Run               *RoleRun       `yaml:"run"`
	Tags              []string       `yaml:"tags"`

	rolesManifest *RoleManifest
}

// RoleRun describes how a role should behave at runtime
type RoleRun struct {
	Scaling           *RoleRunScaling       `yaml:"scaling"`
	Capabilities      []string              `yaml:"capabilities"`
	PersistentVolumes []*RoleRunVolume      `yaml:"persistent-volumes"`
	SharedVolumes     []*RoleRunVolume      `yaml:"shared-volumes"`
	Memory            int                   `yaml:"memory"`
	VirtualCPUs       int                   `yaml:"virtual-cpus"`
	ExposedPorts      []*RoleRunExposedPort `yaml:"exposed-ports"`
	FlightStage       FlightStage           `yaml:"flight-stage"`
	HealthCheck       *HealthCheck          `yaml:"healthcheck,omitempty"`
}

// RoleRunScaling describes how a role should scale out at runtime
type RoleRunScaling struct {
	Min int32 `yaml:"min"`
	Max int32 `yaml:"max"`
}

// RoleRunVolume describes a volume to be attached at runtime
type RoleRunVolume struct {
	Path string `yaml:"path"`
	Tag  string `yaml:"tag"`
	Size int    `yaml:"size"`
}

// RoleRunExposedPort describes a port to be available to other roles, or the outside world
type RoleRunExposedPort struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	External string `yaml:"external"`
	Internal string `yaml:"internal"`
	Public   bool   `yaml:"public"`
}

// HealthCheck describes a non-standard health check endpoint
type HealthCheck struct {
	URL     string            `yaml:"url"`     // URL for a HTTP GET to return 200~399. Cannot be used with other checks.
	Headers map[string]string `yaml:"headers"` // Custom headers; only used for URL.
	Command []string          `yaml:"command"` // Custom command. Cannot be used with other checks.
	Port    int32             `yaml:"port"`    // Port for a TCP probe. Cannot be used with other checks.
}

// Roles is an array of Role*
type Roles []*Role

// Configuration contains information about how to configure the
// resulting images
type Configuration struct {
	Templates map[string]string          `yaml:"templates"`
	Variables ConfigurationVariableSlice `yaml:"variables"`
}

// ConfigurationVariable is a configuration to be exposed to the IaaS
type ConfigurationVariable struct {
	Name        string                          `yaml:"name"`
	Default     interface{}                     `yaml:"default"`
	Description string                          `yaml:"description"`
	Generator   *ConfigurationVariableGenerator `yaml:"generator"`
}

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
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`
	ValueType string `yaml:"value_type"`
}

type roleJob struct {
	Name        string `yaml:"name"`
	ReleaseName string `yaml:"release_name"`
}

// Len is the number of roles in the slice
func (roles Roles) Len() int {
	return len(roles)
}

// Less reports whether role at index i sort before role at index j
func (roles Roles) Less(i, j int) bool {
	return strings.Compare(roles[i].Name, roles[j].Name) < 0
}

// Swap exchanges roles at index i and index j
func (roles Roles) Swap(i, j int) {
	roles[i], roles[j] = roles[j], roles[i]
}

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, releases []*Release) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	mappedReleases := map[string]*Release{}

	for _, release := range releases {
		_, ok := mappedReleases[release.Name]

		if ok {
			return nil, fmt.Errorf("Error - release %s has been loaded more than once", release.Name)
		}

		mappedReleases[release.Name] = release
	}

	rolesManifest := RoleManifest{}
	rolesManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &rolesManifest); err != nil {
		return nil, err
	}

	for i := len(rolesManifest.Roles) - 1; i >= 0; i-- {
		role := rolesManifest.Roles[i]

		// Normalize flight stage
		if role.Run != nil {
			switch role.Run.FlightStage {
			case "":
				role.Run.FlightStage = FlightStageFlight
			case FlightStagePreFlight:
			case FlightStageFlight:
			case FlightStagePostFlight:
			case FlightStageManual:
			default:
				return nil, fmt.Errorf("Role %s has an invalid flight stage %s", role.Name, role.Run.FlightStage)
			}
		}

		// Remove all roles that are not of the "bosh" or "bosh-task" type
		// Default type is considered to be "bosh"
		switch role.Type {
		case "":
			role.Type = RoleTypeBosh
		case RoleTypeBosh, RoleTypeBoshTask:
			continue
		case RoleTypeDocker:
			rolesManifest.Roles = append(rolesManifest.Roles[:i], rolesManifest.Roles[i+1:]...)
		default:
			return nil, fmt.Errorf("Role %s has an invalid type %s", role.Name, role.Type)
		}

		// Ensure that we don't have conflicting health checks
		if role.Run != nil && role.Run.HealthCheck != nil {
			checks := make([]string, 0, 3)
			if role.Run.HealthCheck.URL != "" {
				checks = append(checks, "url")
			}
			if len(role.Run.HealthCheck.Command) > 0 {
				checks = append(checks, "command")
			}
			if role.Run.HealthCheck.Port != 0 {
				checks = append(checks, "port")
			}
			if len(checks) != 1 {
				return nil, fmt.Errorf("Health check for role %s should have exactly one of url, command, or port; got %v", role.Name, checks)
			}
		}
	}

	if rolesManifest.Configuration == nil {
		rolesManifest.Configuration = &Configuration{}
	}
	if rolesManifest.Configuration.Templates == nil {
		rolesManifest.Configuration.Templates = map[string]string{}
	}

	rolesManifest.rolesByName = make(map[string]*Role, len(rolesManifest.Roles))

	for _, role := range rolesManifest.Roles {
		role.rolesManifest = &rolesManifest
		role.Jobs = make(Jobs, 0, len(role.JobNameList))

		for _, roleJob := range role.JobNameList {
			release, ok := mappedReleases[roleJob.ReleaseName]

			if !ok {
				return nil, fmt.Errorf("Error - release %s has not been loaded and is referenced by job %s in role %s",
					roleJob.ReleaseName, roleJob.Name, role.Name)
			}

			job, err := release.LookupJob(roleJob.Name)
			if err != nil {
				return nil, err
			}

			role.Jobs = append(role.Jobs, job)
		}

		role.calculateRoleConfigurationTemplates()
		rolesManifest.rolesByName[role.Name] = role
	}

	return &rolesManifest, nil
}

// GetRoleManifestDevPackageVersion gets the aggregate signature of all the packages
func (m *RoleManifest) GetRoleManifestDevPackageVersion(roles Roles, extra string) string {
	// Make sure our roles are sorted, to have consistent output
	roles = append(Roles{}, roles...)
	sort.Sort(roles)

	hasher := sha1.New()
	hasher.Write([]byte(extra))

	for _, role := range roles {
		hasher.Write([]byte(role.GetRoleDevVersion()))
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// LookupRole will find the given role in the role manifest
func (m *RoleManifest) LookupRole(roleName string) *Role {
	return m.rolesByName[roleName]
}

// GetScriptPaths returns the paths to the startup / post configgin scripts for a role
func (r *Role) GetScriptPaths() map[string]string {
	result := map[string]string{}

	for _, scriptList := range [][]string{r.EnvironScripts, r.Scripts, r.PostConfigScripts} {
		for _, script := range scriptList {
			if filepath.IsAbs(script) {
				// Absolute paths _inside_ the container; there is nothing to copy
				continue
			}
			result[script] = filepath.Join(filepath.Dir(r.rolesManifest.manifestFilePath), script)
		}
	}

	return result

}

// GetRoleDevVersion gets the aggregate signature of all jobs and packages
func (r *Role) GetRoleDevVersion() string {
	roleSignature := ""
	var packages Packages

	// Jobs are *not* sorted because they are an array and the order may be
	// significant, in particular for bosh-task roles.
	for _, job := range r.Jobs {
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, job.SHA1)
		packages = append(packages, job.Packages...)
	}

	sort.Sort(packages)
	for _, pkg := range packages {
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, pkg.SHA1)
	}

	hasher := sha1.New()
	hasher.Write([]byte(roleSignature))
	return hex.EncodeToString(hasher.Sum(nil))
}

// HasTag returns true if the role has a specific tag
func (r *Role) HasTag(tag string) bool {
	for _, t := range r.Tags {
		if t == tag {
			return true
		}
	}

	return false
}

func (r *Role) calculateRoleConfigurationTemplates() {
	if r.Configuration == nil {
		r.Configuration = &Configuration{}
	}
	if r.Configuration.Templates == nil {
		r.Configuration.Templates = map[string]string{}
	}

	roleConfigs := map[string]string{}
	for k, v := range r.rolesManifest.Configuration.Templates {
		roleConfigs[k] = v
	}

	for k, v := range r.Configuration.Templates {
		roleConfigs[k] = v
	}

	r.Configuration.Templates = roleConfigs
}
