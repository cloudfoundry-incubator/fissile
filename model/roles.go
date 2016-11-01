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
	"strings"

	"github.com/hpcloud/fissile/util"

	"gopkg.in/yaml.v2"
)

const (
	boshTaskType = "bosh-task"
	boshType     = "bosh"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	Roles         Roles          `yaml:"roles"`
	Configuration *configuration `yaml:"configuration"`

	manifestFilePath string
	globalConfig     *manifestConfiguration
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name              string         `yaml:"name"`
	Jobs              Jobs           `yaml:"_,omitempty"`
	EnvironScripts    []string       `yaml:"environment_scripts"`
	Scripts           []string       `yaml:"scripts"`
	PostConfigScripts []string       `yaml:"post_config_scripts"`
	Type              string         `yaml:"type,omitempty"`
	JobNameList       []*roleJob     `yaml:"jobs"`
	Configuration     *configuration `yaml:"configuration"`

	rolesManifest *RoleManifest
}

// Roles is an array of Role*
type Roles []*Role

type manifestConfiguration struct {
	darkOpinions        map[string]string
	lightOpinions       map[string]string
	globalDefaultValues map[string]string
}

type configuration struct {
	Templates map[string]string `yaml:"templates"`
}

type roleJob struct {
	Name        string `yaml:"name"`
	ReleaseName string `yaml:"release_name"`
}

// Len is the number of roles in the slice
func (roles Roles) Len() int {
	return len(roles)
}

// Less reports whether role at index i short sort before role at index j
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
			return nil, fmt.Errorf("Error - release %s has been loaded more than once.", release.Name)
		}

		mappedReleases[release.Name] = release
	}

	rolesManifest := RoleManifest{}
	rolesManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &rolesManifest); err != nil {
		return nil, err
	}

	// Remove all roles that are not of the "bosh" or "bosh-task" type
	// Default type is considered to be "bosh"
	for i := len(rolesManifest.Roles) - 1; i >= 0; i-- {
		role := rolesManifest.Roles[i]

		if role.Type != "" && role.Type != boshTaskType && role.Type != boshType {
			rolesManifest.Roles = append(rolesManifest.Roles[:i], rolesManifest.Roles[i+1:]...)
		}
	}

	if rolesManifest.Configuration == nil {
		rolesManifest.Configuration = &configuration{}
	}
	if rolesManifest.Configuration.Templates == nil {
		rolesManifest.Configuration.Templates = map[string]string{}
	}

	for _, role := range rolesManifest.Roles {
		role.rolesManifest = &rolesManifest
		role.Jobs = make(Jobs, 0, len(role.JobNameList))

		for _, roleJob := range role.JobNameList {
			release, ok := mappedReleases[roleJob.ReleaseName]

			if !ok {
				return nil, fmt.Errorf("Error - release %s has not been loaded and is referenced by job %s in role %s.",
					roleJob.ReleaseName, roleJob.Name, role.Name)
			}

			job, err := release.LookupJob(roleJob.Name)
			if err != nil {
				return nil, err
			}

			role.Jobs = append(role.Jobs, job)
		}

		role.calculateRoleConfigurationTemplates()
	}

	return &rolesManifest, nil
}

// SetGlobalConfig initializes the roleManifest's configuration of global entities
// that it needs later to calculate the dev version:
// the global properties, processed light opinions (to get the properties), and raw dark opinions
func (m *RoleManifest) SetGlobalConfig(lightManifestPath, darkManifestPath string) error {
	manifestConfig := manifestConfiguration{}
	opinions, err := manifestConfig.loadOpinionsFromPath(lightManifestPath)
	if err != nil {
		return err
	}
	manifestConfig.lightOpinions = opinions
	opinions, err = manifestConfig.loadOpinionsFromPath(darkManifestPath)
	if err != nil {
		return err
	}
	manifestConfig.darkOpinions = opinions
	manifestConfig.globalDefaultValues = getDefaultValuesHash(m.Configuration.Templates)
	m.globalConfig = &manifestConfig
	return nil
}

// GetRoleManifestDevPackageVersion gets the aggregate signature of all the packages
func (m *RoleManifest) GetRoleManifestDevPackageVersion(extra string) string {
	// Make sure our roles are sorted, to have consistent output
	roles := append(Roles{}, m.Roles...)
	sort.Sort(roles)

	hasher := sha1.New()
	io.WriteString(hasher, extra)

	for _, role := range roles {
		s, err := role.GetRoleDevVersion()
		if err == nil {
			io.WriteString(hasher, s)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// loadOpinionsFromPath flattens the opinions yaml data into a map of dotted property
// names to values and returns it
func (manifest *manifestConfiguration) loadOpinionsFromPath(opinionsPath string) (map[string]string, error) {
	contents, err := ioutil.ReadFile(opinionsPath)
	if err != nil {
		return nil, fmt.Errorf("Error loading opinions from %s: %s\n", opinionsPath, err)
	}
	opinions := map[interface{}]interface{}{}
	err = yaml.Unmarshal(contents, &opinions)
	if err != nil {
		return nil, fmt.Errorf("Error yaml-decoding %s: %s\n", opinionsPath, err)
	}
	return flattenOpinionTree(opinions), nil
}

func lookupProperty(k string, lightOpinions, localDefaultValues, globalDefaultValues map[string]string) (string, error) {
	val, ok := lightOpinions[k]
	if ok {
		return val, nil
	}
	val, ok = localDefaultValues[k]
	if ok {
		return val, nil
	}
	val, ok = localDefaultValues["properties."+k]
	if ok {
		return val, nil
	}
	val, ok = globalDefaultValues[k]
	if ok {
		return val, nil
	}
	val, ok = globalDefaultValues["properties."+k]
	if ok {
		return val, nil
	}
	return "", fmt.Errorf("No value found for property")
}

type anyHash map[interface{}]interface{}
type stringHash map[string]string
type stringAnyHash map[string]interface{}

// flattenOpinionTree converts a tree like
// a:
//   b:
//     c:
//        nonHashValue
// to
// string: a.b.c: nonHashValue
func flattenOpinionTree(h map[interface{}]interface{}) map[string]string {
	result := map[string]string{}
	h1, ok := h["properties"].(map[interface{}]interface{})
	if !ok {
		return result
	}
	for k, v := range h1 {
		k1 := k.(string)
		switch v.(type) {
		case map[interface{}]interface{}:
			flattenOpinionTreeAux(v.(map[interface{}]interface{}), k1+".", result)
		case string:
			result[k1] = v.(string)
		case nil:
			result[k1] = ""
		case int:
			result[k1] = fmt.Sprintf("%d", v.(int))
		default:
			panic(fmt.Sprintf("fissile: Attempt to handle unknown type for key %s in opinions.yml\n", k1))
		}
	}
	return result
}

func flattenOpinionTreeAux(h map[interface{}]interface{}, currentKey string, result map[string]string) {
	for k, v := range h {
		k1 := k.(string)
		switch v1 := v.(type) {
		case map[string]interface{}:
			if getNumKeys(v.(map[interface{}]interface{})) == 0 {
				result[currentKey+k1] = "{}"
			} else {
				flattenOpinionTreeAux(v.(map[interface{}]interface{}), currentKey+k1+".", result)
			}
		case map[interface{}]interface{}:
			if getNumKeys(v.(map[interface{}]interface{})) == 0 {
				result[currentKey+k1] = "{}"
			} else {
				flattenOpinionTreeAux(v1, currentKey+k1+".", result)
			}
		default:
			jbytes, err := util.JSONMarshal(v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Fissile: unexpected error: can't json-encode %s/%v\n", k1, v)
			} else {
				result[currentKey+k1] = string(jbytes)
			}
		}
	}
}

func getNumKeys(h map[interface{}]interface{}) int {
	sum := 0
	for range h {
		sum++
	}
	return sum
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

// func writeScripts(hasher hash.Hash, parentDir string, scripts []string, label string) {
func writeScripts(hasher io.Writer, parentDir string, scripts []string, label string) {
	io.WriteString(hasher, "<"+label+">")
	for _, script := range scripts {
		io.WriteString(hasher, script)
		io.WriteString(hasher, ":\n")
		contents, err := ioutil.ReadFile(filepath.Join(parentDir, script))
		if err != nil {
			// Don't bother emitting an error message as the verifier should have checked
			// the existence of all scripts.
			// Also post_config_scripts are given in terms of destination container paths,
			// not source paths, so we don't have them anyway.
			continue
		}
		hasher.Write(contents)
	}
	io.WriteString(hasher, "</"+label+">")
}

// GetRoleDevVersion gets the aggregate signature of all jobs, packages, scripts, and properties
func (r *Role) GetRoleDevVersion() (string, error) {
	var packages Packages

	globalConfig := r.rolesManifest.globalConfig
	if globalConfig == nil {
		return "", fmt.Errorf("Need to call roleManifest.SetGlobalConfig")
	}
	globalDefaultValues := globalConfig.globalDefaultValues
	lightOpinions := globalConfig.lightOpinions
	darkOpinions := globalConfig.darkOpinions
	localDefaultValues := getDefaultValuesHash(r.Configuration.Templates)
	scriptParentDir := filepath.Dir(r.rolesManifest.manifestFilePath)
	hasher := sha1.New()
	io.WriteString(hasher, r.Name)
	io.WriteString(hasher, r.Type)
	writeScripts(hasher, scriptParentDir, r.EnvironScripts, "EnvironScripts")
	writeScripts(hasher, scriptParentDir, r.Scripts, "Scripts")
	writeScripts(hasher, scriptParentDir, r.PostConfigScripts, "PostConfigScripts")
	io.WriteString(hasher, "<jobs>")
	// Jobs are *not* sorted because they are an array and the order may be
	// significant, in particular for bosh-task roles.
	for _, job := range r.Jobs {
		io.WriteString(hasher, job.SHA1)
		packages = append(packages, job.Packages...)
		hasher.Write([]byte("<properties>"))
		for _, property := range job.Properties {
			_, ok := darkOpinions[property.Name]
			if ok {
				io.WriteString(hasher, "<dark>")
				io.WriteString(hasher, property.Name)
				io.WriteString(hasher, "</dark>")
			}
			val, err := lookupProperty(property.Name, lightOpinions, localDefaultValues, globalDefaultValues)
			if err != nil {
				continue
			}
			io.WriteString(hasher, property.Name)
			io.WriteString(hasher, "=")
			io.WriteString(hasher, val)
			io.WriteString(hasher, "</properties>")
		}
	}
	io.WriteString(hasher, "</jobs>")
	io.WriteString(hasher, "<roleJobs>")
	for _, roleJob := range r.JobNameList {
		io.WriteString(hasher, roleJob.Name)
		io.WriteString(hasher, roleJob.ReleaseName)
	}
	io.WriteString(hasher, "</roleJobs>")
	sort.Sort(packages)
	for _, pkg := range packages {
		io.WriteString(hasher, pkg.SHA1)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (r *Role) calculateRoleConfigurationTemplates() {
	if r.Configuration == nil {
		r.Configuration = &configuration{}
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

func getDefaultValuesHash(templates map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range templates {
		val, err := util.JSONMarshal(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fissile: unexpected error: can't json-encode %v (for key %s)\n", v, k)
			continue
		}
		result[k] = string(val)
	}
	return result
}
