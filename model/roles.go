package model

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/SUSE/fissile/validation"

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
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name              string         `yaml:"name"`
	Description       string         `yaml:"description"`
	Jobs              Jobs           `yaml:"_,omitempty"`
	EnvironScripts    []string       `yaml:"environment_scripts"`
	Scripts           []string       `yaml:"scripts"`
	PostConfigScripts []string       `yaml:"post_config_scripts"`
	Type              RoleType       `yaml:"type,omitempty"`
	JobNameList       []*roleJob     `yaml:"jobs"`
	Configuration     *Configuration `yaml:"configuration"`
	Run               *RoleRun       `yaml:"run"`
	Tags              []string       `yaml:"tags"`

	roleManifest *RoleManifest
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
	Affinity          interface{}           `yaml:"affinity,omitempty"`
	Environment       []string              `yaml:"env"`
}

// RoleRunScaling describes how a role should scale out at runtime
type RoleRunScaling struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
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
	Liveness  *HealthProbe `yaml:"liveness,omitempty"`  // Details of liveness probe configuration
	Readiness *HealthProbe `yaml:"readiness,omitempty"` // Ditto for readiness probe
}

// HealthProbe holds the configuration for liveness and readiness
// probes based on the HealthCheck containing them.
type HealthProbe struct {
	URL              string            `yaml:"url"`                         // URL for a HTTP GET to return 200~399. Cannot be used with other checks.
	Headers          map[string]string `yaml:"headers"`                     // Custom headers; only used for URL.
	Command          []string          `yaml:"command"`                     // Custom command. Cannot be used with other checks.
	Port             int               `yaml:"port"`                        // Port for a TCP probe. Cannot be used with other checks.
	InitialDelay     int               `yaml:"initial_delay,omitempty"`     // Initial Delay in seconds, default 3, minimum 1
	Period           int               `yaml:"period,omitempty"`            // Period in seconds, default 10, minimum 1
	Timeout          int               `yaml:"timeout,omitempty"`           // Timeout in seconds, default 3, minimum 1
	SuccessThreshold int               `yaml:"success_threshold,omitempty"` // Success threshold in seconds, default 1, minimum 1
	FailureThreshold int               `yaml:"failure_threshold,omitempty"` // Failure threshold in seconds, default 3, minimum 1
}

// Roles is an array of Role*
type Roles []*Role

// GeneratorType describes the type of generator used for the configuration value
type GeneratorType string

// These are the available generator types for configuration values
const (
	GeneratorTypePassword      = GeneratorType("Password")      // Password
	GeneratorTypeSSH           = GeneratorType("SSH")           // SSH key
	GeneratorTypeCACertificate = GeneratorType("CACertificate") // CA Certificate
	GeneratorTypeCertificate   = GeneratorType("Certificate")   // Certificate
)

// Configuration contains information about how to configure the
// resulting images
type Configuration struct {
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
	Name        string                          `yaml:"name"`
	Default     interface{}                     `yaml:"default"`
	Description string                          `yaml:"description"`
	Example     string                          `yaml:"example"`
	Generator   *ConfigurationVariableGenerator `yaml:"generator"`
	Type        CVType                          `yaml:"type"`
	Internal    bool                            `yaml:"internal,omitempty"`
	Secret      bool                            `yaml:"secret,omitempty"`
	Required    bool                            `yaml:"required,omitempty"`
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
		stringifiedValue = fmt.Sprintf("%v", value)
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

	// See also 'GetVariablesForRole' (mustache.go).
	declaredConfigs := MakeMapOfVariables(&roleManifest)

	allErrs := validation.ErrorList{}

	for i := len(roleManifest.Roles) - 1; i >= 0; i-- {
		role := roleManifest.Roles[i]

		// Remove all roles that are not of the "bosh" or "bosh-task" type
		// Default type is considered to be "bosh".
		switch role.Type {
		case "":
			role.Type = RoleTypeBosh
		case RoleTypeBosh, RoleTypeBoshTask:
			continue
		case RoleTypeDocker:
			roleManifest.Roles = append(roleManifest.Roles[:i], roleManifest.Roles[i+1:]...)
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("roles[%s].type", role.Name),
				role.Type, "Expected one of bosh, bosh-task, or docker"))
		}

		allErrs = append(allErrs, validateRoleRun(role, &roleManifest, declaredConfigs)...)
	}

	for _, role := range roleManifest.Roles {
		role.roleManifest = &roleManifest
		role.Jobs = make(Jobs, 0, len(role.JobNameList))

		for _, roleJob := range role.JobNameList {
			release, ok := mappedReleases[roleJob.ReleaseName]

			if !ok {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("roles[%s].jobs[%s]", role.Name, roleJob.Name),
					roleJob.ReleaseName,
					"Referenced release is not loaded"))
				continue
			}

			job, err := release.LookupJob(roleJob.Name)
			if err != nil {
				allErrs = append(allErrs, validation.Invalid(
					fmt.Sprintf("roles[%s].jobs[%s]", role.Name, roleJob.Name),
					roleJob.ReleaseName, err.Error()))
				continue
			}

			role.Jobs = append(role.Jobs, job)
		}

		role.calculateRoleConfigurationTemplates()
	}

	allErrs = append(allErrs, roleManifest.resolveLinks()...)
	allErrs = append(allErrs, validateVariableType(roleManifest.Configuration.Variables)...)
	allErrs = append(allErrs, validateVariableSorting(roleManifest.Configuration.Variables)...)
	allErrs = append(allErrs, validateVariableUsage(&roleManifest)...)
	allErrs = append(allErrs, validateTemplateUsage(&roleManifest)...)
	allErrs = append(allErrs, validateNonTemplates(&roleManifest)...)

	if len(allErrs) != 0 {
		return nil, fmt.Errorf(allErrs.Errors())
	}

	return &roleManifest, nil
}

// LookupRole will find the given role in the role manifest
func (m *RoleManifest) LookupRole(roleName string) *Role {
	for _, role := range m.Roles {
		if role.Name == roleName {
			return role
		}
	}
	return nil
}

// resolveLinks examines the BOSH links specified in the job specs and maps
// them to the correct role / job that can be looked up at runtime
func (m *RoleManifest) resolveLinks() validation.ErrorList {
	errors := make(validation.ErrorList, 0)

	// Resolve the consumers
	for _, role := range m.Roles {
		// Prefer finding providers in the same role
		providingRoles := append(Roles{role}, m.Roles...)
		for _, job := range role.Jobs {
		iterateOverConsumers:
			for idx, consumer := range job.LinkConsumes {
				for _, providingRole := range providingRoles {
					for _, providingJob := range providingRole.Jobs {
						for _, provider := range providingJob.LinkProvides {
							if consumer.Type != "" && consumer.Type != provider.Type {
								continue
							}
							if consumer.Name != "" && consumer.Name != provider.Name {
								continue
							}
							if consumer.Name == "" {
								consumer.Name = provider.Name
							}
							consumer.RoleName = providingRole.Name
							consumer.JobName = providingJob.Name
							continue iterateOverConsumers
						}
					}
				}

				// No valid providers
				if !consumer.Optional {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf(`role[%s].job[%s].consumes[%d]`, role.Name, job.Name, idx),
						consumer,
					))
				}
			}
		}
	}

	return errors
}

// SelectRoles will find only the given roles in the role manifest
func (m *RoleManifest) SelectRoles(roleNames []string) (Roles, error) {
	if len(roleNames) == 0 {
		// No role names specified, assume all roles
		return m.Roles, nil
	}

	var results Roles
	var missingRoles []string

	for _, roleName := range roleNames {
		if role := m.LookupRole(roleName); role != nil {
			results = append(results, role)
		} else {
			missingRoles = append(missingRoles, roleName)
		}
	}
	if len(missingRoles) > 0 {
		return nil, fmt.Errorf("Some roles are unknown: %v", missingRoles)
	}

	return results, nil
}

// GetLongDescription returns the description of the role plus a list of all included jobs
func (r *Role) GetLongDescription() string {
	desc := r.Description
	if len(desc) > 0 {
		desc += "\n\n"
	}
	desc += fmt.Sprintf("The %s role contains the following jobs:", r.Name)
	var noDesc []string
	also := ""
	for _, job := range r.Jobs {
		if job.Description == "" {
			noDesc = append(noDesc, job.Name)
		} else {
			desc += fmt.Sprintf("\n\n- %s: %s", job.Name, job.Description)
			also = "Also: "
		}
	}
	if len(noDesc) > 0 {
		desc += fmt.Sprintf("\n\n%s%s", also, strings.Join(noDesc, ", "))
	}
	return desc
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
			result[script] = filepath.Join(filepath.Dir(r.roleManifest.manifestFilePath), script)
		}
	}

	return result

}

// GetScriptSignatures returns the SHA1 of all of the script file names and contents
func (r *Role) GetScriptSignatures() (string, error) {
	hasher := sha1.New()

	paths := r.GetScriptPaths()
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
func (r *Role) GetTemplateSignatures() (string, error) {
	hasher := sha1.New()

	i := 0
	templates := make([]string, len(r.Configuration.Templates))

	for k, v := range r.Configuration.Templates {
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
func (r *Role) GetRoleDevVersion(opinions *Opinions, tagDetails, fissileVersion string) (string, error) {

	// Basic role version
	devVersion, err := r.getRoleJobAndPackagesSignature()
	if err != nil {
		return "", fmt.Errorf("Error calculating checksum for role %s: %s", r.Name, err.Error())
	}

	// Aggregate with the properties from the opinions, per each
	// job in the role.  This is similar to what NewDockerPopulator
	// (and its subordinate WriteConfigs) do, with an important
	// difference:
	// - NDP/WC does not care about order. We do, as we need a
	//   stable hash for the configuration.
	signatures := []string{
		devVersion,
		fissileVersion,
		tagDetails,
	}

	// Job order comes from the role manifest, and is sort of
	// fix. Avoid sorting for now.  Also note, if a property is
	// used multiple times, in different jobs, it will be added
	// that often. No deduplication across the jobs.
	for _, job := range r.Jobs {
		// Get properties ...
		properties, err := job.GetPropertiesForJob(opinions)
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
		for _, property := range keys {
			value := flatProps[property]
			signatures = append(signatures, property, value)
		}
	}
	devVersion = AggregateSignatures(signatures)
	return devVersion, nil
}

// getRoleJobAndPackagesSignature gets the aggregate signature of all jobs and packages
func (r *Role) getRoleJobAndPackagesSignature() (string, error) {
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

	// Collect signatures for various script sections
	sig, err := r.GetScriptSignatures()
	if err != nil {
		return "", err
	}
	roleSignature = fmt.Sprintf("%s\n%s", roleSignature, sig)

	// If there are templates, generate signature for them
	if r.Configuration != nil && r.Configuration.Templates != nil {
		sig, err = r.GetTemplateSignatures()
		if err != nil {
			return "", err
		}
		roleSignature = fmt.Sprintf("%s\n%s", roleSignature, sig)
	}

	hasher := sha1.New()
	hasher.Write([]byte(roleSignature))
	return hex.EncodeToString(hasher.Sum(nil)), nil
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
	for k, v := range r.roleManifest.Configuration.Templates {
		roleConfigs[k] = v
	}

	for k, v := range r.Configuration.Templates {
		roleConfigs[k] = v
	}

	r.Configuration.Templates = roleConfigs
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
		}
		previousName = cv.Name
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

	for _, role := range roleManifest.Roles {
		for _, job := range role.Jobs {
			for _, property := range job.Properties {
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

	// See also 'GetVariablesForRole' (mustache.go), and LoadManifest (caller, this file)
	declaredConfigs := MakeMapOfVariables(roleManifest)

	// Iterate over all roles, jobs, templates, extract the used
	// variables. Report all without a declaration.

	for _, role := range roleManifest.Roles {

		// Note, we cannot use GetVariablesForRole here
		// because it will abort on bad templates. Here we
		// have to ignore them (no sensible variable
		// references) and continue to check everything else.

		for _, job := range role.Jobs {
			for _, property := range job.Properties {
				propertyName := fmt.Sprintf("properties.%s", property.Name)

				if template, ok := role.Configuration.Templates[propertyName]; ok {
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
func validateRoleRun(role *Role, roleManifest *RoleManifest, declared CVMap) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if role.Run == nil {
		return append(allErrs, validation.Required(
			fmt.Sprintf("roles[%s].run", role.Name), ""))
	}

	allErrs = append(allErrs, normalizeFlightStage(role)...)
	allErrs = append(allErrs, validateHealthCheck(role)...)
	allErrs = append(allErrs, validation.ValidateNonnegativeField(int64(role.Run.Memory),
		fmt.Sprintf("roles[%s].run.memory", role.Name))...)
	allErrs = append(allErrs, validation.ValidateNonnegativeField(int64(role.Run.VirtualCPUs),
		fmt.Sprintf("roles[%s].run.virtual-cpus", role.Name))...)

	for i := range role.Run.ExposedPorts {
		if role.Run.ExposedPorts[i].Name == "" {
			allErrs = append(allErrs, validation.Required(
				fmt.Sprintf("roles[%s].run.exposed-ports.name", role.Name), ""))
		}

		allErrs = append(allErrs, validation.ValidatePortRange(role.Run.ExposedPorts[i].External,
			fmt.Sprintf("roles[%s].run.exposed-ports[%s].external", role.Name, role.Run.ExposedPorts[i].Name))...)
		allErrs = append(allErrs, validation.ValidatePortRange(role.Run.ExposedPorts[i].Internal,
			fmt.Sprintf("roles[%s].run.exposed-ports[%s].internal", role.Name, role.Run.ExposedPorts[i].Name))...)

		allErrs = append(allErrs, validation.ValidateProtocol(role.Run.ExposedPorts[i].Protocol,
			fmt.Sprintf("roles[%s].run.exposed-ports[%s].protocol", role.Name, role.Run.ExposedPorts[i].Name))...)
	}

	if len(role.Run.Environment) == 0 {
		return allErrs
	}

	if role.Type == RoleTypeDocker {
		// The environment variables used by docker roles must
		// all be declared, report those which are not.

		for _, envVar := range role.Run.Environment {
			if _, ok := declared[envVar]; ok {
				continue
			}

			allErrs = append(allErrs, validation.NotFound(
				fmt.Sprintf("roles[%s].run.env", role.Name),
				fmt.Sprintf("No variable declaration of '%s'", envVar)))
		}
	} else {
		// Bosh roles must not provide environment variables.

		allErrs = append(allErrs, validation.Forbidden(
			fmt.Sprintf("roles[%s].run.env", role.Name),
			"Non-docker role declares bogus parameters"))
	}

	return allErrs
}

// validateHealthCheck reports a role with conflicting health
// checks in its probes
func validateHealthCheck(role *Role) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// Ensure that we don't have conflicting health checks
	if role.Run.HealthCheck != nil {
		if role.Run.HealthCheck.Readiness != nil {
			allErrs = append(allErrs,
				validateHealthProbe(role, "readiness",
					role.Run.HealthCheck.Readiness)...)
		}
		if role.Run.HealthCheck.Liveness != nil {
			allErrs = append(allErrs,
				validateHealthProbe(role, "liveness",
					role.Run.HealthCheck.Liveness)...)
		}
	}

	return allErrs
}

// validateHealthProbe reports a role with conflicting health checks
// in the specified probe.
func validateHealthProbe(role *Role, probeName string, probe *HealthProbe) validation.ErrorList {
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
			fmt.Sprintf("roles[%s].run.healthcheck.%s", role.Name, probeName),
			checks, "Expected at most one of url, command, or port"))
	}

	return allErrs
}

// normalizeFlightStage reports roles with a bad flightstage, and
// fixes all roles without a flight stage to use the default
// ('flight').
func normalizeFlightStage(role *Role) validation.ErrorList {
	allErrs := validation.ErrorList{}

	// Normalize flight stage
	switch role.Run.FlightStage {
	case "":
		role.Run.FlightStage = FlightStageFlight
	case FlightStagePreFlight:
	case FlightStageFlight:
	case FlightStagePostFlight:
	case FlightStageManual:
	default:
		allErrs = append(allErrs, validation.Invalid(
			fmt.Sprintf("roles[%s].run.flight-stage", role.Name),
			role.Run.FlightStage,
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

// LookupJob will find the given job in this role, or nil if not found
func (r *Role) LookupJob(name string) *Job {
	for _, j := range r.Jobs {
		if j.Name == name {
			return j
		}
	}
	return nil
}

// IsDevRole tests if the role is tagged for development, or not. It
// returns true for development-roles, and false otherwise.
func (r *Role) IsDevRole() bool {
	for _, tag := range r.Tags {
		switch tag {
		case "dev-only":
			return true
		}
	}
	return false
}

// IsStopOnFailureRole tests if the role is tagged to stop on a failure, or
// not.
func (r *Role) IsStopOnFailureRole() bool {
	for _, tag := range r.Tags {
		switch tag {
		case "stop-on-failure":
			return true
		}
	}
	return false
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
