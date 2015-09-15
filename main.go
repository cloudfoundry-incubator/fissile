package main

import (
	"os"

	"github.com/hpcloud/fissile/app"

	"github.com/codegangsta/cli"
)

func main() {
	fissile := app.NewFissileApp()

	app := cli.NewApp()
	app.Name = "fissile"
	app.Usage = "Use fissile to break apart a BOSH release."

	app.Commands = []cli.Command{
		{
			Name:    "download-release",
			Aliases: []string{"dr"},
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
			Usage: "List all jobs in a BOSH release",
			Action: func(c *cli.Context) {
				fissile.ListJobs()
			},
		},
		{
			Name:    "list-jobs",
			Aliases: []string{"lj"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a bosh release.",
				},
			},
			Usage: "List all jobs in a BOSH release",
			Action: func(c *cli.Context) {
				fissile.ListJobs()
			},
		},
		{
			Name:    "list-packages",
			Aliases: []string{"lp"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a bosh release.",
				},
			},
			Usage: "List all packages in a BOSH release",
			Action: func(c *cli.Context) {
				fissile.ListJobs()
			},
		},
		{
			Name:    "list-configs",
			Aliases: []string{"lc"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a bosh release. Can be a tgz, a URL or a local directory.",
				},
			},
			Usage: "List all configurations for all jobs in a release",
			Action: func(c *cli.Context) {
				fissile.ListJobs()
			},
		},
	}

	app.Run(os.Args)
}
