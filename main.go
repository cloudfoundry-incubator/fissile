package main

import (
	"log"
	"os"
	"runtime"

	"github.com/hpcloud/fissile/app"
	"github.com/hpcloud/fissile/config-store"

	"github.com/codegangsta/cli"
	"github.com/fatih/color"
)

var version string

func main() {
	if runtime.GOOS == "windows" {
		log.SetOutput(color.Output)
	}

	log.SetFlags(0)

	cliApp := cli.NewApp()
	cliApp.Name = "fissile"
	cliApp.Usage = "Use fissile to break apart a BOSH release."
	cliApp.Version = version

	fissile := app.NewFissileApplication(version)

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
							Usage: "Repository name prefix.",
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
							Usage: "Repository name prefix.",
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
							Usage: "Repository name.",
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
							Value: 4,
							Usage: "Number of compiler workers to use.",
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
					Usage:  "Print all template for all jobs in a release",
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
							Name:  "target, t",
							Usage: "Path to the location of the generated Dockerfile and assets.",
						},
						cli.StringFlag{
							Name:  "configgin, c",
							Usage: "Path to the tarball containing configgin.",
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
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Docker repository name.",
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
							Usage: "Docker repository name prefix.",
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
					Usage:  "Creates Dockerfiles and a docker image for all roles.",
					Action: fissile.CommandRouter,
				},
				{
					Name:    "list-roles",
					Aliases: []string{"lr"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Docker repository name prefix.",
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
	}

	cliApp.Run(os.Args)
}
