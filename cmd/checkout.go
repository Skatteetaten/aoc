package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/skatteetaten/ao/pkg/command"
	"github.com/skatteetaten/ao/pkg/versioncontrol"
	"github.com/spf13/cobra"
	_ "go/token"
	"os"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout",
	Short: "Checkout AuroraConfig (git repository) for current affiliation",
	Run: func(cmd *cobra.Command, args []string) {

		user, _ := cmd.LocalFlags().GetString("user")
		path, _ := cmd.LocalFlags().GetString("path")
		affiliationFlag, _ := cmd.LocalFlags().GetString("affiliation")

		affiliation := ao.Affiliation
		if affiliationFlag != "" {
			affiliation = affiliationFlag
		}

		if len(path) < 1 {
			wd, _ := os.Getwd()
			path = fmt.Sprintf("%s/%s", wd, affiliation)
		}

		url := command.GetGitUrl(affiliation, user, DefaultApiClient)

		logrus.Debug(url)
		fmt.Printf("Cloning AuroraConfig for affiliation %s\n", affiliation)
		fmt.Printf("From: %s\n\n", url)

		output, err := versioncontrol.Checkout(url, path)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			fmt.Print(output)
		}

		err = ao.AddCheckoutPath(affiliation, path, configLocation)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Checkout success")
	},
}

func init() {
	RootCmd.AddCommand(checkoutCmd)

	user, _ := os.LookupEnv("USER")
	checkoutCmd.Flags().StringP("affiliation", "a", "", "Affiliation to clone")
	checkoutCmd.Flags().StringP("path", "p", "", "Checkout repo to path")
	checkoutCmd.Flags().StringP("user", "u", user, "Checkout repo as user")
}
