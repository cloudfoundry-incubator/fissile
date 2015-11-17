package main

import (
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

	cliApp.Commands = []cli.Command{
		{
			Name:    "release",
			Aliases: []string{"rel"},
			Subcommands: []cli.Command{
				{
					Name:    "jobs-report",
					Aliases: []string{"jr"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "release, r",
							Usage: "Path to a BOSH release.",
							Value: ".",
						},
					},
					Usage:  "List all jobs in a BOSH release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "packages-report",
					Aliases: []string{"pr"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "release, r",
							Usage: "Path to a BOSH release.",
						},
					},
					Usage:  "List all packages in a BOSH release",
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
						cli.StringFlag{
							Name:  "base-image, b",
							Value: "ubuntu:14.04",
							Usage: "Base image.",
						},
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Repository name prefix used to create image names.",
						},
						cli.BoolFlag{
							Name:  "debug, d",
							Usage: "If specified, containers won't be deleted when their build fails.",
						},
					},
					Usage:       "Prepare a compilation base image",
					Description: "The name of the created image will be <REPOSITORY_PREFIX>:<RELEASE_NAME>-<RELEASE_VERSION>-cbase. If the image already exists, this command does nothing.",
					Action:      fissile.CommandRouter,
				},
				{
					Name:    "show-base",
					Aliases: []string{"sb"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "base-image, b",
							Value: "ubuntu:14.04",
							Usage: "Base image.",
						},
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Repository name prefix used to create image names.",
						},
					},
					Usage:  "Show information about a base docker image",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "start",
					Aliases: []string{"st"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Repository name prefix used to create image names.",
						},
						cli.StringFlag{
							Name:  "release, r",
							Usage: "Path to a BOSH release.",
						},
						cli.StringFlag{
							Name:  "target, t",
							Usage: "Path to the location of the compiled packages.",
						},
						cli.IntFlag{
							Name:  "workers, w",
							Value: 2,
							Usage: "Number of compiler workers to use.",
						},
						cli.BoolFlag{
							Name:  "debug, d",
							Usage: "If specified, containers won't be deleted when their build fails.",
						},
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
						cli.StringFlag{
							Name:  "release, r",
							Usage: "Path to a BOSH release.",
						},
					},
					Usage:  "List all configurations for all jobs in a release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "generate",
					Aliases: []string{"gen"},
					Flags: []cli.Flag{
						cli.StringSliceFlag{
							Name:  "release, r",
							Usage: "Path to BOSH release(s).",
						},
						cli.StringFlag{
							Name:  "light-opinions, l",
							Usage: "Path to a BOSH deployment manifest file that contains properties to be used as defaults.",
						},
						cli.StringFlag{
							Name:  "dark-opinions, d",
							Usage: "Path to a BOSH deployment manifest file that contains properties that should not have opinionated defaults.",
						},
						cli.StringFlag{
							Name:  "target, t",
							Usage: "Path to the location of the generated configuration base.",
						},
						cli.StringFlag{
							Name:  "prefix, p",
							Usage: "Prefix to be used for all configuration keys.",
							Value: "hcf",
						},
						cli.StringFlag{
							Name:  "provider, o",
							Usage: "Provider to use when generating the configuration base.",
							Value: configstore.DirTreeProvider,
						},
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
						cli.StringFlag{
							Name:  "release, r",
							Usage: "Path to a BOSH release.",
						},
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
						cli.StringFlag{
							Name:   "target, t",
							Usage:  "Path to the location of the generated Dockerfile and assets.",
							Value:  "/var/fissile/base_dockerfile/",
							EnvVar: "FISSILE_ROLE_BASE_DOCKERFILE_DIR",
						},
						cli.StringFlag{
							Name:   "configgin, c",
							Usage:  "Path to the tarball containing configgin.",
							EnvVar: "FISSILE_CONFIGGIN_PATH",
						},
						cli.StringFlag{
							Name:  "base-image, b",
							Usage: "Name of base image to build FROM in the Dockerfile.",
							Value: "ubuntu:14.04",
						},
						cli.BoolFlag{
							Name:  "no-build, n",
							Usage: "If specified, the Dockerfile and assets will be created, but the image won't be built.",
						},
						cli.StringFlag{
							Name:   "repository, p",
							Value:  "fissile",
							Usage:  "Repository name prefix used to create image names.",
							EnvVar: "FISSILE_REPOSITORY",
						},
					},
					Usage:  "Creates a Dockerfile and a docker image as a base for role images.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "create-roles",
					Aliases: []string{"cr"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "target, t",
							Usage: "Path to the location of the generated Dockerfile and assets.",
						},
						cli.BoolFlag{
							Name:  "no-build, n",
							Usage: "If specified, the Dockerfile and assets will be created, but the image won't be built.",
						},
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Repository name prefix used to create image names.",
						},
						cli.StringSliceFlag{
							Name:  "release, r",
							Usage: "Path to BOSH release(s).",
						},
						cli.StringFlag{
							Name:  "roles-manifest, m",
							Usage: "Path to a yaml file that details which jobs are used for each role",
						},
						cli.StringFlag{
							Name:  "compiled-packages, c",
							Usage: "Path to the directory that contains all compiled packages",
						},
						cli.StringFlag{
							Name:  "default-consul-address",
							Usage: "Default consul address that the container image will try to connect to when run, if not specified",
							Value: "http://127.0.0.1:8500",
						},
						cli.StringFlag{
							Name:  "default-config-store-prefix",
							Usage: "Default configuration store prefix that is used by the container, if not specified",
							Value: "hcf",
						},
						cli.StringFlag{
							Name:  "version, v",
							Usage: "Used as a version label for the created images",
						},
					},
					Usage:  "Creates a Dockerfile and a docker image for each role in a manifest.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "list-roles",
					Aliases: []string{"lr"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Repository name prefix used to create image names.",
						},
						cli.StringSliceFlag{
							Name:  "release, r",
							Usage: "Path to BOSH release(s).",
						},
						cli.StringFlag{
							Name:  "roles-manifest, m",
							Usage: "Path to a yaml file that details which jobs are used for each role",
						},
						cli.StringFlag{
							Name:  "version, v",
							Usage: "Used as a version label for the created images",
						},
						cli.BoolFlag{
							Name:  "docker-only, d",
							Usage: "If the flag is set, only show images that are available on docker",
						},
						cli.BoolFlag{
							Name:  "with-sizes, s",
							Usage: "If the flag is set, also show image virtual sizes; only works if the --docker-only flag is set",
						},
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
					Flags: []cli.Flag{
						cli.StringSliceFlag{
							Name:   "release, r",
							Usage:  "Path to a dev BOSH release",
							EnvVar: "FISSILE_RELEASE",
						},
						cli.StringSliceFlag{
							Name:   "release-name, rn",
							Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_NAME",
						},
						cli.StringSliceFlag{
							Name:   "release-version, rv",
							Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_VERSION",
						},
						cli.StringFlag{
							Name:   "cache-dir, cd",
							Usage:  "Local BOSH cache directory; this should be ~/.bosh/cache",
							EnvVar: "FISSILE_DEV_CACHE_DIR",
						},
					},
					Usage:  "List all jobs in a dev BOSH release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "packages-report",
					Aliases: []string{"pr"},
					Flags: []cli.Flag{
						cli.StringSliceFlag{
							Name:   "release, r",
							Usage:  "Path to a dev BOSH release",
							EnvVar: "FISSILE_RELEASE",
						},
						cli.StringSliceFlag{
							Name:   "release-name, rn",
							Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_NAME",
						},
						cli.StringSliceFlag{
							Name:   "release-version, rv",
							Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_VERSION",
						},
						cli.StringFlag{
							Name:   "cache-dir, cd",
							Usage:  "Local BOSH cache directory; this should be ~/.bosh/cache",
							EnvVar: "FISSILE_DEV_CACHE_DIR",
						},
					},
					Usage:  "List all packages in a dev BOSH release",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "compile",
					Aliases: []string{"comp"},
					Flags: []cli.Flag{
						cli.StringSliceFlag{
							Name:   "release, r",
							Usage:  "Path to a dev BOSH release",
							EnvVar: "FISSILE_RELEASE",
						},
						cli.StringSliceFlag{
							Name:   "release-name, rn",
							Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_NAME",
						},
						cli.StringSliceFlag{
							Name:   "release-version, rv",
							Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_VERSION",
						},
						cli.StringFlag{
							Name:   "cache-dir, cd",
							Usage:  "Local BOSH cache directory; this should be ~/.bosh/cache",
							EnvVar: "FISSILE_DEV_CACHE_DIR",
						},
						cli.StringFlag{
							Name:   "target, t",
							Usage:  "Path to the location of the compiled packages.",
							Value:  "/var/fissile/compilation",
							EnvVar: "FISSILE_COMPILATION_DIR",
						},
						cli.StringFlag{
							Name:   "repository, p",
							Value:  "fissile",
							Usage:  "Repository name prefix used to create image names.",
							EnvVar: "FISSILE_REPOSITORY",
						},
						cli.IntFlag{
							Name:   "workers, w",
							Value:  2,
							Usage:  "Number of compiler workers to use.",
							EnvVar: "FISSILE_COMPILATION_WORKER_COUNT",
						},
					},
					Usage:  "Compiles packages from dev releases using parallel workers",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "create-images",
					Aliases: []string{"ci"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "target, t",
							Usage:  "Path to the location of the generated Dockerfile and assets.",
							Value:  "/var/fissile/dockerfiles",
							EnvVar: "FISSILE_DOCKERFILES_DIR",
						},
						cli.StringSliceFlag{
							Name:   "release, r",
							Usage:  "Path to a dev BOSH release",
							EnvVar: "FISSILE_RELEASE",
						},
						cli.StringSliceFlag{
							Name:   "release-name, rn",
							Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_NAME",
						},
						cli.StringSliceFlag{
							Name:   "release-version, rv",
							Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_VERSION",
						},
						cli.StringFlag{
							Name:   "cache-dir, cd",
							Usage:  "Local BOSH cache directory; this should be ~/.bosh/cache",
							EnvVar: "FISSILE_DEV_CACHE_DIR",
						},
						cli.StringFlag{
							Name:   "repository, p",
							Value:  "fissile",
							Usage:  "Repository name prefix used to create image names.",
							EnvVar: "FISSILE_REPOSITORY",
						},
						cli.StringFlag{
							Name:   "roles-manifest, m",
							Usage:  "Path to a yaml file that details which jobs are used for each role",
							EnvVar: "FISSILE_ROLES_MANIFEST",
						},
						cli.StringFlag{
							Name:   "compiled-packages, c",
							Usage:  "Path to the directory that contains all compiled packages",
							Value:  "/var/fissile/compilation",
							EnvVar: "FISSILE_COMPILATION_DIR",
						},
						cli.StringFlag{
							Name:   "default-consul-address",
							Usage:  "Default consul address that the container image will try to connect to when run, if not specified",
							Value:  "http://127.0.0.1:8500",
							EnvVar: "FISSILE_DEFAULT_CONSUL_ADDRESS",
						},
						cli.StringFlag{
							Name:   "default-config-store-prefix",
							Usage:  "Default configuration store prefix that is used by the container, if not specified",
							Value:  "hcf",
							EnvVar: "FISSILE_DEFAULT_CONFIG_STORE_PREFIX",
						},
						cli.BoolFlag{
							Name:  "no-build, n",
							Usage: "If specified, the Dockerfile and assets will be created, but the image won't be built.",
						},
						cli.BoolFlag{
							Name:  "force, f",
							Usage: "If specified, image creation will proceed even when images already exist.",
						},
					},
					Usage:  "Creates a Dockerfile and a docker image for each role in a manifest",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "list-roles",
					Aliases: []string{"lr"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "repository, p",
							Value:  "fissile",
							Usage:  "Repository name prefix used to create image names.",
							EnvVar: "FISSILE_REPOSITORY",
						},
						cli.StringSliceFlag{
							Name:   "release, r",
							Usage:  "Path to a dev BOSH release",
							EnvVar: "FISSILE_RELEASE",
						},
						cli.StringSliceFlag{
							Name:   "release-name, rn",
							Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_NAME",
						},
						cli.StringSliceFlag{
							Name:   "release-version, rv",
							Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_VERSION",
						},
						cli.StringFlag{
							Name:   "cache-dir, cd",
							Usage:  "Local BOSH cache directory; this should be ~/.bosh/cache",
							EnvVar: "FISSILE_DEV_CACHE_DIR",
						},
						cli.StringFlag{
							Name:   "roles-manifest, m",
							Usage:  "Path to a yaml file that details which jobs are used for each role",
							EnvVar: "FISSILE_ROLES_MANIFEST",
						},
						cli.BoolFlag{
							Name:  "docker-only, d",
							Usage: "If the flag is set, only show images that are available on docker",
						},
						cli.BoolFlag{
							Name:  "with-sizes, s",
							Usage: "If the flag is set, also show image virtual sizes; only works if the --docker-only flag is set",
						},
					},
					Usage:  "Lists dev role images.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "config-gen",
					Aliases: []string{"cg"},
					Flags: []cli.Flag{
						cli.StringSliceFlag{
							Name:   "release, r",
							Usage:  "Path to a dev BOSH release",
							EnvVar: "FISSILE_RELEASE",
						},
						cli.StringSliceFlag{
							Name:   "release-name, rn",
							Usage:  "Name of a dev BOSH release; if empty, default configured dev release name will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_NAME",
						},
						cli.StringSliceFlag{
							Name:   "release-version, rv",
							Usage:  "Version of a dev BOSH release; if empty, the latest dev release will be used",
							Value:  &cli.StringSlice{},
							EnvVar: "FISSILE_DEV_RELEASE_VERSION",
						},
						cli.StringFlag{
							Name:   "cache-dir, cd",
							Usage:  "Local BOSH cache directory; this should be ~/.bosh/cache",
							EnvVar: "FISSILE_DEV_CACHE_DIR",
						},
						cli.StringFlag{
							Name:   "target, t",
							Usage:  "Path to the location of the generated configuration base.",
							Value:  "/var/fissile/dockerfiles",
							EnvVar: "FISSILE_CONFIG_OUTPUT_DIR",
						},
						cli.StringFlag{
							Name:   "light-opinions, l",
							Usage:  "Path to a BOSH deployment manifest file that contains properties to be used as defaults.",
							EnvVar: "FISSILE_LIGHT_OPINIONS",
						},
						cli.StringFlag{
							Name:   "dark-opinions, d",
							Usage:  "Path to a BOSH deployment manifest file that contains properties that should not have opinionated defaults.",
							EnvVar: "FISSILE_DARK_OPINIONS",
						},
						cli.StringFlag{
							Name:   "prefix, p",
							Usage:  "Prefix to be used for all configuration keys.",
							Value:  "hcf",
							EnvVar: "FISSILE_CONFIG_PREFIX",
						},
						cli.StringFlag{
							Name:  "provider, o",
							Usage: "Provider to use when generating the configuration base.",
							Value: configstore.DirTreeProvider,
						},
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
