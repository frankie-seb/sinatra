package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

const Version = "v0.0.1-dev"

var versionCmd = &cli.Command{
	Name:  "version",
	Usage: "print the version string",
	Action: func(ctx *cli.Context) error {
		fmt.Println(Version)
		return nil
	},
}
