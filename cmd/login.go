// Copyright © 2016 Skatteetaten <utvpaas@skatteetaten.no>

package cmd

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/skatteetaten/aoc/openshift"
)

var userName string

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to openshift clusters",
	Long:  `This command will log in to all avilable clusters and store the tokens in the .aoc config file `,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Please specify affiliation to log in to")
			os.Exit(1)
		}
		affiliation := args[0]
		var configLocation = viper.GetString("HOME") + "/.aoc.json"
		openshift.Login(configLocation, userName, affiliation)
	},
}

func init() {
	RootCmd.AddCommand(loginCmd)
	viper.BindEnv("USER")
	viper.BindEnv("HOME")
	loginCmd.LocalFlags().StringVarP(&userName, "username", "u", viper.GetString("USER"), "the username to log in with, standard is $USER")
}
