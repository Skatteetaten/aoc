// Copyright © 2016 Skatteetaten <utvpaas@skatteetaten.no>

package cmd

import (
	"fmt"
	"github.com/skatteetaten/aoc/pkg/cmdoptions"
	"github.com/skatteetaten/aoc/pkg/openshift"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var persistentOptions cmdoptions.CommonCommandOptions

//var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "aoc",
	Short: "Aurora Openshift CLI",
	Long: `A command line interface that interacts with serverapi

This application has two main parts.
1. manage the aoc configuration via cli
2. apply the aoc configuration to the clusters
`,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().BoolVarP(&persistentOptions.Verbose, "verbose",
		"v", false, "Log progress to standard out")
	RootCmd.PersistentFlags().BoolVarP(&persistentOptions.Debug, "debug",
		"", false, "Show debug information")
	RootCmd.PersistentFlags().MarkHidden("debug")
	RootCmd.PersistentFlags().BoolVarP(&persistentOptions.DryRun, "dryrun",
		"d", false,
		"Do not perform a setup, just collect and print the configuration files")
	RootCmd.PersistentFlags().BoolVarP(&persistentOptions.Localhost, "localhost",
		"l", false, "Send setup to Boober on localhost")
	RootCmd.PersistentFlags().MarkHidden("localhost")
	RootCmd.PersistentFlags().BoolVarP(&persistentOptions.ShowConfig, "showconfig",
		"s", false, "Print merged config from Boober to standard out")
	RootCmd.PersistentFlags().BoolVarP(&persistentOptions.ShowObjects, "showobjects",
		"o", false, "Print object definitions from Boober to standard out")
	// test

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigName(".aoc")  // name of config file (without extension)
	viper.AddConfigPath("$HOME") // adding home directory as first search path
	viper.AutomaticEnv()         // read in environment variables that match
	viper.BindEnv("HOME")

	var configLocation = viper.GetString("HOME") + "/.aoc.json"
	openshift.LoadOrInitiateConfigFile(configLocation)

}
