package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hpcloud/fissile/app"
)

var cfgFile string
var fissile *app.Fissile

var flagRoleManifest string
var flagRelease []string
var flagReleaseName []string
var flagReleaseVersion []string
var flagCacheDir string
var flagWorkDir string
var flagRepository string
var flagWorkers int
var flagConfiggin string
var flagLightOpinions string
var flagDarkOpinions string

// workPath* variables contain paths derived from flagWorkDir
var workPathCompilationDir string
var workPathConfigDir string
var workPathBaseDockerfile string
var workPathDockerDir string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "fissile",
	Short: "A brief description of your application",
	Long:  ``,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		extendPathsFromWorkDirectory()

		if flagRoleManifest, err = absolutePath(flagRoleManifest); err != nil {
			return err
		}
		if flagCacheDir, err = absolutePath(flagCacheDir); err != nil {
			return err
		}
		if flagWorkDir, err = absolutePath(flagWorkDir); err != nil {
			return err
		}
		if flagConfiggin, err = absolutePath(flagConfiggin); err != nil {
			return err
		}
		if flagLightOpinions, err = absolutePath(flagLightOpinions); err != nil {
			return err
		}
		if flagDarkOpinions, err = absolutePath(flagDarkOpinions); err != nil {
			return err
		}
		if workPathCompilationDir, err = absolutePath(workPathCompilationDir); err != nil {
			return err
		}
		if workPathConfigDir, err = absolutePath(workPathConfigDir); err != nil {
			return err
		}
		if workPathBaseDockerfile, err = absolutePath(workPathBaseDockerfile); err != nil {
			return err
		}
		if workPathDockerDir, err = absolutePath(workPathDockerDir); err != nil {
			return err
		}

		if flagRelease, err = absolutePathsForArray(flagRelease); err != nil {
			return err
		}

		return validateReleaseArgs()
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(f *app.Fissile) {
	fissile = f

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.fissile.yaml)")

	RootCmd.PersistentFlags().StringVarP(
		&flagRoleManifest,
		"role-manifest",
		"m",
		"",
		"Path to a yaml file that details which jobs are used for each role.",
	)

	RootCmd.PersistentFlags().StringSliceVarP(
		&flagRelease,
		"release",
		"r",
		[]string{},
		"Path to dev BOSH release(s).",
	)

	RootCmd.PersistentFlags().StringSliceVarP(
		&flagReleaseName,
		"release-name",
		"n",
		[]string{},
		"Name of a dev BOSH release; if empty, default configured dev release name will be used",
	)

	RootCmd.PersistentFlags().StringSliceVarP(
		&flagReleaseVersion,
		"release-version",
		"v",
		[]string{},
		"Version of a dev BOSH release; if empty, the latest dev release will be used",
	)

	RootCmd.PersistentFlags().StringVarP(
		&flagCacheDir,
		"cache-dir",
		"c",
		"/home/vagrant/.bosh/cache",
		"Local BOSH cache directory.",
	)

	RootCmd.PersistentFlags().StringVarP(
		&flagCacheDir,
		"work-dir",
		"w",
		"/var/fissile",
		"Path to the location of the work directory.",
	)

	RootCmd.PersistentFlags().StringVarP(
		&flagRepository,
		"repository",
		"p",
		"fissile",
		"Repository name prefix used to create image names.",
	)

	RootCmd.PersistentFlags().IntVarP(
		&flagWorkers,
		"workers",
		"W",
		2,
		"Number of workers to use.",
	)

	RootCmd.PersistentFlags().StringVarP(
		&flagConfiggin,
		"configgin",
		"f",
		"",
		"Path to the tarball containing configgin.",
	)

	RootCmd.PersistentFlags().StringVarP(
		&flagLightOpinions,
		"light-opinions",
		"l",
		"",
		"Path to a BOSH deployment manifest file that contains properties to be used as defaults.",
	)

	RootCmd.PersistentFlags().StringVarP(
		&flagDarkOpinions,
		"dark-opinions",
		"d",
		"",
		"Path to a BOSH deployment manifest file that contains properties that should not have opinionated defaults.",
	)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".fissile") // name of config file (without extension)
	viper.AddConfigPath("$HOME")    // adding home directory as first search path
	viper.AutomaticEnv()            // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
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

	if flagConfiggin == "" {
		flagConfiggin = filepath.Join(workDir, "configgin.tar.gz")
	}

	if flagLightOpinions == "" {
		flagLightOpinions = filepath.Join(workDir, "opinions.yml")
	}

	if flagDarkOpinions == "" {
		flagDarkOpinions = filepath.Join(workDir, "dark-opinions.yml")
	}
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
		if absPath, err := absolutePath(path); err == nil {
			absolutePaths[idx] = absPath
		} else {
			return nil, err
		}
	}

	return absolutePaths, nil
}

func absolutePath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("Error getting absolute path for path %s: %v", path, err)
	}

	return path, nil
}
