package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/SUSE/fissile/app"
)

var (
	cfgFile string
	fissile *app.Fissile
	version string

	flagRoleManifest       string
	flagRelease            []string
	flagReleaseName        []string
	flagReleaseVersion     []string
	flagCacheDir           string
	flagWorkDir            string
	flagDockerRegistry     string
	flagDockerOrganization string
	flagRepository         string
	flagWorkers            int
	flagLightOpinions      string
	flagDarkOpinions       string
	flagOutputFormat       string
	flagMetrics            string
	flagVerbose            bool

	// workPath* variables contain paths derived from flagWorkDir
	workPathCompilationDir string
	workPathConfigDir      string
	workPathBaseDockerfile string
	workPathDockerDir      string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "fissile",
	Short: "The BOSH disintegrator",
	Long: `
Fissile converts existing BOSH dev releases into docker images.

It does this using just the releases, without a BOSH deployment, CPIs, or a BOSH 
agent.
`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		if err = validateBasicFlags(); err != nil {
			return err
		}

		return validateReleaseArgs()
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(f *app.Fissile, v string) error {
	fissile = f
	version = v

	return RootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.fissile.yaml)")

	RootCmd.PersistentFlags().StringP(
		"role-manifest",
		"m",
		"",
		"Path to a yaml file that details which jobs are used for each role.",
	)

	// We can't use slices here because of https://github.com/spf13/viper/issues/112
	RootCmd.PersistentFlags().StringP(
		"release",
		"r",
		"",
		"Path to dev BOSH release(s).",
	)

	// We can't use slices here because of https://github.com/spf13/viper/issues/112
	RootCmd.PersistentFlags().StringP(
		"release-name",
		"n",
		"",
		"Name of a dev BOSH release; if empty, default configured dev release name will be used",
	)

	// We can't use slices here because of https://github.com/spf13/viper/issues/112
	RootCmd.PersistentFlags().StringP(
		"release-version",
		"v",
		"",
		"Version of a dev BOSH release; if empty, the latest dev release will be used",
	)

	RootCmd.PersistentFlags().StringP(
		"cache-dir",
		"c",
		filepath.Join(os.Getenv("HOME"), ".bosh", "cache"),
		"Local BOSH cache directory.",
	)

	RootCmd.PersistentFlags().StringP(
		"work-dir",
		"w",
		"/var/fissile",
		"Path to the location of the work directory.",
	)

	RootCmd.PersistentFlags().StringP(
		"repository",
		"p",
		"fissile",
		"Repository name prefix used to create image names.",
	)

	RootCmd.PersistentFlags().StringP(
		"docker-registry",
		"",
		"",
		"Docker registry used when referencing image names",
	)

	RootCmd.PersistentFlags().StringP(
		"docker-organization",
		"",
		"",
		"Docker organization used when referencing image names",
	)

	RootCmd.PersistentFlags().IntP(
		"workers",
		"W",
		2,
		"Number of workers to use.",
	)

	RootCmd.PersistentFlags().StringP(
		"light-opinions",
		"l",
		"",
		"Path to a BOSH deployment manifest file that contains properties to be used as defaults.",
	)

	RootCmd.PersistentFlags().StringP(
		"dark-opinions",
		"d",
		"",
		"Path to a BOSH deployment manifest file that contains properties that should not have opinionated defaults.",
	)

	RootCmd.PersistentFlags().StringP(
		"metrics",
		"M",
		"",
		"Path to a CSV file to store timing metrics into.",
	)

	RootCmd.PersistentFlags().StringP(
		"output",
		"o",
		app.OutputFormatHuman,
		"Choose output format, one of human, json, or yaml (currently only for 'show properties')",
	)

	RootCmd.PersistentFlags().BoolP(
		"verbose",
		"V",
		false,
		"Enable verbose output.",
	)

	viper.BindPFlags(RootCmd.PersistentFlags())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	initViper(viper.GetViper())
}
func initViper(v *viper.Viper) {
	if cfgFile != "" { // enable ability to specify config file via flag
		v.SetConfigFile(cfgFile)
	}

	v.SetEnvPrefix("FISSILE")

	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.SetConfigName(".fissile") // name of config file (without extension)
	v.AddConfigPath("$HOME")    // adding home directory as first search path
	v.AutomaticEnv()            // read in environment variables that match

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err == nil {
		if v == viper.GetViper() {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

// extendPathsFromWorkDirectory sets some directory defaults derived from the
// --work-dir.
func extendPathsFromWorkDirectory() {
	workDir := flagWorkDir
	if workDir == "" {
		return
	}

	// Initialize paths that are always relative to flagWorkDir
	workPathCompilationDir = filepath.Join(workDir, "compilation")
	workPathConfigDir = filepath.Join(workDir, "config")
	workPathBaseDockerfile = filepath.Join(workDir, "base_dockerfile")
	workPathDockerDir = filepath.Join(workDir, "dockerfiles")

	// Set defaults for empty flags
	if flagRoleManifest == "" {
		flagRoleManifest = filepath.Join(workDir, "role-manifest.yml")
	}

	if flagLightOpinions == "" {
		flagLightOpinions = filepath.Join(workDir, "opinions.yml")
	}

	if flagDarkOpinions == "" {
		flagDarkOpinions = filepath.Join(workDir, "dark-opinions.yml")
	}
}

func validateBasicFlags() error {
	var err error

	flagRoleManifest = viper.GetString("role-manifest")
	flagRelease = splitNonEmpty(viper.GetString("release"), ",")
	flagReleaseName = splitNonEmpty(viper.GetString("release-name"), ",")
	flagReleaseVersion = splitNonEmpty(viper.GetString("release-version"), ",")
	flagCacheDir = viper.GetString("cache-dir")
	flagWorkDir = viper.GetString("work-dir")
	flagRepository = viper.GetString("repository")
	flagDockerRegistry = strings.TrimSuffix(viper.GetString("docker-registry"), "/")
	flagDockerOrganization = viper.GetString("docker-organization")
	flagWorkers = viper.GetInt("workers")
	flagLightOpinions = viper.GetString("light-opinions")
	flagDarkOpinions = viper.GetString("dark-opinions")
	flagOutputFormat = viper.GetString("output")
	flagMetrics = viper.GetString("metrics")
	flagVerbose = viper.GetBool("verbose")

	extendPathsFromWorkDirectory()

	if err = absolutePaths(
		&flagRoleManifest,
		&flagCacheDir,
		&flagWorkDir,
		&flagLightOpinions,
		&flagDarkOpinions,
		&flagMetrics,
		&workPathCompilationDir,
		&workPathConfigDir,
		&workPathBaseDockerfile,
		&workPathDockerDir,
	); err != nil {
		return err
	}

	if flagRelease, err = absolutePathsForArray(flagRelease); err != nil {
		return err
	}

	return nil
}

func validateReleaseArgs() error {
	releasePathsCount := len(flagRelease)
	releaseNamesCount := len(flagReleaseName)
	releaseVersionsCount := len(flagReleaseVersion)

	argList := fmt.Sprintf(
		"validateDevReleaseArgs: paths:%s (%d), names:%s (%d), versions:%s (%d)\n",
		flagRelease,
		releasePathsCount,
		flagReleaseName,
		releaseNamesCount,
		flagReleaseVersion,
		releaseVersionsCount,
	)

	if releasePathsCount == 0 {
		return fmt.Errorf("Please specify at least one release path. Args: %s", argList)
	}

	if releaseNamesCount != 0 && releaseNamesCount != releasePathsCount {
		return fmt.Errorf("If you specify custom release names, you need to do it for all of them. Args: %s", argList)
	}

	if releaseVersionsCount != 0 && releaseVersionsCount != releasePathsCount {
		return fmt.Errorf("If you specify custom release versions, you need to do it for all of them. Args: %s", argList)
	}

	return nil
}

func absolutePathsForArray(paths []string) ([]string, error) {
	absolutePaths := make([]string, len(paths))
	for idx, path := range paths {
		absPath, err := absolutePath(path)
		if err != nil {
			return nil, err
		}

		absolutePaths[idx] = absPath
	}

	return absolutePaths, nil
}

func absolutePaths(paths ...*string) error {
	for _, path := range paths {
		absPath, err := absolutePath(*path)
		if err != nil {
			return err
		}

		*path = absPath
	}

	return nil
}

func absolutePath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("Error getting absolute path for path %s: %v", path, err)
	}

	return path, nil
}

func splitNonEmpty(value string, separator string) []string {
	s := strings.Split(value, separator)

	var r []string
	for _, str := range s {
		if len(str) != 0 {
			r = append(r, str)
		}
	}
	return r
}
