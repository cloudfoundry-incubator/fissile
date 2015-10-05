package main

import (
	"log"
	"os"
	"runtime"

	"github.com/hpcloud/fissile/app"

	"github.com/codegangsta/cli"
	"github.com/fatih/color"
)

func main() {
	if runtime.GOOS == "windows" {
		log.SetOutput(color.Output)
	}

	log.SetFlags(0)

	cliApp := cli.NewApp()
	cliApp.Name = "fissile"
	cliApp.Usage = "Use fissile to break apart a BOSH release."

	cliApp.Commands = []cli.Command{
		{
			Name:    "release",
			Aliases: []string{"rel"},
			Subcommands: []cli.Command{
				{
					Name:    "download",
					Aliases: []string{"dl"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "release, r",
							Usage: "URL for a bosh release.",
						},
						cli.StringFlag{
							Name:  "path, p",
							Usage: "Target path for extracting the release",
						},
					},
					Usage:  "Download a BOSH release",
					Action: app.CommandRouter,
				},
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
					Action: app.CommandRouter,
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
					Action: app.CommandRouter,
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
							Name:  "docker-endpoint, d",
							Value: "unix:///var/run/docker.sock",
							Usage: "Docker endpoint.",
						},
						cli.StringFlag{
							Name:  "base-image, b",
							Value: "ubuntu:14.04",
							Usage: "Base image.",
						},
						cli.StringFlag{
							Name:  "repository, p",
							Value: "fissile",
							Usage: "Repository name.",
						},
						cli.StringFlag{
							Name:  "release, r",
							Usage: "Path to a BOSH release.",
						},
					},
					Usage:       "Prepare a compilation base image",
					Description: "The name of the created image will be <REPOSITORY>:<RELEASE_NAME>-<RELEASE_VERSION>-cbase. If the image already exists, this command does nothing.",
					Action:      app.CommandRouter,
				},
				{
					Name:    "show-base",
					Aliases: []string{"sb"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "docker-endpoint, d",
							Value: "unix:///var/run/docker.sock",
							Usage: "Docker endpoint.",
						},
						cli.StringFlag{
							Name:  "base-image, b",
							Value: "ubuntu:14.04",
							Usage: "Base image.",
						},
					},
					Usage:  "Show information about a base docker image",
					Action: app.CommandRouter,
				},
				{
					Name:    "start",
					Aliases: []string{"st"},
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "docker-endpoint, d",
							Value: "unix:///var/run/docker.sock",
							Usage: "Docker endpoint.",
						},
						cli.StringFlag{
							Name:  "base-image, b",
							Value: "ubuntu:14.04",
							Usage: "Base image.",
						},
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
					Action:      app.CommandRouter,
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
					Action: app.CommandRouter,
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
					Action: app.CommandRouter,
				},
			},
		},
	}

	cliApp.Run(os.Args)
}
