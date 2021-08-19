package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func init() { //nolint:gochecknoinits
	fmt.Println("______               _    _        _    _            _ _   _\n|  ____|             | |  (_)      | |  | |          | | | | |\n| |__ _ __ __ _ _ __ | | ___  ___  | |__| | ___  __ _| | |_| |__\n|  __| '__/ _` | '_ \\| |/ / |/ _ \\ |  __  |/ _ \\/ _` | | __| '_ \\\n| |  | | | (_| | | | |   <| |  __/ | |  | |  __/ (_| | | |_| | | |\n|_|  |_|  \\__,_|_| |_|_|\\_\\_|\\___| |_|  |_|\\___|\\__,_|_|\\__|_| |_|") //nolint:lll
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
