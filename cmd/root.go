package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func init() { //nolint:gochecknoinits
	fmt.Println("")
	fmt.Println("   d888888o.    8 8888 b.             8          .8.    8888888 8888888888 8 888888888o.            .8.          \n .`8888:' `88.  8 8888 888o.          8         .888.         8 8888       8 8888    `88.          .888.         \n 8.`8888.   Y8  8 8888 Y88888o.       8        :88888.        8 8888       8 8888     `88         :88888.        \n `8.`8888.      8 8888 .`Y888888o.    8       . `88888.       8 8888       8 8888     ,88        . `88888.       \n  `8.`8888.     8 8888 8o. `Y888888o. 8      .8. `88888.      8 8888       8 8888.   ,88'       .8. `88888.      \n   `8.`8888.    8 8888 8`Y8o. `Y88888o8     .8`8. `88888.     8 8888       8 888888888P'       .8`8. `88888.     \n    `8.`8888.   8 8888 8   `Y8o. `Y8888    .8' `8. `88888.    8 8888       8 8888`8b          .8' `8. `88888.    \n8b   `8.`8888.  8 8888 8      `Y8o. `Y8   .8'   `8. `88888.   8 8888       8 8888 `8b.       .8'   `8. `88888.   \n`8b.  ;8.`8888  8 8888 8         `Y8o.`  .888888888. `88888.  8 8888       8 8888   `8b.    .888888888. `88888.  \n `Y8888P ,88P'  8 8888 8            `Yo .8'       `8. `88888. 8 8888       8 8888     `88. .8'       `8. `88888. ") //nolint:lll
	fmt.Println("")
	fmt.Println("Sinatra, a go ORM that will light up your ears.")
	fmt.Println("")
}

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
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		} else {
			zerolog.SetGlobalLevel(zerolog.Disabled)
			os.Stdout = nil
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
