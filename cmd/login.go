package cmd

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/skatteetaten/ao/pkg/config"
	"github.com/skatteetaten/ao/pkg/prompt"
	"github.com/spf13/cobra"
)

// Change this for new releases of the Boober API
const supportedApiVersion = 2

var (
	flagPassword   string
	flagUserName   string
	flagLocalhost  bool
	flagApiCluster string
)

var loginCmd = &cobra.Command{
	Use:     "login <AuroraConfig>",
	Short:   "Login to all available OpenShift clusters",
	PreRunE: PreLogin,
	RunE:    Login,
	PostRun: PostLogin,
}

func init() {
	RootCmd.AddCommand(loginCmd)
	var username string
	if runtime.GOOS == "windows" {
		user, err := user.Current()
		if err != nil {
			logrus.Fatal("Unable to get current User info: " + err.Error())
		}
		if strings.Contains(user.Username, "\\") {
			parts := strings.Split(user.Username, "\\")
			if len(parts) > 0 {
				username = parts[1]
			}
		}
	} else {
		username, _ = os.LookupEnv("USER")
	}

	loginCmd.Flags().StringVarP(&flagUserName, "username", "u", username, "the username to log in with, standard is current user")
	loginCmd.Flags().StringVarP(&flagPassword, "password", "", "", "the password to log in with, if not set will prompt.  Should only be used in combination with a capturing function to avoid beeing shown in history files")
	loginCmd.Flags().BoolVarP(&flagLocalhost, "localhost", "", false, "set api to localhost")
	loginCmd.Flags().MarkHidden("localhost")
	loginCmd.Flags().StringVarP(&flagApiCluster, "apicluster", "", "", "select specified API cluster")
	loginCmd.Flags().MarkHidden("apicluster")
}

func PreLogin(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("Please specify AuroraConfig to log in to")
	}
	if len(args) == 1 {
		AO.Affiliation = args[0]
	}

	var password string
	if flagPassword != "" {
		password = flagPassword
	}
	for _, c := range AO.Clusters {
		if !c.Reachable || c.HasValidToken() {
			continue
		}
		if password == "" {
			password = prompt.Password()
		}
		token, err := config.GetToken(c.Url, flagUserName, password)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"url":      c.Url,
				"userName": flagUserName,
			}).Fatal(err)
		}
		c.Token = token
	}

	return nil
}

func Login(cmd *cobra.Command, args []string) error {
	if AO.Localhost != flagLocalhost {
		AO.Localhost = flagLocalhost
	}

	if flagApiCluster != "" {
		if _, ok := AO.Clusters[flagApiCluster]; !ok {
			return errors.Errorf("%s is not a valid cluster option. Choose between %v", flagApiCluster, AO.AvailableClusters)
		}
		AO.APICluster = flagApiCluster
	}

	cluster := AO.Clusters[AO.APICluster]
	DefaultApiClient.Token = cluster.Token

	host := cluster.BooberUrl
	gobohost := cluster.GoboUrl

	if AO.Localhost {
		host = "http://localhost:8080"
	}
	DefaultApiClient.Host = host
	DefaultApiClient.GoboHost = gobohost

	acn, err := DefaultApiClient.GetAuroraConfigNames()
	if err != nil {
		return err
	}
	var found bool
	for _, affiliation := range *acn {
		if affiliation == AO.Affiliation {
			found = true
			break
		}
	}
	if !found {
		err := errors.New("Illegal affiliation: " + AO.Affiliation)
		return err
	}

	var apiVersion int
	clientConfig, err := DefaultApiClient.GetClientConfig()
	if err != nil {
		return err
	}

	apiVersion = clientConfig.ApiVersion
	if apiVersion != supportedApiVersion {
		var grade string
		if apiVersion < supportedApiVersion {
			grade = "downgrade"
		} else {
			grade = "upgrade"
		}
		message := fmt.Sprintf("This version of AO does not support Boober with api version %v, you need to %v.", apiVersion, grade)
		return errors.New(message)
	}

	AO.Update(false)
	return config.WriteConfig(*AO, ConfigLocation)
}

func PostLogin(cmd *cobra.Command, args []string) {

	PrintClusters(cmd, true)
	if AO.RefName != "" && AO.RefName != "master" {
		fmt.Printf("\nrefName=%s in AO configurations file. Consider running command \"ao adm update-ref <refName>\" if this is incorrect\n", AO.RefName)
	}
	fmt.Printf("\nConsider running command \"ao adm update-clusters\" if cluster information above looks incorrect \n")
}
