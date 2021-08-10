package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	cfgFile     string
	userLicense string

	rootCmd = &cobra.Command{
		Use:           "sinatra [flags]",
		Short:         "Generate the Sinatra ORM",
		Long:          "Sinatra is a versatile, transparent, database-first boilerplate generator for Go.",
		Example:       "sinatra",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolP("version", "", false, "Print the version")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(initCmd)

}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".sinatra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".sinatra")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
