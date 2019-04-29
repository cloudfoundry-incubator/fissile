package resolver_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/releaseresolver"
	"code.cloudfoundry.org/fissile/model/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// setRoleManifest parses a string instead of reading a file like LoadRoleManifest
// it bypasses ReleaseResolver.Load()
func setRoleManifest(roleManifestPath string, manifestContent []byte, releases Releases) (*RoleManifest, error) {
	roleManifest := NewRoleManifest()
	roleManifest.ManifestFilePath = roleManifestPath
	roleManifest.ManifestContent = manifestContent

	err := yaml.Unmarshal(manifestContent, roleManifest)
	if err != nil {
		return nil, err
	}
	roleManifest.Configuration = &Configuration{RawTemplates: yaml.MapSlice{}}
	roleManifest.LoadedReleases = releases
	return roleManifest, nil
}

// resolveRoleManifest bypasses resolver.Resolve(), this seems to be necessary to set releases manually.
// Just calling resolver.Resolve() or loader.LoadManifest() instead would require a different setup.
func resolveRoleManifest(roleManifest *RoleManifest, roleManifestPath string, allowMissingScripts bool) error {
	r := resolver.NewResolver(
		roleManifest,
		releaseresolver.NewReleaseResolver(roleManifestPath),
		LoadRoleManifestOptions{
			Grapher:           nil,
			ValidationOptions: RoleManifestValidationOptions{AllowMissingScripts: allowMissingScripts},
		},
	)
	return r.ResolveRoleManifest()
}

func TestRoleManifestTagList(t *testing.T) {
	t.Parallel()
	workDir, err := os.Getwd()
	require.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	releases, err := releaseresolver.LoadReleasesFromDisk(
		ReleaseOptions{
			ReleasePaths:     []string{torReleasePath},
			ReleaseNames:     []string{},
			ReleaseVersions:  []string{},
			BOSHCacheDir:     filepath.Join(workDir, "../../test-assets/bosh-cache"),
			FinalReleasesDir: filepath.Join(workDir, "../../test-assets/.final_releases"),
		})
	require.NoError(t, err, "Error reading BOSH release")

	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/tor-good.yml")
	manifestContents, err := ioutil.ReadFile(roleManifestPath)
	require.NoError(t, err, "Error reading role manifest")

	for tag, acceptableRoleTypes := range map[string][]RoleType{
		"stop-on-failure":    []RoleType{RoleTypeBoshTask},
		"sequential-startup": []RoleType{RoleTypeBosh},
		"active-passive":     []RoleType{RoleTypeBosh},
		"indexed":            []RoleType{},
		"clustered":          []RoleType{},
		"invalid":            []RoleType{},
		"no-monit":           []RoleType{},
	} {
		for _, roleType := range []RoleType{RoleTypeBosh, RoleTypeBoshTask, RoleTypeColocatedContainer} {
			func(tag string, roleType RoleType, acceptableRoleTypes []RoleType) {
				t.Run(tag, func(t *testing.T) {
					t.Parallel()
					roleManifest, err := setRoleManifest(roleManifestPath, manifestContents, releases)
					require.NoError(t, err, "Error unmarshalling role manifest")

					require.NotEmpty(t, roleManifest.InstanceGroups, "No instance groups loaded")
					roleManifest.InstanceGroups[0].Type = roleType
					roleManifest.InstanceGroups[0].Tags = []RoleTag{RoleTag(tag)}
					if RoleTag(tag) == RoleTagActivePassive {
						// An active/passive probe is required when tagged as active/passive
						roleManifest.InstanceGroups[0].JobReferences[0].ContainerProperties.BoshContainerization.Run = &RoleRun{ActivePassiveProbe: "hello"}
					}
					err = resolveRoleManifest(roleManifest, roleManifestPath, true)
					acceptable := false
					for _, acceptableRoleType := range acceptableRoleTypes {
						if acceptableRoleType == roleType {
							acceptable = true
						}
					}
					if acceptable {
						assert.NoError(t, err)
					} else {
						message := "Unknown tag"
						if len(acceptableRoleTypes) > 0 {
							var roleNames []string
							for _, acceptableRoleType := range acceptableRoleTypes {
								roleNames = append(roleNames, string(acceptableRoleType))
							}
							message = fmt.Sprintf("%s tag is only supported in [%s] instance groups, not %s",
								tag,
								strings.Join(roleNames, ", "),
								roleType)
						}
						fullMessage := fmt.Sprintf(`instance_groups[myrole].tags[0]: Invalid value: "%s": %s`, tag, message)
						assert.EqualError(t, err, fullMessage)
					}
				})
			}(tag, roleType, acceptableRoleTypes)
		}
	}
}

func TestLoadRoleManifestHealthChecks(t *testing.T) {
	t.Parallel()
	workDir, err := os.Getwd()
	require.NoError(t, err)

	torReleasePath := filepath.Join(workDir, "../../test-assets/tor-boshrelease")
	release, err := NewDevRelease(torReleasePath, "", "", filepath.Join(workDir, "../../test-assets/bosh-cache"))
	require.NoError(t, err, "Error reading BOSH release")

	roleManifestPath := filepath.Join(workDir, "../../test-assets/role-manifests/model/tor-good.yml")
	manifestContents, err := ioutil.ReadFile(roleManifestPath)
	require.NoError(t, err, "Error reading role manifest")

	type sampleStruct struct {
		name        string
		roleType    RoleType
		healthCheck HealthCheck
		err         []string
	}
	for _, sample := range []sampleStruct{
		{
			name: "empty",
		},
		{
			name:     "bosh task with health check",
			roleType: RoleTypeBoshTask,
			healthCheck: HealthCheck{
				Readiness: &HealthProbe{
					Command: []string{"hello"},
				},
			},
			err: []string{
				`instance_groups[myrole].run.healthcheck.readiness: Forbidden: bosh-task instance groups cannot have health checks`,
			},
		},
		{
			name:     "bosh role with command",
			roleType: RoleTypeBosh,
			healthCheck: HealthCheck{
				Readiness: &HealthProbe{
					Command: []string{"/bin/echo", "hello"},
				},
			},
		},
		{
			name:     "bosh role with url",
			roleType: RoleTypeBosh,
			healthCheck: HealthCheck{
				Readiness: &HealthProbe{
					URL: "about:crashes",
				},
			},
			err: []string{
				`instance_groups[myrole].run.healthcheck.readiness: Invalid value: ["url"]: Only command health checks are supported for BOSH instance groups`,
			},
		},
		{
			name:     "bosh role with liveness check with multiple commands",
			roleType: RoleTypeBosh,
			healthCheck: HealthCheck{
				Liveness: &HealthProbe{
					Command: []string{"hello", "world"},
				},
			},
			err: []string{
				`instance_groups[myrole].run.healthcheck.liveness.command: Invalid value: ["hello","world"]: liveness check can only have one command`,
			},
		},
	} {
		func(sample sampleStruct) {
			t.Run(sample.name, func(t *testing.T) {
				var err error // Do not share err with parallel invocations
				t.Parallel()
				roleManifest, err := setRoleManifest(roleManifestPath, manifestContents, []*Release{release})
				require.NoError(t, err, "Error unmarshalling role manifest")

				require.NotEmpty(t, roleManifest.InstanceGroups, "No instance groups loaded")
				if sample.roleType != RoleType("") {
					roleManifest.InstanceGroups[0].Type = sample.roleType
				}
				roleManifest.InstanceGroups[0].JobReferences[0].ContainerProperties.BoshContainerization.Run = &RoleRun{
					HealthCheck: &sample.healthCheck,
				}
				err = resolveRoleManifest(roleManifest, roleManifestPath, true)
				if len(sample.err) > 0 {
					assert.EqualError(t, err, strings.Join(sample.err, "\n"))
					return
				}
				assert.NoError(t, err)
			})
		}(sample)
	}

	t.Run("bosh role with untagged active/passive probe", func(t *testing.T) {
		t.Parallel()
		roleManifest, err := setRoleManifest(roleManifestPath, manifestContents, []*Release{release})
		require.NoError(t, err, "Error unmarshalling role manifest")
		require.NotEmpty(t, roleManifest.InstanceGroups, "No instance groups loaded")

		roleManifest.InstanceGroups[0].Type = RoleTypeBosh
		roleManifest.InstanceGroups[0].Tags = []RoleTag{}
		roleManifest.InstanceGroups[0].JobReferences[0].ContainerProperties.BoshContainerization.Run = &RoleRun{
			ActivePassiveProbe: "/bin/true",
		}
		err = resolveRoleManifest(roleManifest, roleManifestPath, false)
		assert.EqualError(t, err,
			`instance_groups[myrole].run.active-passive-probe: Invalid value: "/bin/true": Active/passive probes are only valid on instance groups with active-passive tag`)
	})

	t.Run("active/passive bosh role without a probe", func(t *testing.T) {
		t.Parallel()
		roleManifest, err := setRoleManifest(roleManifestPath, manifestContents, []*Release{release})
		require.NoError(t, err, "Error unmarshalling role manifest")
		require.NotEmpty(t, roleManifest.InstanceGroups, "No instance groups loaded")

		roleManifest.InstanceGroups[0].Type = RoleTypeBosh
		roleManifest.InstanceGroups[0].Tags = []RoleTag{RoleTagActivePassive}
		roleManifest.InstanceGroups[0].JobReferences[0].ContainerProperties.BoshContainerization.Run = &RoleRun{}
		err = resolveRoleManifest(roleManifest, roleManifestPath, false)
		assert.EqualError(t, err,
			`instance_groups[myrole].run.active-passive-probe: Required value: active-passive instance groups must specify the correct probe`)
	})

	t.Run("bosh task tagged as active/passive", func(t *testing.T) {
		t.Parallel()
		roleManifest, err := setRoleManifest(roleManifestPath, manifestContents, []*Release{release})
		require.NoError(t, err, "Error unmarshalling role manifest")
		require.NotEmpty(t, roleManifest.InstanceGroups, "No instance groups loaded")

		roleManifest.InstanceGroups[0].Type = RoleTypeBoshTask
		roleManifest.InstanceGroups[0].Tags = []RoleTag{RoleTagActivePassive}
		roleManifest.InstanceGroups[0].JobReferences[0].ContainerProperties.BoshContainerization.Run = &RoleRun{ActivePassiveProbe: "/bin/false"}
		err = resolveRoleManifest(roleManifest, roleManifestPath, false)
		assert.EqualError(t, err,
			`instance_groups[myrole].tags[0]: Invalid value: "active-passive": active-passive tag is only supported in [bosh] instance groups, not bosh-task`)
	})
}
