package model

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudfoundry-incubator/fissile/util"
	"github.com/cloudfoundry-incubator/fissile/validation"

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
	Name              string         `yaml:"name"`
	Description       string         `yaml:"description"`
	EnvironScripts    []string       `yaml:"environment_scripts"`
	Scripts           []string       `yaml:"scripts"`
	PostConfigScripts []string       `yaml:"post_config_scripts"`
	Type              RoleType       `yaml:"type,omitempty"`
	JobReferences     JobReferences  `yaml:"jobs"`
	Configuration     *Configuration `yaml:"configuration"`
	Tags              []RoleTag      `yaml:"tags"`
	Run               *RoleRun       `yaml:"-"`

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

// RoleTag are the acceptable tags
type RoleTag string

// The list of acceptable tags
const (
	RoleTagStopOnFailure     = RoleTag("stop-on-failure")
	RoleTagSequentialStartup = RoleTag("sequential-startup")
	RoleTagActivePassive     = RoleTag("active-passive")
)

// Validate implements several checks for the instance group and its job references. It's run after the
// instance groups are filtered and i.e. Run has been calculated.
func (g *InstanceGroup) Validate(mappedReleases releaseByName) validation.ErrorList {
	allErrs := validation.ErrorList{}

	if g.Run.ActivePassiveProbe != "" {
		if !g.HasTag(RoleTagActivePassive) {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].run.active-passive-probe", g.Name),
				g.Run.ActivePassiveProbe,
				"Active/passive probes are only valid on instance groups with active-passive tag"))
		}
	}

	for _, jobReference := range g.JobReferences {
		release, ok := mappedReleases[jobReference.ReleaseName]

		if !ok {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].jobs[%s]", g.Name, jobReference.Name),
				jobReference.ReleaseName,
				"Referenced release is not loaded"))
			continue
		}

		job, err := release.LookupJob(jobReference.Name)
		if err != nil {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].jobs[%s]", g.Name, jobReference.Name),
				jobReference.ReleaseName, err.Error()))
			continue
		}
		jobReference.Job = job

		if jobReference.ResolvedConsumers == nil {
			// No explicitly specified consumers
			jobReference.ResolvedConsumers = make(map[string]jobConsumesInfo)
		}

		for name, info := range jobReference.ResolvedConsumers {
			info.Name = name
			jobReference.ResolvedConsumers[name] = info
		}
	}

	g.calculateRoleConfigurationTemplates()

	// Validate that specified colocated containers are configured and of the
	// correct type
	for idx, roleName := range g.ColocatedContainers() {
		if lookupRole := g.roleManifest.LookupInstanceGroup(roleName); lookupRole == nil {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", g.Name, idx),
				roleName,
				"There is no such instance group defined"))

		} else if lookupRole.Type != RoleTypeColocatedContainer {
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].colocated_containers[%d]", g.Name, idx),
				roleName,
				"The instance group is not of required type colocated-container"))
		}
	}

	return allErrs
}

// calculateRoleRun collects properties from the jobs run properties and puts them on the instance group
// It also validates where necessary and is run *before* validateRoleRun
func (g *InstanceGroup) calculateRoleRun() validation.ErrorList {
	allErrs := validation.ErrorList{}

	g.Run = &RoleRun{}

	if ok := g.JobReferences.atLeastOnce(runPropertyPresent); !ok {
		return append(allErrs, validation.Required(
			fmt.Sprintf("instance_groups[%s]", g.Name), "`properties.bosh_containerization.run` required for at least one Job"))
	}

	jobReferences := g.JobReferences.WithRunProperty()

	// need to be the same across jobs, specifying on one job is enough
	if ok := jobReferences.equalOrMissing(flightStagePresent); ok {
		g.Run.FlightStage = jobReferences.firstFlightStage()
	} else {
		allErrs = append(allErrs, validation.Invalid(fmt.Sprintf("instance_groups[%s]", g.Name), g, "If specifying Run.FlightStage properties on multiple jobs of the same instance group, they need to have the same value"))
	}

	g.Run.setScaling(jobReferences)

	g.Run.mergeCapabilities(jobReferences)

	g.Run.mergeVolumes(jobReferences)

	g.Run.setMaxFields(jobReferences)

	for _, j := range jobReferences {
		g.Run.ExposedPorts = append(g.Run.ExposedPorts, j.ContainerProperties.BoshContainerization.Run.ExposedPorts...)
	}

	if ok := jobReferences.atMostOnce(healthCheckPresent); ok {
		g.Run.HealthCheck = jobReferences.firstHealthCheck()
	} else {
		allErrs = append(allErrs, validation.Invalid(fmt.Sprintf("instance_groups[%s]", g.Name), jobReferences.firstHealthCheck(), "Cannot specify Run.HealthCheck properties on more than one job of the same instance group"))
	}

	if property, err := jobReferences.uniqueStringProperty(func(j JobReference) string {
		return j.ContainerProperties.BoshContainerization.Run.ActivePassiveProbe
	}); err == nil {
		g.Run.ActivePassiveProbe = property
	} else {
		allErrs = append(allErrs, validation.Invalid(fmt.Sprintf("instance_groups[%s]", g.Name), property, "Cannot specify Run.ActivePassiveProbe properties on more than one job of the same instance group"))
	}

	if property, err := jobReferences.uniqueStringProperty(func(j JobReference) string {
		return j.ContainerProperties.BoshContainerization.Run.ServiceAccount
	}); err == nil {
		g.Run.ServiceAccount = property
	} else {
		allErrs = append(allErrs, validation.Invalid(fmt.Sprintf("instance_groups[%s]", g.Name), property, "Cannot specify Run.ServiceAccount properties on more than one job of the same instance group"))
	}

	if ok := jobReferences.atMostOnce(affinityPresent); ok {
		g.Run.Affinity = jobReferences.firstAffinity()
	} else {
		allErrs = append(allErrs, validation.Invalid(fmt.Sprintf("instance_groups[%s]", g.Name), jobReferences.firstHealthCheck(), "Cannot specify Run.HealthCheck properties on more than one job of the same instance group"))
	}

	return allErrs
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

// ColocatedContainers returns colocated_container entries from all jobs
func (g *InstanceGroup) ColocatedContainers() []string {
	var containers []string
	for _, j := range g.JobReferences {
		for _, c := range j.ContainerProperties.BoshContainerization.ColocatedContainers {

			containers = append(containers, c)
		}
	}
	return containers

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
func (g *InstanceGroup) GetColocatedRoles() InstanceGroups {
	var result InstanceGroups
	for _, job := range g.JobReferences {
		for _, name := range job.ContainerProperties.BoshContainerization.ColocatedContainers {
			if role := g.roleManifest.LookupInstanceGroup(name); role != nil {
				result = append(result, role)
			}
		}
	}

	return result
}
