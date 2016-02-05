package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/hpcloud/fissile/app"
	"github.com/hpcloud/fissile/config-store"

	"github.com/codegangsta/cli"
	"github.com/fatih/color"
	"github.com/hpcloud/termui"
	"github.com/hpcloud/termui/sigint"
)

var version string

func main() {
	var ui *termui.UI

	if runtime.GOOS == "windows" {
		ui = termui.New(
			os.Stdin,
			color.Output,
			nil,
		)
	} else {
		ui = termui.New(
			os.Stdin,
			os.Stdout,
			nil,
		)
	}

	cliApp := cli.NewApp()
	cliApp.Name = "fissile"
	cliApp.Usage = "Use fissile to break apart a BOSH release."
	cliApp.Version = version

	fissile := app.NewFissileApplication(version, ui)

	// Towards DRY. A number of helper variables holding flag
	// definitions used in many places.

	boshCacheDir := fmt.Sprintf("%s/%s", os.Getenv("HOME"), ".bosh/cache")

	cacheDirFlag := cli.StringFlag{
		Name:   "cache-dir, cd",
		Usage:  "Local BOSH cache directory",
		Value:  boshCacheDir,
		EnvVar: "FISSILE_DEV_CACHE_DIR",
	}

	baseImageFlag := baseImageFlagFor("Base image.")

	// 2x release - single/multiple, usage differences

	releaseOptionalFlag := cli.StringFlag{
		Name:   "release, r",
		Usage:  "Path to a BOSH release.",
		Value:  ".",
		EnvVar: "FISSILE_RELEASE",
	}
	releasesFlag := cli.StringSliceFlag{
		Name:   "release, r",
		Usage:  "Path to dev BOSH release(s).",
		EnvVar: "FISSILE_RELEASE",
	}
	releaseNameFlag := cli.StringSliceFlag{
		Name:   "release-name, rn",
		Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
		Value:  &cli.StringSlice{},
		EnvVar: "FISSILE_DEV_RELEASE_NAME",
	}
	releaseVersionFlag := cli.StringSliceFlag{
		Name:   "release-version, rv",
		Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
		Value:  &cli.StringSlice{},
		EnvVar: "FISSILE_DEV_RELEASE_VERSION",
	}
	repositoryFlag := cli.StringFlag{
		Name:   "repository, p",
		Value:  "fissile",
		Usage:  "Repository name prefix used to create image names.",
		EnvVar: "FISSILE_REPOSITORY",
	}
	rolesManifestFlag := cli.StringFlag{
		Name:   "roles-manifest, m",
		Usage:  "Path to a yaml file that details which jobs are used for each role",
		EnvVar: "FISSILE_ROLES_MANIFEST",
	}
	debugFlag := cli.BoolFlag{
		Name:   "debug, d",
		Usage:  "If specified, containers won't be deleted when their build fails.",
		EnvVar: "FISSILE_DEBUG",
	}

	workdirFlag := cli.StringFlag{
		Name:   "work-dir, wd",
		Usage:  "Path to the location of the work directory.",
		Value:  "/var/fissile",
		EnvVar: "FISSILE_WORK_DIR",
	}

	prefixFlag := cli.StringFlag{
		Name:   "prefix, p",
		Usage:  "Prefix to be used for all configuration keys.",
		Value:  "hcf",
		EnvVar: "FISSILE_CONFIG_PREFIX",
	}
	defaultConsulAddressFlag := cli.StringFlag{
		Name:   "default-consul-address",
		Usage:  "Default consul address that the container image will try to connect to when run, if not specified",
		Value:  "http://127.0.0.1:8500",
		EnvVar: "FISSILE_DEFAULT_CONSUL_ADDRESS",
	}
	defaultConfigStorePrefixFlag := cli.StringFlag{
		Name:   "default-config-store-prefix",
		Usage:  "Default configuration store prefix that is used by the container, if not specified",
		Value:  "hcf",
		EnvVar: "FISSILE_DEFAULT_CONFIG_STORE_PREFIX",
	}
	lightOpinionsFlag := cli.StringFlag{
		Name:   "light-opinions, l",
		Usage:  "Path to a BOSH deployment manifest file that contains properties to be used as defaults.",
		EnvVar: "FISSILE_LIGHT_OPINIONS",
	}
	darkOpinionsFlag := cli.StringFlag{
		Name:   "dark-opinions, d",
		Usage:  "Path to a BOSH deployment manifest file that contains properties that should not have opinionated defaults.",
		EnvVar: "FISSILE_DARK_OPINIONS",
	}

	// 2x workers

	workersFlag := cli.IntFlag{
		Name:   "workers, w",
		Value:  2,
		Usage:  "Number of compiler workers to use.",
		EnvVar: "FISSILE_COMPILATION_WORKER_COUNT",
	}
	workersBFlag := cli.IntFlag{
		Name:   "workers, w",
		Value:  1,
		Usage:  "Number of workers to use.",
		EnvVar: "FISSILE_WORKER_COUNT",
	}

	noBuildFlag := cli.BoolFlag{
		Name:   "no-build, n",
		Usage:  "If specified, the Dockerfile and assets will be created, but the image won't be built.",
		EnvVar: "FISSILE_NO_BUILD",
	}
	dockerOnlyFlag := cli.BoolFlag{
		Name:   "docker-only, d",
		Usage:  "If the flag is set, only show images that are available on docker",
		EnvVar: "FISSILE_DOCKER_ONLY",
	}
	withSizesFlag := cli.BoolFlag{
		Name:   "with-sizes, s",
		Usage:  "If the flag is set, also show image virtual sizes; only works if the --docker-only flag is set",
		EnvVar: "FISSILE_WITH_SIZES",
	}
	versionFlag := cli.StringFlag{
		Name:   "version, v",
		Usage:  "Used as a version label for the created images",
		EnvVar: "FISSILE_VERSION",
	}
	providerFlag := cli.StringFlag{
		Name:   "provider, o",
		Usage:  "Provider to use when generating the configuration base.",
		Value:  configstore.DirTreeProvider,
		EnvVar: "FISSILE_PROVIDER",
	}

	devReportCommandFlags := []cli.Flag{
		releasesFlag,
		releaseNameFlag,
		releaseVersionFlag,
		cacheDirFlag,
	}

	cliApp.Commands = []cli.Command{
		{
			Name:    "release",
			Aliases: []string{"rel"},
			Subcommands: []cli.Command{
				{
					Name:    "jobs-report",
					Aliases: []string{"jr"},
					Flags: []cli.Flag{
						releaseOptionalFlag,
					},
					Usage:  "List all jobs in a BOSH release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "packages-report",
					Aliases: []string{"pr"},
					Flags: []cli.Flag{
						releaseOptionalFlag,
					},
					Usage:  "List all packages in a BOSH release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "verify",
					Aliases: []string{"v"},
					Flags: []cli.Flag{
						releaseOptionalFlag,
					},
					Usage:  "Verify that the release is intact.",
					Action: fissile.CommandRouter,
				},
			},
		},
		{
			Name:    "compilation",
			Aliases: []string{"comp"},
			Subcommands: []cli.Command{
				{
					Name:    "build-base",
					Aliases: []string{"bb"},
					Flags: []cli.Flag{
						baseImageFlag,
						repositoryFlag,
						debugFlag,
					},
					Usage:       "Prepare a compilation base image",
					Description: "The name of the created image will be <REPOSITORY_PREFIX>:<RELEASE_NAME>-<RELEASE_VERSION>-cbase. If the image already exists, this command does nothing.",
					Action:      fissile.CommandRouter,
				},
				{
					Name:    "show-base",
					Aliases: []string{"sb"},
					Flags: []cli.Flag{
						baseImageFlag,
						repositoryFlag,
					},
					Usage:  "Show information about a base docker image",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "start",
					Aliases: []string{"st"},
					Flags: []cli.Flag{
						repositoryFlag,
						releaseOptionalFlag,
						workdirFlag,
						workersFlag,
						debugFlag,
					},
					Usage:       "Compile packages",
					Description: "Compiles packages from the release using parallel workers",
					Action:      fissile.CommandRouter,
				},
			},
		},
		{
			Name:    "configuration",
			Aliases: []string{"conf"},
			Subcommands: []cli.Command{
				{
					Name:    "report",
					Aliases: []string{"rep"},
					Flags: []cli.Flag{
						releaseOptionalFlag,
					},
					Usage:  "List all configurations for all jobs in a release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "generate",
					Aliases: []string{"gen"},
					Flags: []cli.Flag{
						releasesFlag,
						lightOpinionsFlag,
						darkOpinionsFlag,
						rolesManifestFlag,
						workdirFlag,
						prefixFlag,
						providerFlag,
					},
					Usage:  "Generates a configuration base that can be loaded into something like consul",
					Action: fissile.CommandRouter,
				},
			},
		},
		{
			Name:    "templates",
			Aliases: []string{"tmpl"},
			Subcommands: []cli.Command{
				{
					Name:    "report",
					Aliases: []string{"rep"},
					Flags: []cli.Flag{
						releaseOptionalFlag,
					},
					Usage:  "Print all templates for all jobs in a release",
					Action: fissile.CommandRouter,
				},
			},
		},
		{
			Name:    "images",
			Aliases: []string{"img"},
			Subcommands: []cli.Command{
				{
					Name:    "create-base",
					Aliases: []string{"cb"},
					Flags: []cli.Flag{
						workdirFlag,
						cli.StringFlag{
							Name:   "configgin, c",
							Usage:  "Path to the tarball containing configgin.",
							EnvVar: "FISSILE_CONFIGGIN_PATH",
						},
						baseImageFlagFor("Name of base image to build FROM in the Dockerfile."),
						noBuildFlag,
						repositoryFlag,
					},
					Usage:  "Creates a Dockerfile and a docker image as a base for role images.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "create-roles",
					Aliases: []string{"cr"},
					Flags: []cli.Flag{
						workdirFlag,
						noBuildFlag,
						repositoryFlag,
						releasesFlag,
						rolesManifestFlag,
						defaultConsulAddressFlag,
						defaultConfigStorePrefixFlag,
						versionFlag,
						workersBFlag,
					},
					Usage:  "Creates a Dockerfile and a docker image for each role in a manifest.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "list-roles",
					Aliases: []string{"lr"},
					Flags: []cli.Flag{
						repositoryFlag,
						releasesFlag,
						rolesManifestFlag,
						versionFlag,
						dockerOnlyFlag,
						withSizesFlag,
					},
					Usage:  "Lists role images.",
					Action: fissile.CommandRouter,
				},
			},
		},
		{
			Name: "dev",
			Subcommands: []cli.Command{
				{

					Name:    "jobs-report",
					Aliases: []string{"jr"},
					Flags:   devReportCommandFlags,
					Usage:   "List all jobs in a dev BOSH release",
					Action:  fissile.CommandRouter,
				},
				{
					Name:    "packages-report",
					Aliases: []string{"pr"},
					Flags:   devReportCommandFlags,
					Usage:   "List all packages in a dev BOSH release",
					Action:  fissile.CommandRouter,
				},
				{
					Name:    "compile",
					Aliases: []string{"comp"},
					Flags: []cli.Flag{
						releasesFlag,
						releaseNameFlag,
						releaseVersionFlag,
						cacheDirFlag,
						workdirFlag,
						repositoryFlag,
						workersFlag,
					},
					Usage:  "Compiles packages from dev releases using parallel workers",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "create-images",
					Aliases: []string{"ci"},
					Flags: []cli.Flag{
						workdirFlag,
						releasesFlag,
						releaseNameFlag,
						releaseVersionFlag,
						cacheDirFlag,
						repositoryFlag,
						rolesManifestFlag,
						defaultConsulAddressFlag,
						defaultConfigStorePrefixFlag,
						noBuildFlag,
						cli.BoolFlag{
							Name:   "force, f",
							Usage:  "If specified, image creation will proceed even when images already exist.",
							EnvVar: "FISSILE_FORCE",
						},
					},
					Usage:  "Creates a Dockerfile and a docker image for each role in a manifest",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "list-roles",
					Aliases: []string{"lr"},
					Flags: []cli.Flag{
						repositoryFlag,
						releasesFlag,
						releaseNameFlag,
						releaseVersionFlag,
						cacheDirFlag,
						rolesManifestFlag,
						dockerOnlyFlag,
						withSizesFlag,
					},
					Usage:  "Lists dev role images.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "config-gen",
					Aliases: []string{"cg"},
					Flags: []cli.Flag{
						releasesFlag,
						releaseNameFlag,
						releaseVersionFlag,
						cacheDirFlag,
						rolesManifestFlag,
						workdirFlag,
						lightOpinionsFlag,
						darkOpinionsFlag,
						prefixFlag,
						providerFlag,
					},
					Usage:  "Generates a configuration base that can be loaded into something like consul.",
					Action: fissile.CommandRouter,
				},
			},
		},
	}

	cliApp.After = fissile.CommandAfterHandler

	if err := cliApp.Run(os.Args); err != nil {
		ui.Println(color.RedString("%v", err))
		sigint.DefaultHandler.Exit(1)
	}
}

func baseImageFlagFor(usage string) cli.StringFlag {
	return cli.StringFlag{
		Name:  "base-image, b",
		Usage: usage,
		Value: "ubuntu:14.04",
	}
}
