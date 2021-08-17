package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

// Execute executes the root command.
func Execute() {
	app := cli.NewApp()
	app.Name = "sinatra"
	app.Usage = generateCmd.Usage
	app.Description = "Sinatra is a versatile, transparent, database-first boilerplate generator for Go."
	app.HideVersion = true
	app.Flags = generateCmd.Flags
	app.Version = Version
	app.Before = func(context *cli.Context) error {
		if context.Bool("verbose") {
			log.SetFlags(0)
		} else {
			log.SetOutput(ioutil.Discard)
		}
		return nil
	}

	app.Action = generateCmd.Action
	app.Commands = []*cli.Command{
		generateCmd,
		initCmd,
		versionCmd,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
