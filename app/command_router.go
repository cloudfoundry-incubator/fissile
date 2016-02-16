package app

import (
	"fmt"
	"path/filepath"

	"github.com/codegangsta/cli"
)

// CommandRouter will dispatch CLI commands to their relevant functions
func (f *Fissile) CommandRouter(c *cli.Context) {
	var paths map[string]string
	var releasePaths []string
	var err error
	switch c.Command.FullName() {
	case "dev jobs-report",
		"dev packages-report",
		"dev compile",
		"dev create-images",
		"dev list-roles",
		"dev config-gen",
		"configuration generate",
		"images list-roles",
		"images create-roles":
		{
			paths, err = absolutePathsForFlags(c, "work-dir", "light-opinions", "dark-opinions", "roles-manifest", "cache-dir")
			if err != nil {
				f.cmdErr = err
				return
			}

			releasePaths, err = absolutePathsForArray(c.StringSlice("release"))
			if err != nil {
				f.cmdErr = err
				return
			}
		}
	case "configuration diff":
		{
			releasePaths, err = absolutePathsForArray(c.StringSlice("release"))
			if err != nil {
				f.cmdErr = err
				return
			}
		}
	default:
		{
			paths, err = absolutePathsForFlags(c, "release", "work-dir", "configgin", "light-opinions", "dark-opinions", "roles-manifest", "cache-dir")
			if err != nil {
				f.cmdErr = err
				return
			}
		}
	}

	extendPathsFromWorkDirectory(paths)
	switch c.Command.FullName() {
	case "release jobs-report":
		err = f.ListJobs(
			paths["release"],
		)
	case "release packages-report":
		err = f.ListPackages(
			paths["release"],
		)
	case "release verify":
		err = f.VerifyRelease(
			paths["release"],
		)
	case "compilation build-base":
		err = f.CreateBaseCompilationImage(
			c.String("base-image"),
			c.String("repository"),
			c.Bool("debug"),
		)
	case "compilation show-base":
		err = f.ShowBaseImage(
			c.String("base-image"),
			c.String("repository"),
		)
	case "compilation start":
		err = f.Compile(
			paths["release"],
			c.String("repository"),
			paths["compilation-dir"],
			c.Int("workers"),
			c.Bool("debug"),
		)
	case "configuration report":
		err = f.ListFullConfiguration(
			paths["release"],
		)
	case "templates report":
		err = f.PrintTemplateReport(
			paths["release"],
		)
	case "configuration generate":
		err = f.GenerateConfigurationBase(
			releasePaths,
			paths["light-opinions"],
			paths["dark-opinions"],
			paths["roles-manifest"],
			paths["config-dir"],
			c.String("prefix"),
			c.String("provider"),
		)
	case "configuration diff":
		err = f.DiffConfigurationBases(
			releasePaths,
			c.String("prefix"),
		)
	case "images create-base":
		err = f.GenerateBaseDockerImage(
			paths["base-docker-file"],
			paths["configgin"],
			c.String("base-image"),
			c.Bool("no-build"),
			c.String("repository"),
		)
	case "images create-roles":
		err = f.GenerateRoleImages(
			paths["docker-dir"],
			c.String("repository"),
			c.Bool("no-build"),
			releasePaths,
			paths["roles-manifest"],
			paths["compilation-dir"],
			c.String("default-consul-address"),
			c.String("default-config-store-prefix"),
			c.String("version"),
			c.Int("workers"),
		)
	case "images list-roles":
		err = f.ListRoleImages(
			c.String("repository"),
			releasePaths,
			paths["roles-manifest"],
			c.String("version"),
			c.Bool("docker-only"),
			c.Bool("with-sizes"),
		)
	case "dev jobs-report":
		if err = validateDevReleaseArgs(c); err != nil {
			break
		}

		err = f.ListDevJobs(
			releasePaths,
			c.StringSlice("release-name"),
			c.StringSlice("release-version"),
			paths["cache-dir"],
		)
	case "dev packages-report":
		if err = validateDevReleaseArgs(c); err != nil {
			break
		}

		err = f.ListDevPackages(
			releasePaths,
			c.StringSlice("release-name"),
			c.StringSlice("release-version"),
			paths["cache-dir"],
		)
	case "dev compile":
		if err = validateDevReleaseArgs(c); err != nil {
			break
		}

		err = f.CompileDev(
			releasePaths,
			c.StringSlice("release-name"),
			c.StringSlice("release-version"),
			paths["cache-dir"],
			c.String("repository"),
			paths["compilation-dir"],
			c.Int("workers"),
		)
	case "dev create-images":
		if err = validateDevReleaseArgs(c); err != nil {
			break
		}

		err = f.GenerateRoleDevImages(
			paths["docker-dir"],
			c.String("repository"),
			c.Bool("no-build"),
			c.Bool("force"),
			releasePaths,
			c.StringSlice("release-name"),
			c.StringSlice("release-version"),
			paths["cache-dir"],
			paths["roles-manifest"],
			paths["compilation-dir"],
			c.String("default-consul-address"),
			c.String("default-config-store-prefix"),
		)
	case "dev list-roles":
		if err = validateDevReleaseArgs(c); err != nil {
			break
		}

		err = f.ListDevRoleImages(
			c.String("repository"),
			releasePaths,
			c.StringSlice("release-name"),
			c.StringSlice("release-version"),
			paths["cache-dir"],
			paths["roles-manifest"],
			c.Bool("docker-only"),
			c.Bool("with-sizes"),
		)
	case "dev config-gen":
		if err = validateDevReleaseArgs(c); err != nil {
			break
		}

		err = f.GenerateDevConfigurationBase(
			releasePaths,
			c.StringSlice("release-name"),
			c.StringSlice("release-version"),
			paths["cache-dir"],
			paths["roles-manifest"],
			paths["light-opinions"],
			paths["dark-opinions"],
			paths["config-dir"],
			c.String("prefix"),
			c.String("provider"),
		)
	}

	f.cmdErr = err
}

// extendPathsFromWorkDirectory takes the paths map pulled from the commands
// options and extends it with a number of entries derived from the
// --work-dir.
func extendPathsFromWorkDirectory(paths map[string]string) {

	workDir := paths["work-dir"]
	if workDir == "" {
		return
	}

	defaults := []struct {
		key   string
		check bool
		path  string
	}{
		// Paths which are unconditionally created from the --work-dir
		{"compilation-dir", false, "compilation"},
		{"config-dir", false, "config"},
		{"base-docker-file", false, "base_dockerfile"},
		{"docker-dir", false, "dockerfiles"},

		// Paths which have a --work-dir derived default. I.e. are not set
		// if they exist already
		{"configgin", true, "configgin.tar.gz"},
		{"light-opinions", true, "opinions.yml"},
		{"dark-opinions", true, "dark-opinions.yml"},
		{"roles-manifest", true, "role-manifest.yml"},
	}

	for _, v := range defaults {
		if v.check {
			if _, ok := paths[v.key]; ok {
				continue
			}
		}
		paths[v.key] = filepath.Join(workDir, v.path)
	}
}

func validateDevReleaseArgs(c *cli.Context) error {
	releasePathsCount := len(c.StringSlice("release"))
	releaseNamesCount := len(c.StringSlice("release-name"))
	releaseVersionsCount := len(c.StringSlice("release-version"))

	if releasePathsCount == 0 {
		return fmt.Errorf("Please specify at least one release path")
	}

	if releaseNamesCount != 0 && releaseNamesCount != releasePathsCount {
		return fmt.Errorf("If you specify custom release names, you need to do it for all of them")
	}

	if releaseVersionsCount != 0 && releaseVersionsCount != releasePathsCount {
		return fmt.Errorf("If you specify custom release versions, you need to do it for all of them")
	}

	return nil
}

func absolutePathsForArray(paths []string) ([]string, error) {
	absolutePaths := make([]string, len(paths))
	for idx, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("Error getting absolute path for path %s: %v", path, err)
		}

		absolutePaths[idx] = absPath
	}

	return absolutePaths, nil
}

func absolutePathsForFlags(c *cli.Context, flagNames ...string) (map[string]string, error) {
	absolutePaths := map[string]string{}
	for _, flagName := range flagNames {
		if c.String(flagName) == "" {
			continue
		}
		path, err := filepath.Abs(c.String(flagName))
		if err != nil {
			return nil, fmt.Errorf("Error getting absolute path for option %s, path %s: %v", flagName, c.String(flagName), err)
		}

		absolutePaths[flagName] = path
	}

	return absolutePaths, nil
}

// CommandAfterHandler is used to expose any errors from running the commands to
// the rest of the system, to work around the fact that cli.Command.Action does
// not return any error codes.
func (f *Fissile) CommandAfterHandler(c *cli.Context) error {
	return f.cmdErr
}
